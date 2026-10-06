// Copyright 2019-2026 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gatewayapi

import (
	"context"
	"fmt"
	"strings"
	"sync"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	offloadingv1beta1apply "github.com/liqotech/liqo/pkg/client/applyconfiguration/offloading/v1beta1"
	offloadingv1beta1clients "github.com/liqotech/liqo/pkg/client/clientset/versioned/typed/offloading/v1beta1"
	offloadingv1beta1listers "github.com/liqotech/liqo/pkg/client/listers/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// statusReflector reflects the status of the remote objects back to the local cluster, through shadow resources
// which are then aggregated by the controller manager, since the same object may be reflected to multiple clusters.
type statusReflector[O object] interface {
	// Enforce ensures the shadow resource reports the status of the given remote object.
	Enforce(ctx context.Context, local, remote O) error
	// Fail ensures the shadow resource reports that the given local object could not be reflected, for the given reason.
	Fail(ctx context.Context, local O, message string) error
	// Delete ensures the absence of the shadow resource associated with the given local object.
	Delete(ctx context.Context, name string) error
	// Cleanup deletes all the shadow resources reporting the status of the objects reflected to the remote cluster.
	Cleanup(ctx context.Context) error
}

// ShadowName returns the name of the shadow resource reporting the status of the given object in the remote cluster.
func ShadowName(prefix, name string) string {
	if prefix == "" {
		return fmt.Sprintf("%s-%s", name, forge.RemoteCluster)
	}
	return fmt.Sprintf("%s-%s-%s", prefix, name, forge.RemoteCluster)
}

// shadowLabels returns the labels of the shadow resources, identifying the remote cluster they refer to.
func shadowLabels() map[string]string {
	return map[string]string{forge.LiqoOriginClusterIDKey: string(forge.RemoteCluster)}
}

// shadowSelector returns the selector of the shadow resources referring to the remote cluster.
func shadowSelector() labels.Selector {
	return labels.SelectorFromSet(shadowLabels())
}

// virtualNodeOwner caches the owner reference to the virtual node, which is associated with the shadow resources
// to ensure they are garbage collected when the virtual node (i.e., the peering) is removed.
var virtualNodeOwner struct {
	sync.Mutex
	uid types.UID
}

func virtualNodeOwnerReference(ctx context.Context, client kubernetes.Interface) (*metav1apply.OwnerReferenceApplyConfiguration, error) {
	virtualNodeOwner.Lock()
	defer virtualNodeOwner.Unlock()

	if virtualNodeOwner.uid == "" {
		node, err := client.CoreV1().Nodes().Get(ctx, forge.LiqoNodeName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve virtual node %q: %w", forge.LiqoNodeName, err)
		}
		virtualNodeOwner.uid = node.GetUID()
	}

	return metav1apply.OwnerReference().WithAPIVersion("v1").WithKind("Node").
		WithName(forge.LiqoNodeName).WithUID(virtualNodeOwner.uid).WithBlockOwnerDeletion(false), nil
}

// conditionsApply converts the given conditions into the corresponding apply configurations.
func conditionsApply(conditions []metav1.Condition) []*metav1apply.ConditionApplyConfiguration {
	result := make([]*metav1apply.ConditionApplyConfiguration, 0, len(conditions))
	for i := range conditions {
		c := &conditions[i]
		result = append(result, metav1apply.Condition().WithType(c.Type).WithStatus(c.Status).WithReason(c.Reason).
			WithMessage(c.Message).WithLastTransitionTime(c.LastTransitionTime).WithObservedGeneration(c.ObservedGeneration))
	}
	return result
}

// failedCondition returns a condition reporting that the object could not be reflected to the remote cluster.
// The last transition time of the previous condition of the same type is preserved, if the status is unchanged,
// so that the shadow resource is not modified again when the reflection keeps failing for the same reason.
func failedCondition(previous []metav1.Condition, conditionType, message string, generation int64) metav1.Condition {
	return newCondition(previous, conditionType, metav1.ConditionFalse, forge.ConditionReasonReflectionFailed, message, generation)
}

// newCondition returns a condition with the given parameters, preserving the last transition time of the previous
// condition of the same type, if the status is unchanged.
func newCondition(previous []metav1.Condition, conditionType string, status metav1.ConditionStatus,
	reason, message string, generation int64) metav1.Condition {
	condition := metav1.Condition{
		Type: conditionType, Status: status, Reason: reason,
		Message: message, ObservedGeneration: generation, LastTransitionTime: metav1.Now(),
	}
	if p := meta.FindStatusCondition(previous, conditionType); p != nil && p.Status == condition.Status {
		condition.LastTransitionTime = p.LastTransitionTime
	}
	return condition
}

// gatewayStatusReflector reflects the status of the remote Gateways through ShadowGatewayStatus resources.
type gatewayStatusReflector struct {
	namespace string
	kube      kubernetes.Interface
	client    offloadingv1beta1clients.ShadowGatewayStatusInterface
	lister    offloadingv1beta1listers.ShadowGatewayStatusNamespaceLister

	// remoteRoutes list the reflected routes in the remote namespace, which report the addresses of the shared Gateway.
	remoteRoutes []remoteRoutesLister
}

func newGatewayStatusReflector(opts *options.NamespacedOpts, _ *forge.GatewayAPIForgingOpts) statusReflector[*gwv1.Gateway] {
	return &gatewayStatusReflector{
		namespace: opts.LocalNamespace,
		kube:      opts.LocalClient,
		client:    opts.LocalLiqoClient.OffloadingV1beta1().ShadowGatewayStatuses(opts.LocalNamespace),
		lister:    opts.LocalLiqoFactory.Offloading().V1beta1().ShadowGatewayStatuses().Lister().ShadowGatewayStatuses(opts.LocalNamespace),
	}
}

func (gsr *gatewayStatusReflector) Enforce(ctx context.Context, local, remote *gwv1.Gateway) error {
	return gsr.apply(ctx, local.GetName(), offloadingv1beta1apply.ShadowGatewayStatusSpec().
		WithAddresses(remote.Status.Addresses...).
		WithConditions(conditionsApply(remote.Status.Conditions)...).
		WithListeners(remote.Status.Listeners...))
}

func (gsr *gatewayStatusReflector) Fail(ctx context.Context, local *gwv1.Gateway, message string) error {
	var previous []metav1.Condition
	if shadow, err := gsr.lister.Get(ShadowName("", local.GetName())); err == nil {
		previous = shadow.Spec.Conditions
	}

	conditions := []metav1.Condition{
		failedCondition(previous, string(gwv1.GatewayConditionAccepted), message, local.GetGeneration()),
		failedCondition(previous, string(gwv1.GatewayConditionProgrammed), message, local.GetGeneration()),
	}
	return gsr.apply(ctx, local.GetName(), offloadingv1beta1apply.ShadowGatewayStatusSpec().WithConditions(conditionsApply(conditions)...))
}

// apply enforces the shadow resource associated with the given local Gateway, completing the given spec.
func (gsr *gatewayStatusReflector) apply(ctx context.Context, name string,
	spec *offloadingv1beta1apply.ShadowGatewayStatusSpecApplyConfiguration) error {
	owner, err := virtualNodeOwnerReference(ctx, gsr.kube)
	if err != nil {
		return err
	}

	shadow := offloadingv1beta1apply.ShadowGatewayStatus(ShadowName("", name), gsr.namespace).
		WithLabels(shadowLabels()).WithOwnerReferences(owner).
		WithSpec(spec.WithGatewayName(name).WithClusterID(string(forge.RemoteCluster)))

	if _, err := gsr.client.Apply(ctx, shadow, forge.ApplyOptions()); err != nil {
		return fmt.Errorf("failed to enforce ShadowGatewayStatus %q: %w", klog.KRef(gsr.namespace, *shadow.Name), err)
	}
	return nil
}

func (gsr *gatewayStatusReflector) Delete(ctx context.Context, name string) error {
	shadow := ShadowName("", name)
	if _, err := gsr.lister.Get(shadow); kerrors.IsNotFound(err) {
		return nil
	}
	if err := gsr.client.Delete(ctx, shadow, metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ShadowGatewayStatus %q: %w", klog.KRef(gsr.namespace, shadow), err)
	}
	return nil
}

func (gsr *gatewayStatusReflector) Cleanup(ctx context.Context) error {
	shadows, err := gsr.lister.List(shadowSelector())
	if err != nil {
		return fmt.Errorf("failed to list ShadowGatewayStatuses for cleanup: %w", err)
	}
	for _, shadow := range shadows {
		if err := gsr.client.Delete(ctx, shadow.GetName(), metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ShadowGatewayStatus %q: %w", klog.KObj(shadow), err)
		}
	}
	return nil
}

// routeStatusReflector reflects the status of the remote routes through ShadowRouteStatus resources.
type routeStatusReflector[O object] struct {
	kind     offloadingv1beta1.RouteKind
	parents  func(route O) []gwv1.ParentReference
	statuses func(route O) []gwv1.RouteParentStatus

	namespace       string
	remoteNamespace string
	forgingOpts     *forge.GatewayAPIForgingOpts

	kube   kubernetes.Interface
	client offloadingv1beta1clients.ShadowRouteStatusInterface
	lister offloadingv1beta1listers.ShadowRouteStatusNamespaceLister
}

func newRouteStatusReflector[O object](kind offloadingv1beta1.RouteKind, parents func(route O) []gwv1.ParentReference,
	statuses func(route O) []gwv1.RouteParentStatus) func(*options.NamespacedOpts, *forge.GatewayAPIForgingOpts) statusReflector[O] {
	return func(opts *options.NamespacedOpts, forgingOpts *forge.GatewayAPIForgingOpts) statusReflector[O] {
		return &routeStatusReflector[O]{
			kind: kind, parents: parents, statuses: statuses,
			namespace: opts.LocalNamespace, remoteNamespace: opts.RemoteNamespace, forgingOpts: forgingOpts,
			kube:   opts.LocalClient,
			client: opts.LocalLiqoClient.OffloadingV1beta1().ShadowRouteStatuses(opts.LocalNamespace),
			lister: opts.LocalLiqoFactory.Offloading().V1beta1().ShadowRouteStatuses().Lister().ShadowRouteStatuses(opts.LocalNamespace),
		}
	}
}

func (rsr *routeStatusReflector[O]) shadowName(name string) string {
	return ShadowName(strings.ToLower(string(rsr.kind)), name)
}

func (rsr *routeStatusReflector[O]) Enforce(ctx context.Context, local, remote O) error {
	// The references to the remote parents are translated to the ones of the local route.
	shared := forge.ResolvedSharedGateway(remote)
	parents := forge.LocalRouteParentStatuses(rsr.namespace, rsr.remoteNamespace, rsr.parents(local), rsr.statuses(remote),
		shared, rsr.forgingOpts)

	// The placeholder of the shared Gateway is replaced by the remote cluster when the route is applied: if not replaced,
	// the remote cluster does not offer any shared Gateway (anymore), and the corresponding parents are reported as failed.
	if shared == nil {
		var placeholders []gwv1.ParentReference
		for _, parent := range forge.ReflectedRouteParents(rsr.namespace, rsr.parents(local), rsr.forgingOpts) {
			if remoteParent, _ := forge.RemoteParentRef(rsr.namespace, parent, rsr.forgingOpts); forge.IsSharedGatewayPlaceholder(&remoteParent) {
				placeholders = append(placeholders, parent)
			}
		}
		parents = append(parents, rsr.failedParents(local, placeholders,
			"No shared Gateway offered by the remote cluster, which the route can be attached to")...)
	}
	return rsr.apply(ctx, local.GetName(), parents)
}

func (rsr *routeStatusReflector[O]) Fail(ctx context.Context, local O, message string) error {
	// The failure is reported for each local parent the route would be attached to in the remote cluster.
	reflected := forge.ReflectedRouteParents(rsr.namespace, rsr.parents(local), rsr.forgingOpts)
	if len(reflected) == 0 {
		// The route would not be attached to any parent in the remote cluster, hence no status shall be reported.
		return rsr.Delete(ctx, local.GetName())
	}

	return rsr.apply(ctx, local.GetName(), rsr.failedParents(local, reflected, message))
}

// failedParents returns the statuses reporting the failure, with the given message, for the given parents of the local route.
func (rsr *routeStatusReflector[O]) failedParents(local O, reflected []gwv1.ParentReference, message string) []gwv1.RouteParentStatus {
	var previous []gwv1.RouteParentStatus
	if shadow, err := rsr.lister.Get(rsr.shadowName(local.GetName())); err == nil {
		previous = shadow.Spec.Parents
	}

	parents := make([]gwv1.RouteParentStatus, 0, len(reflected))
	for i := range reflected {
		var conditions []metav1.Condition
		for j := range previous {
			// The parent references are compared after normalization, since the stored ones include the default values.
			if forge.ParentRefKey(&previous[j].ParentRef, rsr.namespace) == forge.ParentRefKey(&reflected[i], rsr.namespace) {
				conditions = previous[j].Conditions
			}
		}

		parents = append(parents, gwv1.RouteParentStatus{
			ParentRef:      reflected[i],
			ControllerName: consts.GatewayControllerName,
			Conditions: []metav1.Condition{
				failedCondition(conditions, string(gwv1.RouteConditionAccepted), message, local.GetGeneration()),
			},
		})
	}
	return parents
}

// apply enforces the shadow resource associated with the given local route, reporting the given parent statuses.
func (rsr *routeStatusReflector[O]) apply(ctx context.Context, name string, parents []gwv1.RouteParentStatus) error {
	owner, err := virtualNodeOwnerReference(ctx, rsr.kube)
	if err != nil {
		return err
	}

	shadow := offloadingv1beta1apply.ShadowRouteStatus(rsr.shadowName(name), rsr.namespace).
		WithLabels(shadowLabels()).WithOwnerReferences(owner).
		WithSpec(offloadingv1beta1apply.ShadowRouteStatusSpec().
			WithKind(rsr.kind).WithRouteName(name).WithClusterID(string(forge.RemoteCluster)).
			WithParents(parents...))

	if _, err := rsr.client.Apply(ctx, shadow, forge.ApplyOptions()); err != nil {
		return fmt.Errorf("failed to enforce ShadowRouteStatus %q: %w", klog.KRef(rsr.namespace, *shadow.Name), err)
	}
	return nil
}

func (rsr *routeStatusReflector[O]) Delete(ctx context.Context, name string) error {
	shadow := rsr.shadowName(name)
	if _, err := rsr.lister.Get(shadow); kerrors.IsNotFound(err) {
		return nil
	}
	if err := rsr.client.Delete(ctx, shadow, metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ShadowRouteStatus %q: %w", klog.KRef(rsr.namespace, shadow), err)
	}
	return nil
}

func (rsr *routeStatusReflector[O]) Cleanup(ctx context.Context) error {
	shadows, err := rsr.lister.List(shadowSelector())
	if err != nil {
		return fmt.Errorf("failed to list ShadowRouteStatuses for cleanup: %w", err)
	}
	for _, shadow := range shadows {
		// Each route reflector is responsible for the shadow resources of its own kind.
		if shadow.Spec.Kind != rsr.kind {
			continue
		}
		if err := rsr.client.Delete(ctx, shadow.GetName(), metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ShadowRouteStatus %q: %w", klog.KObj(shadow), err)
		}
	}
	return nil
}

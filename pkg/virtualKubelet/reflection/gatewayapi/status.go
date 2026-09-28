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
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// statusReflector reflects the status of the remote objects back to the local cluster, through shadow resources
// which are then aggregated by the controller manager, since the same object may be reflected to multiple clusters.
type statusReflector[O object] interface {
	// Enforce ensures the shadow resource reports the status of the given remote object.
	Enforce(ctx context.Context, local, remote O) error
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

// gatewayStatusReflector reflects the status of the remote Gateways through ShadowGatewayStatus resources.
type gatewayStatusReflector struct {
	namespace string
	kube      kubernetes.Interface
	client    offloadingv1beta1clients.ShadowGatewayStatusInterface
	lister    offloadingv1beta1listers.ShadowGatewayStatusNamespaceLister
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
	owner, err := virtualNodeOwnerReference(ctx, gsr.kube)
	if err != nil {
		return err
	}

	shadow := offloadingv1beta1apply.ShadowGatewayStatus(ShadowName("", local.GetName()), gsr.namespace).
		WithLabels(shadowLabels()).WithOwnerReferences(owner).
		WithSpec(offloadingv1beta1apply.ShadowGatewayStatusSpec().
			WithGatewayName(local.GetName()).WithClusterID(string(forge.RemoteCluster)).
			WithAddresses(remote.Status.Addresses...).
			WithConditions(conditionsApply(remote.Status.Conditions)...).
			WithListeners(remote.Status.Listeners...))

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
	owner, err := virtualNodeOwnerReference(ctx, rsr.kube)
	if err != nil {
		return err
	}

	// The references to the remote parents are translated to the ones of the local route.
	parents := forge.LocalRouteParentStatuses(rsr.namespace, rsr.remoteNamespace, rsr.parents(local), rsr.statuses(remote), rsr.forgingOpts)

	shadow := offloadingv1beta1apply.ShadowRouteStatus(rsr.shadowName(local.GetName()), rsr.namespace).
		WithLabels(shadowLabels()).WithOwnerReferences(owner).
		WithSpec(offloadingv1beta1apply.ShadowRouteStatusSpec().
			WithKind(rsr.kind).WithRouteName(local.GetName()).WithClusterID(string(forge.RemoteCluster)).
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

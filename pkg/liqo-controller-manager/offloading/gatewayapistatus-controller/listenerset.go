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

package gatewayapistatusctrl

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// maxListenerSupportedKinds is the maximum number of supported kinds of a listener, as enforced by the Gateway API validation.
const maxListenerSupportedKinds = 8

// ListenerSetReconciler aggregates the status of the ListenerSets attached to the Gateways of the virtual class, reflected to
// the remote clusters (reported through the ShadowListenerSetStatus resources), into the local ListenerSets, as Liqo acts as
// their controller. The ListenerSets attached to other Gateways are managed by the controllers of their Gateways (e.g., the
// ones reflected from the consumer clusters, attached to the shared Gateway), hence their status is never modified.
type ListenerSetReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=listenersets,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=listenersets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways;gatewayclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=shadowlistenersetstatuses,verbs=get;list;watch;delete

// Reconcile aggregates the status of the given ListenerSet.
func (r *ListenerSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var shadows offloadingv1beta1.ShadowListenerSetStatusList
	if err := r.List(ctx, &shadows, client.InNamespace(req.Namespace), client.MatchingFields{listenerSetNameField: req.Name}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list the ShadowListenerSetStatuses of ListenerSet %q: %w", req.NamespacedName, err)
	}

	var listenerSet gwv1.ListenerSet
	if err := r.Get(ctx, req.NamespacedName, &listenerSet); err != nil {
		if kerrors.IsNotFound(err) {
			// The ListenerSet has been deleted: remove the orphan ShadowListenerSetStatuses.
			return ctrl.Result{}, deleteAll(ctx, r.Client, shadows.Items)
		}
		return ctrl.Result{}, err
	}

	gateway, err := virtualParentGateway(ctx, r.Client, &listenerSet)
	if err != nil || gateway == nil {
		if gateway == nil && err == nil {
			klog.V(4).Infof("Skipping ListenerSet %q, as it is not attached to a Gateway of a virtual class", req.NamespacedName)
		}
		return ctrl.Result{}, err
	}

	original := listenerSet.Status.DeepCopy()
	AggregateListenerSetStatus(&listenerSet, forge.ListenerSetAllowed(gateway, listenerSet.GetNamespace()), shadows.Items)
	if equality.Semantic.DeepEqual(original, &listenerSet.Status) {
		return ctrl.Result{}, nil
	}

	if err := r.Status().Update(ctx, &listenerSet); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update the status of ListenerSet %q: %w", req.NamespacedName, err)
	}
	klog.V(4).Infof("Status of ListenerSet %q aggregated from %d remote cluster(s)", req.NamespacedName, len(shadows.Items))
	return ctrl.Result{}, nil
}

// virtualParentGateway returns the Gateway the given ListenerSet is attached to, if it belongs to a virtual class (i.e., a class
// managed by Liqo), and nil otherwise, including if either the Gateway or its class do not exist (as not managed by Liqo).
func virtualParentGateway(ctx context.Context, cl client.Client, listenerSet *gwv1.ListenerSet) (*gwv1.Gateway, error) {
	parent, ok := forge.ListenerSetParent(listenerSet)
	if !ok {
		return nil, nil
	}

	var gateway gwv1.Gateway
	if err := cl.Get(ctx, parent, &gateway); err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	var class gwv1.GatewayClass
	if err := cl.Get(ctx, types.NamespacedName{Name: string(gateway.Spec.GatewayClassName)}, &class); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	if class.Spec.ControllerName != controllerName {
		return nil, nil
	}
	return &gateway, nil
}

// AggregateListenerSetStatus aggregates the statuses reported by the remote clusters into the status of the given ListenerSet,
// attached to a Gateway of the virtual class. If the Gateway does not allow its attachment, the ListenerSet is not reflected,
// and it is reported as not accepted.
func AggregateListenerSetStatus(listenerSet *gwv1.ListenerSet, allowed bool, shadows []offloadingv1beta1.ShadowListenerSetStatus) {
	generation := listenerSet.GetGeneration()
	if !allowed {
		message := fmt.Sprintf("The parent Gateway does not allow the attachment of the ListenerSets in namespace %q "+
			"(allowedListeners), hence the ListenerSet is not reflected to the remote clusters", listenerSet.GetNamespace())
		meta.SetStatusCondition(&listenerSet.Status.Conditions, metav1.Condition{Type: string(gwv1.ListenerSetConditionAccepted),
			Status: metav1.ConditionFalse, Reason: string(gwv1.ListenerSetReasonNotAllowed), Message: message, ObservedGeneration: generation})
		meta.SetStatusCondition(&listenerSet.Status.Conditions, metav1.Condition{Type: string(gwv1.ListenerSetConditionProgrammed),
			Status: metav1.ConditionFalse, Reason: string(gwv1.ListenerSetReasonInvalid), Message: message, ObservedGeneration: generation})
		listenerSet.Status.Listeners = nil
		return
	}

	sort.Slice(shadows, func(i, j int) bool { return shadows[i].Spec.ClusterID < shadows[j].Spec.ClusterID })

	reported := make([]clusterConditions, 0, len(shadows))
	for i := range shadows {
		reported = append(reported, clusterConditions{cluster: shadows[i].Spec.ClusterID, conditions: shadows[i].Spec.Conditions})
	}
	listenerSet.Status.Conditions = aggregateAllConditions(listenerSet.Status.Conditions, reported, generation,
		string(gwv1.ListenerSetConditionAccepted), string(gwv1.ListenerSetConditionProgrammed))

	listeners := make([]gwv1.ListenerEntryStatus, 0, len(listenerSet.Spec.Listeners))
	for i := range listenerSet.Spec.Listeners {
		listeners = append(listeners, aggregateListenerEntryStatus(listenerSet, listenerSet.Spec.Listeners[i].Name, shadows))
	}
	listenerSet.Status.Listeners = listeners
}

// aggregateListenerEntryStatus aggregates the statuses of the given listener of a ListenerSet reported by the remote clusters.
func aggregateListenerEntryStatus(listenerSet *gwv1.ListenerSet, name gwv1.SectionName,
	shadows []offloadingv1beta1.ShadowListenerSetStatus) gwv1.ListenerEntryStatus {
	status := gwv1.ListenerEntryStatus{Name: name, SupportedKinds: []gwv1.RouteGroupKind{}}
	var previous []metav1.Condition
	for i := range listenerSet.Status.Listeners {
		if listenerSet.Status.Listeners[i].Name == name {
			// Preserve the existing conditions, to keep the last transition times.
			previous = listenerSet.Status.Listeners[i].Conditions
		}
	}

	kinds := make(map[string]bool)
	var reported []clusterConditions
	for i := range shadows {
		for j := range shadows[i].Spec.Listeners {
			listener := &shadows[i].Spec.Listeners[j]
			if listener.Name != name {
				continue
			}

			status.AttachedRoutes += listener.AttachedRoutes
			for _, kind := range listener.SupportedKinds {
				key := fmt.Sprintf("%s/%s", ptr.Deref(kind.Group, gwv1.GroupName), kind.Kind)
				if !kinds[key] && len(status.SupportedKinds) < maxListenerSupportedKinds {
					status.SupportedKinds = append(status.SupportedKinds, kind)
					kinds[key] = true
				}
			}
			reported = append(reported, clusterConditions{cluster: shadows[i].Spec.ClusterID, conditions: listener.Conditions})
		}
	}

	// The Conflicted condition is True if the listener conflicts in at least one remote cluster (e.g., since the
	// hostname is already used by an older ListenerSet), consistently with the aggregation of the other conditions.
	status.Conditions = aggregateAllConditions(previous, reported, listenerSet.GetGeneration(),
		string(gwv1.ListenerEntryConditionAccepted), string(gwv1.ListenerEntryConditionProgrammed))
	return status
}

// clusterConditions associates the conditions of an object with the cluster reporting them.
type clusterConditions struct {
	cluster    string
	conditions []metav1.Condition
}

// aggregateAllConditions aggregates all the conditions reported by the remote clusters, by type, preserving the last
// transition times of the previous ones. The conditions no longer reported by any remote cluster are removed, except for
// the required ones, which are reported as pending if not (yet) reported by any remote cluster.
func aggregateAllConditions(previous []metav1.Condition, reported []clusterConditions, generation int64,
	required ...string) []metav1.Condition {
	grouped := make(map[string][]clusterCondition)
	for i := range reported {
		for j := range reported[i].conditions {
			condition := &reported[i].conditions[j]
			grouped[condition.Type] = append(grouped[condition.Type], clusterCondition{cluster: reported[i].cluster, condition: condition})
		}
	}

	types := make([]string, 0, len(grouped)+len(required))
	for conditionType := range grouped {
		types = append(types, conditionType)
	}
	for _, conditionType := range required {
		if _, found := grouped[conditionType]; !found {
			types = append(types, conditionType)
		}
	}
	sort.Strings(types)

	conditions := make([]metav1.Condition, 0, len(types))
	for _, conditionType := range types {
		if p := meta.FindStatusCondition(previous, conditionType); p != nil {
			conditions = append(conditions, *p)
		}
	}

	for _, conditionType := range types {
		var result metav1.Condition
		if reports, found := grouped[conditionType]; found {
			result, _ = aggregateConditions(conditionType, reports, generation)
		} else {
			message := "Not yet reflected to any remote cluster"
			if len(reported) > 0 {
				message = "Not yet reported by any remote cluster"
			}
			result = metav1.Condition{Type: conditionType, Status: metav1.ConditionUnknown, Reason: string(gwv1.ListenerSetReasonPending),
				Message: message, ObservedGeneration: generation}
		}
		setCondition(&conditions, &result)
	}
	return conditions
}

// attachedListenerSets returns the number of ListenerSets successfully attached to the given Gateway of the virtual class,
// that is, the ones allowed by the Gateway and accepted in at least one remote cluster.
func attachedListenerSets(ctx context.Context, cl client.Client, gateway *gwv1.Gateway) (int32, error) {
	var listenerSets gwv1.ListenerSetList
	key := types.NamespacedName{Namespace: gateway.GetNamespace(), Name: gateway.GetName()}.String()
	if err := cl.List(ctx, &listenerSets, client.MatchingFields{listenerSetParentField: key}); err != nil {
		return 0, fmt.Errorf("failed to list the ListenerSets attached to Gateway %q: %w", key, err)
	}

	var attached int32
	for i := range listenerSets.Items {
		listenerSet := &listenerSets.Items[i]
		if forge.ListenerSetAllowed(gateway, listenerSet.GetNamespace()) &&
			meta.IsStatusConditionTrue(listenerSet.Status.Conditions, string(gwv1.ListenerSetConditionAccepted)) {
			attached++
		}
	}
	return attached, nil
}

// listenerSetParentIndexer indexes the ListenerSets by the Gateway they are attached to (in the form <namespace>/<name>).
func listenerSetParentIndexer(obj client.Object) []string {
	listenerSet, ok := obj.(*gwv1.ListenerSet)
	if !ok {
		return nil
	}
	parent, ok := forge.ListenerSetParent(listenerSet)
	if !ok {
		return nil
	}
	return []string{parent.String()}
}

// SetupWithManager sets up the controller with the Manager. The ListenerSets are enqueued also when their Gateway changes,
// since it determines whether they are managed by Liqo, and whether their attachment is allowed.
func (r *ListenerSetReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, workers int) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &offloadingv1beta1.ShadowListenerSetStatus{},
		listenerSetNameField, listenerSetNameIndexer); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlShadowListenerSetStatus).
		For(&gwv1.ListenerSet{}).
		Watches(&offloadingv1beta1.ShadowListenerSetStatus{}, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, obj client.Object) []reconcile.Request {
				shadow, ok := obj.(*offloadingv1beta1.ShadowListenerSetStatus)
				if !ok {
					return nil
				}
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: shadow.Namespace, Name: shadow.Spec.ListenerSetName}}}
			})).
		Watches(&gwv1.Gateway{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				return attachedListenerSetsRequests(ctx, r.Client, obj)
			})).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

// attachedListenerSetsRequests returns the requests to reconcile the ListenerSets attached to the given Gateway.
func attachedListenerSetsRequests(ctx context.Context, cl client.Client, gateway client.Object) []reconcile.Request {
	var listenerSets gwv1.ListenerSetList
	key := types.NamespacedName{Namespace: gateway.GetNamespace(), Name: gateway.GetName()}.String()
	if err := cl.List(ctx, &listenerSets, client.MatchingFields{listenerSetParentField: key}); err != nil {
		klog.Errorf("Failed to list the ListenerSets attached to Gateway %q: %v", key, err)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(listenerSets.Items))
	for i := range listenerSets.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&listenerSets.Items[i])})
	}
	return requests
}

// SetupListenerSetParentIndexer registers the field indexer of the ListenerSets by the Gateway they are attached to,
// shared by the ListenerSet and Gateway reconcilers.
func SetupListenerSetParentIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &gwv1.ListenerSet{}, listenerSetParentField, listenerSetParentIndexer)
}

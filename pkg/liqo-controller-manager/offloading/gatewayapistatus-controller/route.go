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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// maxRouteParents is the maximum number of parents in the status of a route, as enforced by the Gateway API validation.
const maxRouteParents = 32

// EventPartiallyAccepted is the reason of the event recorded on the routes accepted only by part of the remote clusters.
const EventPartiallyAccepted = "PartiallyAccepted"

// RouteReconciler aggregates the status of the routes of a given kind reflected to the remote clusters (reported
// through the ShadowRouteStatus resources) into the local routes, adding one entry for each local parent reflected.
type RouteReconciler struct {
	client.Client

	// Kind is the kind of routes managed by the reconciler.
	Kind offloadingv1beta1.RouteKind
	// Recorder records the events on the routes accepted only by part of the remote clusters.
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes;grpcroutes,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status;grpcroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=shadowroutestatuses,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile aggregates the status of the given route.
func (r *RouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var shadows offloadingv1beta1.ShadowRouteStatusList
	if err := r.List(ctx, &shadows, client.InNamespace(req.Namespace),
		client.MatchingFields{routeNameField: routeIndexKey(r.Kind, req.Name)}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list the ShadowRouteStatuses of %s %q: %w", r.Kind, req.NamespacedName, err)
	}

	route, parents := r.newRoute()
	if err := r.Get(ctx, req.NamespacedName, route); err != nil {
		if kerrors.IsNotFound(err) {
			// The route has been deleted: remove the orphan ShadowRouteStatuses.
			return ctrl.Result{}, deleteAll(ctx, r.Client, shadows.Items)
		}
		return ctrl.Result{}, err
	}

	original := make([]gwv1.RouteParentStatus, len(*parents))
	for i := range *parents {
		(*parents)[i].DeepCopyInto(&original[i])
	}

	var degraded []string
	*parents, degraded = AggregateRouteParents(*parents, shadows.Items, route.GetNamespace(), route.GetGeneration())
	if equality.Semantic.DeepEqual(original, *parents) {
		return ctrl.Result{}, nil
	}

	// The event is recorded only when the status changes, hence when the set of clusters not accepting the route changes.
	if len(degraded) > 0 && r.Recorder != nil {
		r.Recorder.Eventf(route, nil, corev1.EventTypeWarning, EventPartiallyAccepted, "AggregateStatus",
			"%s accepted only by part of the remote clusters: %s", r.Kind, strings.Join(degraded, "; "))
	}

	if err := r.Status().Update(ctx, route); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update the status of %s %q: %w", r.Kind, req.NamespacedName, err)
	}
	klog.V(4).Infof("Status of %s %q aggregated from %d remote cluster(s)", r.Kind, req.NamespacedName, len(shadows.Items))
	return ctrl.Result{}, nil
}

// newRoute returns a new route of the managed kind, and a pointer to the parents in its status.
func (r *RouteReconciler) newRoute() (client.Object, *[]gwv1.RouteParentStatus) {
	switch r.Kind {
	case offloadingv1beta1.GRPCRouteKind:
		route := &gwv1.GRPCRoute{}
		return route, &route.Status.Parents
	default:
		route := &gwv1.HTTPRoute{}
		return route, &route.Status.Parents
	}
}

// AggregateRouteParents aggregates the statuses reported by the remote clusters into the parents of the status of a route.
// The entries managed by other controllers are preserved, while the ones managed by Liqo are replaced. It also returns
// the description of the parents accepted only by part of the remote clusters, along with the clusters not accepting them.
func AggregateRouteParents(current []gwv1.RouteParentStatus, shadows []offloadingv1beta1.ShadowRouteStatus,
	namespace string, generation int64) (parents []gwv1.RouteParentStatus, degraded []string) {
	parents = make([]gwv1.RouteParentStatus, 0, len(current))
	existing := make(map[string]*gwv1.RouteParentStatus)
	for i := range current {
		if current[i].ControllerName == controllerName {
			existing[parentRefKey(&current[i].ParentRef, namespace)] = &current[i]
			continue
		}
		parents = append(parents, current[i])
	}

	// Group the conditions reported by the remote clusters by local parent and type.
	type reported struct {
		ref        gwv1.ParentReference
		conditions map[string][]clusterCondition
	}
	grouped := make(map[string]*reported)
	for i := range shadows {
		for j := range shadows[i].Spec.Parents {
			parent := &shadows[i].Spec.Parents[j]
			key := parentRefKey(&parent.ParentRef, namespace)
			if grouped[key] == nil {
				grouped[key] = &reported{ref: parent.ParentRef, conditions: make(map[string][]clusterCondition)}
			}
			for k := range parent.Conditions {
				condition := &parent.Conditions[k]
				grouped[key].conditions[condition.Type] = append(grouped[key].conditions[condition.Type],
					clusterCondition{cluster: shadows[i].Spec.ClusterID, condition: condition})
			}
		}
	}

	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if len(parents) >= maxRouteParents {
			klog.Warningf("Too many parents in the status of the route: %d entries managed by Liqo dropped", len(keys))
			break
		}

		status := gwv1.RouteParentStatus{ParentRef: grouped[key].ref, ControllerName: controllerName, Conditions: []metav1.Condition{}}
		if previous, found := existing[key]; found {
			// Preserve the existing conditions, to keep the last transition times, except for the ones no longer
			// reported by any remote cluster (e.g., conditions removed by the remote controllers once satisfied).
			for i := range previous.Conditions {
				if _, reported := grouped[key].conditions[previous.Conditions[i].Type]; reported {
					status.Conditions = append(status.Conditions, previous.Conditions[i])
				}
			}
		}

		conditionTypes := make([]string, 0, len(grouped[key].conditions))
		for conditionType := range grouped[key].conditions {
			conditionTypes = append(conditionTypes, conditionType)
		}
		sort.Strings(conditionTypes)
		for _, conditionType := range conditionTypes {
			aggregated, notAccepted := aggregateConditions(conditionType, grouped[key].conditions[conditionType], generation)
			setCondition(&status.Conditions, &aggregated)
			if conditionType == string(gwv1.RouteConditionAccepted) && len(notAccepted) > 0 {
				degraded = append(degraded, fmt.Sprintf("parent %q not accepted in %s", grouped[key].ref.Name, strings.Join(notAccepted, ", ")))
			}
		}
		parents = append(parents, status)
	}

	return parents, degraded
}

// setCondition sets the given condition, preserving the last transition time if the status is unchanged.
func setCondition(conditions *[]metav1.Condition, condition *metav1.Condition) {
	for i := range *conditions {
		if (*conditions)[i].Type == condition.Type {
			condition.LastTransitionTime = (*conditions)[i].LastTransitionTime
			if (*conditions)[i].Status != condition.Status || condition.LastTransitionTime.IsZero() {
				condition.LastTransitionTime = metav1.Now()
			}
			(*conditions)[i] = *condition
			return
		}
	}
	condition.LastTransitionTime = metav1.Now()
	*conditions = append(*conditions, *condition)
}

// SetupWithManager sets up the controller with the Manager. The field indexer is shared by the route reconcilers,
// hence it shall be registered only once, through the SetupRouteIndexer function.
func (r *RouteReconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	route, _ := r.newRoute()
	name := consts.CtrlShadowHTTPRouteStatus
	if r.Kind == offloadingv1beta1.GRPCRouteKind {
		name = consts.CtrlShadowGRPCRouteStatus
	}

	return ctrl.NewControllerManagedBy(mgr).Named(name).
		For(route).
		Watches(&offloadingv1beta1.ShadowRouteStatus{}, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, obj client.Object) []reconcile.Request {
				shadow, ok := obj.(*offloadingv1beta1.ShadowRouteStatus)
				if !ok || shadow.Spec.Kind != r.Kind {
					return nil
				}
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: shadow.Namespace, Name: shadow.Spec.RouteName}}}
			})).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

// SetupRouteIndexer registers the field indexer of the ShadowRouteStatuses, shared by the route reconcilers.
func SetupRouteIndexer(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &offloadingv1beta1.ShadowRouteStatus{}, routeNameField, routeNameIndexer)
}

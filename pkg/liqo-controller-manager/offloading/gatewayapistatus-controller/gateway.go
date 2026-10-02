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

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// GatewayReconciler aggregates the status of the Gateways of the virtual class reflected to the remote clusters
// (reported through the ShadowGatewayStatus resources) into the local Gateways, as Liqo acts as their controller.
type GatewayReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=shadowgatewaystatuses,verbs=get;list;watch;delete

// Reconcile aggregates the status of the given Gateway.
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var shadows offloadingv1beta1.ShadowGatewayStatusList
	if err := r.List(ctx, &shadows, client.InNamespace(req.Namespace), client.MatchingFields{gatewayNameField: req.Name}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list the ShadowGatewayStatuses of Gateway %q: %w", req.NamespacedName, err)
	}

	var gateway gwv1.Gateway
	if err := r.Get(ctx, req.NamespacedName, &gateway); err != nil {
		if kerrors.IsNotFound(err) {
			// The Gateway has been deleted: remove the orphan ShadowGatewayStatuses.
			return ctrl.Result{}, deleteAll(ctx, r.Client, shadows.Items)
		}
		return ctrl.Result{}, err
	}

	// The status is managed only for the Gateways belonging to the virtual classes.
	var class gwv1.GatewayClass
	if err := r.Get(ctx, types.NamespacedName{Name: string(gateway.Spec.GatewayClassName)}, &class); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if class.Spec.ControllerName != controllerName {
		return ctrl.Result{}, nil
	}

	original := gateway.Status.DeepCopy()
	AggregateGatewayStatus(&gateway, shadows.Items)
	if equality.Semantic.DeepEqual(original, &gateway.Status) {
		return ctrl.Result{}, nil
	}

	if err := r.Status().Update(ctx, &gateway); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update the status of Gateway %q: %w", req.NamespacedName, err)
	}
	klog.V(4).Infof("Status of Gateway %q aggregated from %d remote cluster(s)", req.NamespacedName, len(shadows.Items))
	return ctrl.Result{}, nil
}

// AggregateGatewayStatus aggregates the statuses reported by the remote clusters into the status of the given Gateway.
func AggregateGatewayStatus(gateway *gwv1.Gateway, shadows []offloadingv1beta1.ShadowGatewayStatus) {
	sort.Slice(shadows, func(i, j int) bool { return shadows[i].Spec.ClusterID < shadows[j].Spec.ClusterID })

	// The addresses are the union of the ones assigned in all clusters.
	gateway.Status.Addresses = nil
	seen := make(map[string]bool)
	for i := range shadows {
		for _, address := range shadows[i].Spec.Addresses {
			key := fmt.Sprintf("%v/%s", address.Type, address.Value)
			if !seen[key] {
				gateway.Status.Addresses = append(gateway.Status.Addresses, address)
				seen[key] = true
			}
		}
	}

	meta.SetStatusCondition(&gateway.Status.Conditions, metav1.Condition{
		Type:   string(gwv1.GatewayConditionAccepted),
		Status: metav1.ConditionTrue,
		Reason: string(gwv1.GatewayReasonAccepted),
		Message: "The Gateway is handled by Liqo in the remote clusters where its namespace is offloaded " +
			"(either reflected, or mapped to the shared Gateway offered by the remote cluster)",
		ObservedGeneration: gateway.Generation,
	})
	meta.SetStatusCondition(&gateway.Status.Conditions, programmedCondition(shadows, gateway.Generation))

	listeners := make([]gwv1.ListenerStatus, 0, len(gateway.Spec.Listeners))
	for i := range gateway.Spec.Listeners {
		listeners = append(listeners, aggregateListenerStatus(gateway, gateway.Spec.Listeners[i].Name, shadows))
	}
	gateway.Status.Listeners = listeners
}

// aggregateListenerStatus aggregates the statuses of the given listener reported by the remote clusters.
func aggregateListenerStatus(gateway *gwv1.Gateway, name gwv1.SectionName, shadows []offloadingv1beta1.ShadowGatewayStatus) gwv1.ListenerStatus {
	status := gwv1.ListenerStatus{Name: name, SupportedKinds: []gwv1.RouteGroupKind{}}
	for i := range gateway.Status.Listeners {
		if gateway.Status.Listeners[i].Name == name {
			// Preserve the existing conditions, to keep the last transition times.
			status.Conditions = gateway.Status.Listeners[i].Conditions
		}
	}

	kinds := make(map[string]bool)
	var remote []offloadingv1beta1.ShadowGatewayStatus
	for i := range shadows {
		for j := range shadows[i].Spec.Listeners {
			listener := &shadows[i].Spec.Listeners[j]
			if listener.Name != name {
				continue
			}

			status.AttachedRoutes += listener.AttachedRoutes
			for _, kind := range listener.SupportedKinds {
				if key := fmt.Sprintf("%v/%s", kind.Group, kind.Kind); !kinds[key] {
					status.SupportedKinds = append(status.SupportedKinds, kind)
					kinds[key] = true
				}
			}
			remote = append(remote, offloadingv1beta1.ShadowGatewayStatus{Spec: offloadingv1beta1.ShadowGatewayStatusSpec{
				ClusterID: shadows[i].Spec.ClusterID, Conditions: listener.Conditions}})
		}
	}

	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type: string(gwv1.ListenerConditionAccepted), Status: metav1.ConditionTrue,
		Reason: string(gwv1.ListenerReasonAccepted), ObservedGeneration: gateway.Generation,
		Message: "The listener is handled by Liqo in the remote clusters where the Gateway namespace is offloaded",
	})
	meta.SetStatusCondition(&status.Conditions, programmedCondition(remote, gateway.Generation))
	return status
}

// programmedCondition returns the Programmed condition, which is True if the object is programmed in at least one remote cluster.
func programmedCondition(shadows []offloadingv1beta1.ShadowGatewayStatus, generation int64) metav1.Condition {
	condition := metav1.Condition{Type: string(gwv1.GatewayConditionProgrammed), ObservedGeneration: generation}
	if len(shadows) == 0 {
		condition.Status, condition.Reason = metav1.ConditionFalse, string(gwv1.GatewayReasonPending)
		condition.Message = "Not yet reflected to any remote cluster"
		return condition
	}

	var programmed, pending []string
	for i := range shadows {
		remote := meta.FindStatusCondition(shadows[i].Spec.Conditions, string(gwv1.GatewayConditionProgrammed))
		switch {
		case remote != nil && remote.Status == metav1.ConditionTrue:
			programmed = append(programmed, shadows[i].Spec.ClusterID)
		case remote != nil:
			pending = append(pending, fmt.Sprintf("cluster %q: %s", shadows[i].Spec.ClusterID, remote.Message))
		default:
			pending = append(pending, fmt.Sprintf("cluster %q: not yet programmed", shadows[i].Spec.ClusterID))
		}
	}

	if len(programmed) == 0 {
		condition.Status, condition.Reason = metav1.ConditionFalse, string(gwv1.GatewayReasonPending)
		condition.Message = strings.Join(pending, "; ")
		return condition
	}

	condition.Status, condition.Reason = metav1.ConditionTrue, string(gwv1.GatewayReasonProgrammed)
	condition.Message = fmt.Sprintf("Programmed in cluster(s) %s", strings.Join(programmed, ", "))
	if len(pending) > 0 {
		condition.Message += fmt.Sprintf(" (not programmed in %s)", strings.Join(pending, "; "))
	}
	return condition
}

// deleteAll deletes the given orphan shadow resources.
func deleteAll[T any, PT interface {
	*T
	client.Object
}](ctx context.Context, cl client.Client, objects []T) error {
	for i := range objects {
		obj := PT(&objects[i])
		err := cl.Delete(ctx, obj)
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete orphan %q: %w", client.ObjectKeyFromObject(obj), err)
		}
		klog.V(4).Infof("Orphan %q deleted", client.ObjectKeyFromObject(obj))
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, workers int) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &offloadingv1beta1.ShadowGatewayStatus{}, gatewayNameField, gatewayNameIndexer); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlShadowGatewayStatus).
		For(&gwv1.Gateway{}).
		Watches(&offloadingv1beta1.ShadowGatewayStatus{}, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, obj client.Object) []reconcile.Request {
				shadow, ok := obj.(*offloadingv1beta1.ShadowGatewayStatus)
				if !ok {
					return nil
				}
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: shadow.Namespace, Name: shadow.Spec.GatewayName}}}
			})).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

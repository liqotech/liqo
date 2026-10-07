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

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/liqotech/liqo/pkg/consts"
)

// GatewayClassReconciler marks the virtual GatewayClasses (i.e., the ones managed by Liqo) as accepted.
type GatewayClassReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses/status,verbs=get;update;patch

// Reconcile sets the Accepted condition of the virtual GatewayClasses.
func (r *GatewayClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var class gwv1.GatewayClass
	if err := r.Get(ctx, req.NamespacedName, &class); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	original := class.Status.DeepCopy()
	meta.SetStatusCondition(&class.Status.Conditions, metav1.Condition{
		Type:               string(gwv1.GatewayClassConditionStatusAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gwv1.GatewayClassReasonAccepted),
		Message:            "The Gateways of this class are reflected to the remote clusters where their namespace is offloaded",
		ObservedGeneration: class.Generation,
	})

	if equality.Semantic.DeepEqual(original, &class.Status) {
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, &class); err != nil {
		return ctrl.Result{}, err
	}
	klog.V(4).Infof("GatewayClass %q marked as accepted", req.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlGatewayClassStatus).
		For(&gwv1.GatewayClass{}, builder.WithPredicates(predicate.NewPredicateFuncs(isVirtualGatewayClass))).
		Complete(r)
}

func isVirtualGatewayClass(obj client.Object) bool {
	class, ok := obj.(*gwv1.GatewayClass)
	return ok && class.Spec.ControllerName == controllerName
}

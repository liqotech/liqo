// Copyright 2019-2021 The Liqo Authors
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

package foreignclusterreconciler

import (
	"context"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// ForeignClusterReconciler reconciles a ForeignCluster object.
type InducedForeignClusterReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// clusterRole
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/finalizers,verbs=get;update;patch
// Reconcile reconciles ForeignCluster resources.
func (r *InducedForeignClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	klog.V(4).Infof("Reconciling ForeignCluster %s", req.Name)
	var foreignCluster discoveryv1alpha1.ForeignCluster
	updateStatus := func() {
		if newErr := r.Client.Status().Update(ctx, &foreignCluster); newErr != nil {
			klog.Error(newErr)
			err = newErr
		}
	}
	if err := r.Client.Get(ctx, req.NamespacedName, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if foreignCluster.Spec.ClusterIdentity.ClusterID == "" {
		return result, nil
	}
	owner, err := foreigncluster.GetForeignClusterByID(ctx, r.Client, foreignCluster.Spec.InducedPeering.OriginClusterIdentity.ClusterID)
	if err != nil {
		klog.Error(err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if reflect.DeepEqual(foreignCluster.Status.TenantNamespace, owner.Status.TenantNamespace) {
		return result, nil
	}
	foreignCluster.Status.TenantNamespace = *owner.Status.TenantNamespace.DeepCopy()
	// defer the status update function
	defer updateStatus()

	return result, nil
}

// SetupWithManager assigns the operator to a manager.
func (r *InducedForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Prevent triggering a reconciliation in case of status modifications only.
	filterInducedPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      discovery.DiscoveryTypeLabel,
				Values:   []string{string(discovery.InducedPeeringDiscovery)},
				Operator: metav1.LabelSelectorOpIn,
			},
		},
	})
	if err != nil {
		klog.Error(err)
	}
	foreignClusterPredicate := predicate.And(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}), filterInducedPredicate)

	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ForeignCluster{}, builder.WithPredicates(foreignClusterPredicate)).
		Complete(r)
}

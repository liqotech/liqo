// Copyright 2019-2022 The Liqo Authors
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

package namespaceoffloadingctrl

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// NamespaceOffloadingReconciler adds/removes DesiredMapping to/from NamespaceMaps in according with
// ClusterSelector field.
type NamespaceOffloadingReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	LocalCluster discoveryv1alpha1.ClusterIdentity
}

const (
	namespaceOffloadingControllerFinalizer = "namespaceoffloading-controller.liqo.io/finalizer"
)

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=namespacemaps,verbs=get;list;watch;patch;update;create;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

// NamespaceOffloadingReconciler ownership:
// --> NamespaceOffloading.Spec.
// --> NamespaceOffloading.Annotation.
// --> NamespaceOffloading.Status.RemoteNamespaceName.
// --> NamespaceOffloadingController finalizer.
// --> NamespaceMap.Spec.DesiredMapping, only for my namespace entries.

// Reconcile requires creation of remote namespaces according to ClusterSelector field.
func (r *NamespaceOffloadingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	namespaceOffloading := &offv1alpha1.NamespaceOffloading{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      liqoconst.DefaultNamespaceOffloadingName,
	}, namespaceOffloading); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no namespaceOffloading in namespace '%s'", req.Namespace)
			return ctrl.Result{}, nil
		}
		klog.Errorf("%s --> Unable to get namespaceOffloading for the namespace '%s'", err, req.Namespace)
		return ctrl.Result{}, err
	}

	// Get all NamespaceMaps in the cluster and create a Map 'cluster-id : *NamespaceMap'
	clusterIDMap, err := r.getClusterIDMap(ctx)
	namespaceMapNumber := len(clusterIDMap)
	if err != nil {
		return ctrl.Result{}, err
	}

	// If deletion timestamp is set, it starts deletion logic and waits until all remote Namespaces
	// associated with this resource are deleted.
	if !namespaceOffloading.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, r.deletionLogic(ctx, namespaceOffloading, clusterIDMap)
	}
	// Initialize NamespaceOffloading Resource if it has been just created.
	if !ctrlutils.ContainsFinalizer(namespaceOffloading, namespaceOffloadingControllerFinalizer) {
		if err := r.initialConfiguration(ctx, namespaceOffloading); err != nil {
			return ctrl.Result{}, err
		}
	}
	// Request creation of remote Namespaces according to the ClusterSelector field.
	if err := r.enforceClusterSelector(ctx, namespaceOffloading, clusterIDMap); err != nil {
		return ctrl.Result{}, err
	}

	// If there is at least one remote Namespace add liqo scheduling label to the Namespace.
	if len(clusterIDMap) != namespaceMapNumber {
		// this label will trigger the liqo webhook.
		if err := addLiqoSchedulingLabel(ctx, r.Client, namespaceOffloading.Namespace); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// Todo: how to awake this controller for every NamespaceOffloading when a new NamespaceMap is created (or recreated).
// The name of all NamespaceOffloading resources must be always equal to "offloading", resources with a different
// name are not considered.
func namespaceOffloadingPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == liqoconst.DefaultNamespaceOffloadingName
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == liqoconst.DefaultNamespaceOffloadingName
		},
	}
}

// SetupWithManager reconciles NamespaceOffloading Resources.
func (r *NamespaceOffloadingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&offv1alpha1.NamespaceOffloading{}).
		WithEventFilter(namespaceOffloadingPredicate()).
		Complete(r)
}

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

package virtualnodectrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

const (
	virtualNodeControllerFinalizer = "virtualnode-controller.liqo.io/finalizer"
)

// VirtualNodeReconciler manage NamespaceMap lifecycle.
type VirtualNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// cluster-role
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups="",resources=nodes/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=namespacemaps,verbs=get;list;watch;delete;create
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch

// Reconcile manage NamespaceMaps associated with the virtual-node.
func (r *VirtualNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	virtualNode := &corev1.Node{}
	if err := r.Get(ctx, req.NamespacedName, virtualNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no a virtual-node called '%s'", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf(" %s --> Unable to get the virtual-node '%s'", err, req.Name)
		return ctrl.Result{}, err
	}

	// The virtual-node must have the cluster-id annotation.
	remoteClusterID, ok := virtualNode.Labels[liqoconst.RemoteClusterID]
	if !ok {
		err := fmt.Errorf("the label '%s' is not found on node '%s'", liqoconst.RemoteClusterID, virtualNode.GetName())
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// Retrieve the foreign cluster associated with the given cluster ID
	fc, err := foreignclusterutils.GetForeignClusterByID(ctx, r.Client, remoteClusterID)
	if err != nil {
		klog.Errorf("Failed to retrieve foreign cluster associated with cluster ID %q: %v", remoteClusterID, err)
		return ctrl.Result{}, err
	}

	if fc.Status.TenantNamespace.Local == "" {
		err = fmt.Errorf("local tenant namespace not set")
		klog.Errorf("Failed processing foreign cluster %q: %v", klog.KObj(fc), err)
		return ctrl.Result{}, err
	}

	// If the deletion timestamp is set all the NamespaceMaps associated with the virtual-node must be deleted.
	if !virtualNode.GetDeletionTimestamp().IsZero() {
		klog.Infof("The virtual node '%s' is requested to be deleted", virtualNode.GetName())
		return ctrl.Result{}, r.ensureNamespaceMapAbsence(ctx, fc, virtualNode)
	}

	// It is necessary to have a finalizer on the virtual-node.
	if err := r.ensureVirtualNodeFinalizerPresence(ctx, virtualNode); err != nil {
		return ctrl.Result{}, err
	}

	// If there is no NamespaceMap associated with this virtual-node, it creates a new one.
	if err := r.ensureNamespaceMapPresence(ctx, fc, virtualNode); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// All the events on virtual-nodes are monitored.
// Only the deletion event on NamespaceMaps is monitored.
func filterVirtualNodes() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			value, ok := (e.ObjectNew.GetLabels())[liqoconst.TypeLabel]
			return ok && value == liqoconst.TypeNode && !e.ObjectNew.GetDeletionTimestamp().IsZero()
		},
		CreateFunc: func(e event.CreateEvent) bool {
			value, ok := (e.Object.GetLabels())[liqoconst.TypeLabel]
			return ok && value == liqoconst.TypeNode
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// It is necessary to monitor also the deletion of the NamespaceMap.
			value, ok := (e.Object.GetLabels())[liqoconst.TypeLabel]
			// This controller watches the deletion of two kind of resources: virtual-nodes and
			// NamespaceMaps associated with corresponding virtual-nodes.
			// If the object has the label 'liqoconst.TypeLabel' with value 'liqoconst.TypeNode' it is a virtual-node,
			// while if the object has a non-empty namespace it is a NamespaceMap.
			return (ok && value == liqoconst.TypeNode) || e.Object.GetNamespace() != ""
		},
	}
}

// SetupWithManager monitors Virtual-nodes and their associated NamespaceMaps.
func (r *VirtualNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Owns(&mapsv1alpha1.NamespaceMap{}).
		WithEventFilter(filterVirtualNodes()).
		Complete(r)
}

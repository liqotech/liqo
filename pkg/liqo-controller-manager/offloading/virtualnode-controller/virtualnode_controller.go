// Copyright 2019-2025 The Liqo Authors
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

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/vkMachinery"
)

const (
	// virtualNodeControllerFinalizer is the finalizer added to virtual-node to allow the controller to clean up.
	virtualNodeControllerFinalizer = "virtualnode-controller.liqo.io/finalizer"
)

// VirtualNodeReconciler manage NamespaceMap lifecycle.
type VirtualNodeReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder

	HomeClusterID    liqov1beta1.ClusterID
	namespaceManager tenantnamespace.Manager
	dr               *DeletionRoutine
}

// NewVirtualNodeReconciler returns a new VirtualNodeReconciler.
func NewVirtualNodeReconciler(
	ctx context.Context,
	cl client.Client,
	s *runtime.Scheme, er record.EventRecorder,
	hci liqov1beta1.ClusterID,
	namespaceManager tenantnamespace.Manager,
) (*VirtualNodeReconciler, error) {
	vnr := &VirtualNodeReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,

		HomeClusterID:    hci,
		namespaceManager: namespaceManager,
	}
	var err error
	vnr.dr, err = RunDeletionRoutine(ctx, vnr)
	if err != nil {
		klog.Errorf("Unable to run the deletion routine: %s", err)
		return nil, err
	}
	return vnr, nil
}

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes/status,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes/finalizers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespacemaps,verbs=get;list;watch;delete;create
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage NamespaceMaps associated with the virtual-node.
func (r *VirtualNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	virtualNode := &offloadingv1beta1.VirtualNode{}
	if err := r.Get(ctx, req.NamespacedName, virtualNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no virtual-node called %q in %q", req.Name, req.Namespace)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the virtual-node %q: %w", req.NamespacedName, err)
	}

	if virtualNode.DeletionTimestamp.IsZero() {
		if !ctrlutil.ContainsFinalizer(virtualNode, virtualNodeControllerFinalizer) {
			if err := r.ensureVirtualNodeFinalizerPresence(ctx, virtualNode); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if ctrlutil.ContainsFinalizer(virtualNode, virtualNodeControllerFinalizer) {
			// If the virtual-node is being deleted, it deletes the node and the virtual-node resource.
			if err := r.dr.EnsureNodeAbsence(virtualNode); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to delete the virtual-node: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if err := r.ensureVirtualKubeletDeploymentPresence(ctx, virtualNode); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to create the virtual-kubelet deployment: %w", err)
	}
	if !*virtualNode.Spec.CreateNode {
		// If the virtual-node is not enabled, it deletes the node but not the virtual-node resource.
		if err := r.dr.EnsureNodeAbsence(virtualNode); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to delete the node: %w", err)
		}
	}

	// If there is no NamespaceMap associated with this virtual-node, it creates a new one.
	if err := r.ensureNamespaceMapPresence(ctx, virtualNode); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func enqueFromDeployment(dep *appsv1.Deployment, rli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	rli.Add(
		reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      dep.Labels[consts.VirtualNodeLabel],
				Namespace: dep.Namespace,
			},
		},
	)
}

var deploymentHandler = &handler.Funcs{
	DeleteFunc: func(_ context.Context, de event.TypedDeleteEvent[client.Object], trli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		dep := de.Object.(*appsv1.Deployment)
		enqueFromDeployment(dep, trli)
	},
	UpdateFunc: func(_ context.Context, ue event.TypedUpdateEvent[client.Object], trli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		dep := ue.ObjectNew.(*appsv1.Deployment)
		enqueFromDeployment(dep, trli)
	},
}

func (r *VirtualNodeReconciler) enqueFromNamespaceMap() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, o client.Object) []reconcile.Request {
			nm, ok := o.(*offloadingv1beta1.NamespaceMap)
			if !ok {
				return []reconcile.Request{}
			}

			if nm.Labels == nil || nm.Labels[consts.RemoteClusterID] == "" {
				return []reconcile.Request{}
			}
			clusterID := nm.Labels[consts.RemoteClusterID]

			// list virtualnode resources with the remote cluster ID label
			virtualnodes, err := getters.ListVirtualNodesByClusterID(ctx, r.Client, liqov1beta1.ClusterID(clusterID))
			if err != nil {
				klog.Errorf("unable to list virtualnodes with clusterID %s: %v", clusterID, err)
				return []reconcile.Request{}
			}

			requests := []reconcile.Request{}
			for i := range virtualnodes {
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&virtualnodes[i]),
				})
			}
			return requests
		})
}

// SetupWithManager register the VirtualNodeReconciler to the manager.
func (r *VirtualNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// select virtual kubelet deployments only
	deployPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: vkMachinery.KubeletBaseLabels,
	})
	if err != nil {
		klog.Error(err)
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlVirtualNode).
		For(&offloadingv1beta1.VirtualNode{}).
		Watches(&appsv1.Deployment{}, deploymentHandler, builder.WithPredicates(deployPredicate)).
		Watches(&offloadingv1beta1.NamespaceMap{}, r.enqueFromNamespaceMap()).
		Complete(r)
}

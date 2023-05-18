// Copyright 2019-2023 The Liqo Authors
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

	"github.com/pterm/pterm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/source"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

const (
	virtualNodeControllerFinalizer = "virtualnode-controller.liqo.io/finalizer"
	nodeNameField                  = "spec.nodeName"
)

// VirtualNodeReconciler manage NamespaceMap lifecycle.
type VirtualNodeReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	HomeClusterIdentity   *discoveryv1alpha1.ClusterIdentity
	VirtualKubeletOptions *vkforge.VirtualKubeletOpts
	EventsRecorder        record.EventRecorder
	dr                    *DeletionRoutine
}

// cluster-role
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=virtualnodes,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=virtualnodes/finalizers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=namespacemaps,verbs=get;list;watch;delete;create
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage NamespaceMaps associated with the virtual-node.
func (r *VirtualNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !deletionRoutineRunning {
		var err error
		r.dr, err = RunDeletionRoutine(r)
		if err != nil {
			klog.Errorf("Unable to run the deletion routine: %s", err)
			return ctrl.Result{}, err
		}
		deletionRoutineRunning = true
	}
	pterm.FgLightYellow.Println("Reconciling", req.NamespacedName)
	virtualNode := &virtualkubeletv1alpha1.VirtualNode{}
	if err := r.Get(ctx, req.NamespacedName, virtualNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no a virtual-node called '%s' in '%s'", req.Name, req.Namespace)
			return ctrl.Result{}, nil
		}
		klog.Errorf(" %s --> Unable to get the virtual-node '%s'", err, req.Name)
		return ctrl.Result{}, err
	}

	if virtualNode.DeletionTimestamp.IsZero() {
		if !ctrlutil.ContainsFinalizer(virtualNode, virtualNodeControllerFinalizer) {
			if err := r.ensureVirtualNodeFinalizerPresence(ctx, virtualNode); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if ctrlutil.ContainsFinalizer(virtualNode, virtualNodeControllerFinalizer) {
			pterm.FgYellow.Printfln("Deleting the virtual-node '%s' in '%s'", req.Name, req.Namespace)
			if err := r.ensureVirtualKubeletDeploymentAbsence(ctx, virtualNode); err != nil {
				pterm.FgMagenta.Println("Unable to delete the virtual-kubelet deployment")
				return ctrl.Result{}, err
			}
			if err := r.ensureNamespaceMapAbsence(ctx, virtualNode); err != nil {
				pterm.FgMagenta.Println("Unable to delete the namespace-map")
				return ctrl.Result{}, err
			}
			r.dr.DeleteVirtualNode(virtualNode)
			return ctrl.Result{}, nil
		}
	}

	// It creates the virtual-kubelet deployment.
	if err := r.ensureVirtualKubeletDeploymentPresence(ctx, virtualNode); err != nil {
		klog.Errorf(" %s --> Unable to create the virtual-kubelet deployment", err)
		return ctrl.Result{}, err
	}

	// If there is no NamespaceMap associated with this virtual-node, it creates a new one.
	if err := r.ensureNamespaceMapPresence(ctx, virtualNode); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the VirtualNodeReconciler to the manager.
func (r *VirtualNodeReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Add field containing node Name to the Field Indexer
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Pod{}, nodeNameField, extractNodeNameFromPod); err != nil {
		return err
	}
	// select virtual kubelet deployments only
	deployPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: vkMachinery.KubeletBaseLabels,
	})
	if err != nil {
		klog.Error(err)
		return err
	}
	reconcileFromDeployment := func(dep *appsv1.Deployment, rli workqueue.RateLimitingInterface) {
		rli.Add(
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      dep.Labels[discovery.VirtualNodeLabel],
					Namespace: dep.Namespace,
				},
			},
		)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&virtualkubeletv1alpha1.VirtualNode{}).Watches(
		&source.Kind{Type: &appsv1.Deployment{}},
		&handler.Funcs{
			DeleteFunc: func(de event.DeleteEvent, rli workqueue.RateLimitingInterface) {
				dep := de.Object.(*appsv1.Deployment)
				reconcileFromDeployment(dep, rli)
			},
			UpdateFunc: func(ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
				dep := ue.ObjectNew.(*appsv1.Deployment)
				reconcileFromDeployment(dep, rli)
			},
		},
		builder.WithPredicates(deployPredicate),
	).Complete(r)
}

func extractNodeNameFromPod(rawObj client.Object) []string {
	pod, ok := rawObj.(*corev1.Pod)
	if !ok {
		return []string{}
	}
	return []string{pod.Spec.NodeName}
}

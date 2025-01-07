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

package nsoffctrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/syncset"
)

// NamespaceOffloadingReconciler reconciles NamespaceOffloading resources, and appropriately updates the corresponding NamespaceMaps.
type NamespaceOffloadingReconciler struct {
	client.Client
	Recorder     record.EventRecorder
	LocalCluster liqov1beta1.ClusterID

	// namespaces tracks the set of namespaces for which a NamespaceOffloading resource exists.
	namespaces *syncset.SyncSet
}

const (
	namespaceOffloadingControllerFinalizer = "namespaceoffloading-controller.liqo.io/finalizer"
)

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespacemaps,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnode, verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile implements the NamespaceOffloading reconciliation logic.
func (r *NamespaceOffloadingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	nsoff := &offloadingv1beta1.NamespaceOffloading{}
	if err := r.Get(ctx, req.NamespacedName, nsoff); err != nil {
		if apierrors.IsNotFound(err) {
			r.namespaces.Remove(req.Namespace)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Failed to retrieve NamespaceOffloading %q: %v", klog.KRef(req.Namespace, req.Name), err)
		return ctrl.Result{}, err
	}

	// Get all NamespaceMaps in the cluster and create a Map 'cluster-id : *NamespaceMap'
	clusterIDMap, err := r.getClusterIDMap(ctx)
	if err != nil {
		klog.Errorf("Failed to reconcile NamespaceOffloading %q: %v", klog.KObj(nsoff), err)
		return ctrl.Result{}, err
	}

	r.namespaces.Add(req.Namespace)

	// Defer the function to output the error message if necessary, as well as update the NamespaceOffloading status.
	defer func() {
		if err != nil {
			klog.Errorf("Failed to reconcile NamespaceOffloading %q: %v", klog.KObj(nsoff), err)
		}

		// Update the status, regardless of whether an error occurred.
		if err = r.enforceStatus(ctx, nsoff, clusterIDMap); err != nil {
			klog.Errorf("Failed to update NamespaceOffloading %q status: %v", klog.KObj(nsoff), err)
			return
		}

		klog.Infof("NamespaceOffloading %q status correctly updated", klog.KObj(nsoff))
	}()

	// If deletion timestamp is set, it starts deletion logic and waits until all remote Namespaces
	// associated with this resource are deleted.
	if !nsoff.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, r.deletionLogic(ctx, nsoff, clusterIDMap)
	}

	// Ensure the presence of the finalizer, to guarantee resource cleanup upon deletion.
	if err := r.enforceFinalizerPresence(ctx, nsoff); err != nil {
		return ctrl.Result{}, err
	}

	// Request creation of remote Namespaces according to the ClusterSelector field.
	if err := r.enforceClusterSelector(ctx, nsoff, clusterIDMap); err != nil {
		return ctrl.Result{}, err
	}

	switch nsoff.Spec.PodOffloadingStrategy {
	case offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType, offloadingv1beta1.RemotePodOffloadingStrategyType:
		// If the offloading policy includes remote clusters, then ensure the corresponding namespace has the liqo scheduling label.
		return ctrl.Result{}, r.enforceSchedulingLabelPresence(ctx, nsoff.Namespace)
	default:
		// Otherwise, ensure the label is not present.
		return ctrl.Result{}, r.enforceSchedulingLabelAbsence(ctx, nsoff.Namespace)
	}
}

// SetupWithManager reconciles NamespaceOffloading Resources.
func (r *NamespaceOffloadingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.namespaces = syncset.New()

	filter := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == consts.DefaultNamespaceOffloadingName
	})

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlNamespaceOffloading).
		For(&offloadingv1beta1.NamespaceOffloading{}, builder.WithPredicates(filter)).
		Watches(&offloadingv1beta1.NamespaceMap{}, r.namespaceMapHandlers()).
		Watches(&offloadingv1beta1.VirtualNode{}, r.enqueueAll()).
		Watches(&corev1.Node{}, r.enqueueAll()).
		Complete(r)
}

func (r *NamespaceOffloadingReconciler) namespaceMapHandlers() handler.EventHandler {
	enqueue := func(rli workqueue.TypedRateLimitingInterface[reconcile.Request], namespace string) {
		rli.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      consts.DefaultNamespaceOffloadingName,
			Namespace: namespace,
		}})
	}

	return handler.Funcs{
		CreateFunc: func(_ context.Context, _ event.TypedCreateEvent[client.Object], trli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			// Enqueue an event for all known NamespaceOffloadings.
			r.namespaces.ForEach(func(namespace string) { enqueue(trli, namespace) })
		},
		UpdateFunc: func(_ context.Context, ue event.TypedUpdateEvent[client.Object], trli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			oldMappings := ue.ObjectOld.(*offloadingv1beta1.NamespaceMap).Status.CurrentMapping
			newMappings := ue.ObjectNew.(*offloadingv1beta1.NamespaceMap).Status.CurrentMapping

			// Enqueue an event for all elements that are different between the old and the new object.
			for namespace, oldStatus := range oldMappings {
				if newStatus, found := newMappings[namespace]; !found || oldStatus.Phase != newStatus.Phase {
					enqueue(trli, namespace)
				}
			}

			// Enqueue an event for all elements that have just been added.
			for namespace := range newMappings {
				if _, found := oldMappings[namespace]; !found {
					enqueue(trli, namespace)
				}
			}
		},
		DeleteFunc: func(_ context.Context, _ event.TypedDeleteEvent[client.Object], trli workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			// Enqueue an event for all known NamespaceOffloadings.
			r.namespaces.ForEach(func(namespace string) { enqueue(trli, namespace) })
		},
	}
}

func (r *NamespaceOffloadingReconciler) enqueueAll() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		var nsolist offloadingv1beta1.NamespaceOffloadingList
		if err := r.Client.List(ctx, &nsolist); err != nil {
			klog.Errorf("Failed to retrieve NamespaceOffloadingList: %v", err)
			return nil
		}
		reqs := make([]reconcile.Request, len(nsolist.Items))
		for i := range nsolist.Items {
			reqs[i] = reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      nsolist.Items[i].Name,
				Namespace: nsolist.Items[i].Namespace,
			}}
		}
		return reqs
	},
	)
}

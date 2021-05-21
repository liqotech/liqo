/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespaceoffloadingctrl

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/slice"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
)

// NamespaceOffloadingReconciler adds/removes DesiredMapping to/from NamespaceMaps in according with
// ClusterSelector field.
type NamespaceOffloadingReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	LocalClusterID string
}

const (
	namespaceOffloadingControllerFinalizer = "namespaceoffloading-controller.liqo.io/finalizer"
)

func (r *NamespaceOffloadingReconciler) getClusterIDMap(ctx context.Context, nms *mapsv1alpha1.NamespaceMapList,
	clusterIDMap map[string]*mapsv1alpha1.NamespaceMap) error {
	if err := r.List(ctx, nms); err != nil {
		klog.Error(err, " --> Unable to List NamespaceMaps")
		return err
	}

	if len(nms.Items) == 0 {
		klog.Info("No NamespaceMaps at the moment in the cluster")
		return nil
	}

	for i := range nms.Items {
		clusterIDMap[nms.Items[i].Labels[liqoconst.RemoteClusterID]] = &nms.Items[i]
	}
	return nil
}

func (r *NamespaceOffloadingReconciler) deletionLogic(ctx context.Context,
	noff *offv1alpha1.NamespaceOffloading, clusterIDMap map[string]*mapsv1alpha1.NamespaceMap) error {
	klog.Infof("The NamespaceOffloading of the namespace '%s' is requested to be deleted", noff.Namespace)
	// 1 - remove Liqo scheduling label from the associated namespace
	if err := removeLiqoSchedulingLabel(ctx, r.Client, noff.Namespace); err != nil {
		return err
	}
	// 2 - remove the involved DesiredMapping from alla NamespaceMap
	if err := removeDesiredMappings(r.Client, noff.Namespace, clusterIDMap); err != nil {
		return err
	}
	// 3 - check if all remote namespace associated with this NamespaceOffloading resource are really deleted
	if len(noff.Status.RemoteNamespacesConditions) != 0 {
		err := fmt.Errorf("some remote namespaces still exist")
		klog.Info(err)
		return err
	}
	// 4 - remove NamespaceOffloading controller finalizer; all remote namespaces associated with this resource
	// have been deleted.
	patch := noff.DeepCopy()
	ctrlutils.RemoveFinalizer(noff, namespaceOffloadingControllerFinalizer)
	if err := r.Patch(ctx, noff, client.MergeFrom(patch)); err != nil {
		klog.Errorf("%s --> Unable to remove finalizer from NamespaceOffloading", err)
		return err
	}
	klog.Info("Finalizer correctly removed from NamespaceOffloading")
	return nil
}

func (r *NamespaceOffloadingReconciler) initialConfiguration(ctx context.Context,
	noff *offv1alpha1.NamespaceOffloading) error {
	patch := noff.DeepCopy()
	// 1 - Add NamespaceOffloadingController Finalizer.
	ctrlutils.AddFinalizer(noff, namespaceOffloadingControllerFinalizer)
	// 2 - Add empty cluster selector if not specified by the user.
	if noff.Spec.ClusterSelector.Size() == 0 {
		noff.Spec.ClusterSelector = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}}
	}
	// 3 - Add NamespaceOffloading.Status.RemoteNamespaceName.
	if noff.Spec.NamespaceMappingStrategy == offv1alpha1.EnforceSameNameMappingStrategyType {
		noff.Status.RemoteNamespaceName = noff.Namespace
	} else {
		if r.LocalClusterID == "" {
			clusterID, err := liqoutils.GetClusterID(r.Client)
			if err != nil {
				return err
			}
			r.LocalClusterID = clusterID
		}
		noff.Status.RemoteNamespaceName = fmt.Sprintf("%s-%s", noff.Namespace, r.LocalClusterID)
	}
	// 4 - Patch the NamespaceOffloading resource.
	if err := r.Patch(ctx, noff, client.MergeFrom(patch)); err != nil {
		klog.Errorf("%s --> Unable to update NamespaceOffloading in namespace '%s'",
			err, noff.Namespace)
		return err
	}
	return nil
}

// NamespaceOffloadingReconciler ownership:
// --> NamespaceOffloading.Spec.
// --> NamespaceOffloading.Annotation.
// --> NamespaceOffloading.Status.RemoteNamespaceName.
// --> NamespaceOffloadingController finalizer.
// --> NamespaceMap.Spec.DesiredMapping, only for my namespace entries.

// Reconcile requires creation of remote namespaces according to ClusterSelector field.
func (r *NamespaceOffloadingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	namespaceOffloading := &offv1alpha1.NamespaceOffloading{}
	if err := r.Get(context.TODO(), types.NamespacedName{
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
	namespaceMapsList := &mapsv1alpha1.NamespaceMapList{}
	clusterIDMap := make(map[string]*mapsv1alpha1.NamespaceMap)
	if err := r.getClusterIDMap(ctx, namespaceMapsList, clusterIDMap); err != nil {
		return ctrl.Result{}, err
	}
	// There are no NamespaceMap in the cluster
	if len(clusterIDMap) == 0 {
		return ctrl.Result{}, nil
	}

	// If deletion timestamp is set, it starts deletion logic and waits until all remote Namespaces
	// associated with this resource are deleted.
	if !namespaceOffloading.GetDeletionTimestamp().IsZero() {
		if err := r.deletionLogic(ctx, namespaceOffloading, clusterIDMap); err != nil {
			return ctrl.Result{}, err
		}
	}
	// Initialize NamespaceOffloading Resource if it has been just created.
	if !ctrlutils.ContainsFinalizer(namespaceOffloading, namespaceOffloadingControllerFinalizer) {
		if err := r.initialConfiguration(ctx, namespaceOffloading); err != nil {
			return ctrl.Result{}, err
		}
	}
	// Request creation of remote Namespaces in according with ClusterSelector field.
	if err := r.enforceClusterSelector(namespaceOffloading, clusterIDMap); err != nil {
		return ctrl.Result{}, err
	}

	// If there is at least one remote Namespace add liqo scheduling label to the Namespace.
	if len(clusterIDMap) != len(namespaceMapsList.Items) {
		// this label will trig liqo webhook
		if err := addLiqoSchedulingLabel(ctx, r.Client, namespaceOffloading.Namespace); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// Todo: how to awake this controller for every NamespaceOffloading when a new NamespaceMap is created (or recreated).
func namespaceOffloadingPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !(e.ObjectNew.GetDeletionTimestamp().IsZero()) && slice.ContainsString(e.ObjectNew.GetFinalizers(),
				namespaceOffloadingControllerFinalizer, nil) {
				return true
			}
			return false
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

package namespaceoffloadingctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
)

func (r *NamespaceOffloadingReconciler) deletionLogic(ctx context.Context,
	noff *offv1alpha1.NamespaceOffloading, clusterIDMap map[string]*mapsv1alpha1.NamespaceMap) error {
	klog.Infof("The NamespaceOffloading of the namespace '%s' is requested to be deleted", noff.Namespace)
	// 1 - remove Liqo scheduling label from the associated namespace.
	if err := removeLiqoSchedulingLabel(ctx, r.Client, noff.Namespace); err != nil {
		return err
	}
	// 2 - remove the involved DesiredMapping from the NamespaceMap.
	if err := removeDesiredMappings(r.Client, noff.Namespace, clusterIDMap); err != nil {
		return err
	}
	// 3 - check if all remote namespaces associated with this NamespaceOffloading resource are really deleted.
	if len(noff.Status.RemoteNamespacesConditions) != 0 {
		err := fmt.Errorf("some remote namespaces still exist")
		klog.Info(err)
		return err
	}
	// 4 - remove NamespaceOffloading controller finalizer; all remote namespaces associated with this resource
	// have been deleted.
	original := noff.DeepCopy()
	ctrlutils.RemoveFinalizer(noff, namespaceOffloadingControllerFinalizer)
	if err := r.Patch(ctx, noff, client.MergeFrom(original)); err != nil {
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

package namespaceoffloadingctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8shelper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

func (r *NamespaceOffloadingReconciler) enforceClusterSelector(ctx context.Context, noff *offv1alpha1.NamespaceOffloading,
	clusterIDMap map[string]*mapsv1alpha1.NamespaceMap) error {
	virtualNodes := &corev1.NodeList{}
	if err := r.List(ctx, virtualNodes,
		client.MatchingLabels{liqoconst.TypeLabel: liqoconst.TypeNode}); err != nil {
		klog.Error(err, " --> Unable to List all virtual nodes")
		return err
	}

	// If here there are no virtual nodes is an error because it means that in the cluster there are NamespaceMap
	// but not their associated virtual nodes
	if len(virtualNodes.Items) != len(clusterIDMap) {
		err := fmt.Errorf(" No VirtualNodes at the moment in the cluster")
		klog.Error(err)
		return err
	}

	errorCondition := false
	for i := range virtualNodes.Items {
		match, err := k8shelper.MatchNodeSelectorTerms(&virtualNodes.Items[i], &noff.Spec.ClusterSelector)
		if err != nil {
			klog.Infof("%s -> Unable to offload the namespace '%s', there is an error in ClusterSelectorField",
				err, noff.Namespace)
			patch := noff.DeepCopy()
			if noff.Annotations == nil {
				noff.Annotations = map[string]string{}
			}
			noff.Annotations[liqoconst.SchedulingLiqoLabel] = fmt.Sprintf("Invalid Cluster Selector : %s", err)
			if err = r.Patch(ctx, noff, client.MergeFrom(patch)); err != nil {
				klog.Errorf("%s -> unable to add the liqo scheduling annotation to the NamespaceOffloading in the namespace '%s'",
					err, noff.Namespace)
				return err
			}
			klog.Infof("The liqo scheduling annotation is correctly added to the NamespaceOffloading in the namespace '%s'",
				noff.Namespace)
			break
		}
		if match {
			if err = addDesiredMapping(ctx, r.Client, noff.Namespace, noff.Status.RemoteNamespaceName,
				clusterIDMap[virtualNodes.Items[i].Annotations[liqoconst.RemoteClusterID]]); err != nil {
				errorCondition = true
				continue
			}
			delete(clusterIDMap, virtualNodes.Items[i].Annotations[liqoconst.RemoteClusterID])
		}
	}
	if errorCondition {
		err := fmt.Errorf("some desiredMappings has not been added")
		klog.Error(err)
		return err
	}
	return nil
}

func (r *NamespaceOffloadingReconciler) getClusterIDMap(ctx context.Context) (map[string]*mapsv1alpha1.NamespaceMap, error) {
	nms := &mapsv1alpha1.NamespaceMapList{}
	if err := r.List(ctx, nms); err != nil {
		klog.Error(err, " --> Unable to List NamespaceMaps")
		return nil, err
	}

	clusterIDMap := make(map[string]*mapsv1alpha1.NamespaceMap)
	if len(nms.Items) == 0 {
		klog.Info("No NamespaceMaps at the moment in the cluster")
		return clusterIDMap, nil
	}

	for i := range nms.Items {
		clusterIDMap[nms.Items[i].Labels[liqoconst.RemoteClusterID]] = &nms.Items[i]
	}
	return clusterIDMap, nil
}

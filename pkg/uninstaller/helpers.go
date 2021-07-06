package uninstaller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	discoveryV1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// getForeignList retrieve the list of available ForeignCluster and return it as a ForeignClusterList object.
func getForeignList(client dynamic.Interface) (*discoveryV1alpha1.ForeignClusterList, error) {
	r1 := client.Resource(discoveryV1alpha1.ForeignClusterGroupVersionResource)
	t, err := r1.Namespace("").List(context.TODO(), metav1.ListOptions{TypeMeta: metav1.TypeMeta{}})
	if err != nil {
		return nil, err
	}
	klog.V(5).Info("Getting ForeignClusters list")
	var foreign *discoveryV1alpha1.ForeignClusterList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(t.UnstructuredContent(), &foreign); err != nil {
		return nil, err
	}
	return foreign, nil
}

// checkPeeringsStatus verifies if the cluster has any active peerings with foreign clusters.
func checkPeeringsStatus(foreign *discoveryV1alpha1.ForeignClusterList) bool {
	var returnValue = true
	for i := range foreign.Items {
		item := &foreign.Items[i]
		if foreigncluster.IsIncomingEnabled(item) || foreigncluster.IsOutgoingEnabled(item) {
			klog.Infof("Cluster %s still has a valid peering: (Incoming: %s, Outgoing: %s",
				item.Name, item.Status.Incoming.PeeringPhase, item.Status.Outgoing.PeeringPhase)
			returnValue = false
		}
	}
	return returnValue
}

// generateLabelString converts labelSelector to string.
func generateLabelString(labelSelector metav1.LabelSelector) (string, error) {
	labelMap, err := metav1.LabelSelectorAsMap(&labelSelector)
	if err != nil {
		return "", err
	}
	return labels.SelectorFromSet(labelMap).String(), nil
}

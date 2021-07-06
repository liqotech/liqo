package util

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
)

// GetNodes returns the list of nodes of the cluster matching the given labels.
func GetNodes(ctx context.Context, client kubernetes.Interface, clusterID, labelSelector string) (*v1.NodeList, error) {
	remoteNodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while listing nodes: %s", clusterID, err)
		return nil, err
	}
	return remoteNodes, nil
}

// CheckVirtualNodes checks if the Liqo virtual nodes of cluster C.
func CheckVirtualNodes(ctx context.Context, homeClusterClient kubernetes.Interface) (ready bool) {
	var nodeLabel = make(map[string]string)
	nodeLabel[consts.TypeLabel] = consts.TypeNode
	virtualNodes, err := homeClusterClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(nodeLabel).String(),
	})
	if err != nil {
		klog.Error(err)
		return false
	}
	for index := range virtualNodes.Items {
		for _, condition := range virtualNodes.Items[index].Status.Conditions {
			if condition.Type == v1.NodeReady {
				if condition.Status == v1.ConditionFalse {
					return false
				}
			} else {
				if condition.Status == v1.ConditionTrue {
					return false
				}
			}
		}
	}
	return true
}

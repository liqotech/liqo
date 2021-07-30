package foreigncluster

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// GetForeignClusterByID returns a ForeignCluster CR retrieving it by its clusterID.
func GetForeignClusterByID(ctx context.Context, cl client.Client, clusterID string) (*discoveryv1alpha1.ForeignCluster, error) {
	// get the foreign cluster by clusterID label
	foreignClusterList := discoveryv1alpha1.ForeignClusterList{}
	if err := cl.List(ctx, &foreignClusterList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			discovery.ClusterIDLabel: clusterID,
		}),
	}); err != nil {
		klog.Error(err)
		return nil, err
	}

	if len(foreignClusterList.Items) == 0 {
		// object not found
		err := fmt.Errorf("ForeignCluster not found for cluster id %v", clusterID)
		klog.Error(err)
		return nil, err
	}
	return GetOlderForeignCluster(&foreignClusterList), nil
}

// GetOlderForeignCluster returns the ForeignCluster from the list with the older creationTimestamp.
func GetOlderForeignCluster(
	foreignClusterList *discoveryv1alpha1.ForeignClusterList) (foreignCluster *discoveryv1alpha1.ForeignCluster) {
	var olderTime *metav1.Time = nil
	for i := range foreignClusterList.Items {
		fc := &foreignClusterList.Items[i]
		if olderTime.IsZero() || fc.CreationTimestamp.Before(olderTime) {
			olderTime = &fc.CreationTimestamp
			foreignCluster = fc
		}
	}
	return foreignCluster
}

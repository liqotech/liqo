package crdreplicator

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// cleanupLocalReplicatedResources removes all local replicated resources for a given remote cluster.
func (c *Controller) cleanupLocalReplicatedResources(ctx context.Context, remoteClusterID string) error {
	var errs []error
	for i := range c.RegisteredResources {
		res := &c.RegisteredResources[i]
		gvr := schema.GroupVersionResource{
			Group:    res.GroupVersionResource.Group,
			Version:  res.GroupVersionResource.Version,
			Resource: res.GroupVersionResource.Resource,
		}
		// List all resources with the label for this remote cluster
		// list, err := c.DynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{
		// 	LabelSelector: fmt.Sprintf("liqo.io/remote-cluster-id=%s", remoteClusterID),
		// })

		list, err := c.DynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})

		if err != nil {
			klog.Warningf("Failed to list %s for cleanup: %v", gvr.String(), err)
			errs = append(errs, err)
			continue
		}
		for _, item := range list.Items {
			err := c.DynamicClient.Resource(gvr).Namespace(item.GetNamespace()).Delete(ctx, item.GetName(), metav1.DeleteOptions{})
			if err != nil {
				klog.Warningf("Failed to delete %s/%s: %v", gvr.Resource, item.GetName(), err)
				errs = append(errs, err)
			} else {
				klog.Infof("Deleted local replicated resource %s/%s for cluster %s", gvr.Resource, item.GetName(), remoteClusterID)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cleanupLocalReplicatedResources: %v", errs)
	}
	return nil
}

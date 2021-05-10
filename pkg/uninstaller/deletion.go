package uninstaller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	clusterconfigV1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryV1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// UnjoinClusters disables outgoing peerings with available clusters.
func UnjoinClusters(client dynamic.Interface) error {
	foreign, err := getForeignList(client)
	if err != nil {
		return err
	}
	klog.Infof("Unjoin %v ForeignClusters", len(foreign.Items))
	r1 := client.Resource(discoveryV1alpha1.ForeignClusterGroupVersionResource)
	for _, item := range foreign.Items {
		patch := []byte(`{"spec": {"join": false}}`)
		_, err = r1.Namespace(item.Namespace).Patch(context.TODO(), item.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// DisableBroadcasting disables broadcasting of advertisements from the ClusterConfig.
func DisableBroadcasting(client dynamic.Interface) error {
	r1 := client.Resource(clusterconfigV1alpha1.ClusterConfigGroupVersionResource)
	t, err := r1.Namespace("").List(context.TODO(), metav1.ListOptions{TypeMeta: metav1.TypeMeta{}})
	if err != nil {
		return err
	}
	klog.V(5).Infof("Getting clusterConfigs")
	var clusterconfigs clusterconfigV1alpha1.ClusterConfigList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(t.UnstructuredContent(), &clusterconfigs); err != nil {
		return err
	}
	klog.V(5).Infof("Patching ClusterConfigs")
	for _, item := range clusterconfigs.Items {
		patch := []byte(`{"spec": {"advertisementConfig": { "outgoingConfig" : { "enableBroadcaster" : false}}}}`)
		_, err = r1.Namespace(item.Namespace).Patch(context.TODO(), item.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

package uninstaller

import (
	"context"
	"encoding/json"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	clusterconfigV1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryV1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// UnjoinClusters disables outgoing peerings with available clusters.
func UnjoinClusters(ctx context.Context, client dynamic.Interface) error {
	foreign, err := getForeignList(client)
	if err != nil {
		return err
	}
	klog.Infof("Unjoin %v ForeignClusters", len(foreign.Items))
	r1 := client.Resource(discoveryV1alpha1.ForeignClusterGroupVersionResource)
	for index := range foreign.Items {
		patch := []byte(`{"spec": {"outgoingPeeringEnabled": "No"}}`)
		_, err = r1.Patch(ctx, foreign.Items[index].Name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// DisableDiscoveryAndPeering disables discovery and peering mechanism to prevent new peerings to happen.
func DisableDiscoveryAndPeering(ctx context.Context, client dynamic.Interface) error {
	r1 := client.Resource(clusterconfigV1alpha1.ClusterConfigGroupVersionResource)
	t, err := r1.List(ctx, metav1.ListOptions{TypeMeta: metav1.TypeMeta{}})
	if err != nil {
		return err
	}
	klog.V(5).Infof("Getting clusterConfigs")
	var clusterconfigs clusterconfigV1alpha1.ClusterConfigList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(t.UnstructuredContent(), &clusterconfigs); err != nil {
		return err
	}
	if len(clusterconfigs.Items) != 1 {
		return fmt.Errorf("ERROR: Wrong number of ClusterConfig: %v", len(clusterconfigs.Items))
	}
	err = patchClusterConfig(ctx, forgeUninstallClusterConfig, &clusterconfigs.Items[0], client)
	return err
}

// DeleteAllForeignClusters deletes all ForeignCluster resources.
func DeleteAllForeignClusters(ctx context.Context, client dynamic.Interface) error {
	r1 := client.Resource(discoveryV1alpha1.ForeignClusterGroupVersionResource)
	err := r1.DeleteCollection(ctx,
		metav1.DeleteOptions{TypeMeta: metav1.TypeMeta{}}, metav1.ListOptions{TypeMeta: metav1.TypeMeta{}})
	return err
}

func forgeUninstallClusterConfig(clusterConfig *clusterconfigV1alpha1.ClusterConfig) {
	clusterConfig.Spec.DiscoveryConfig.EnableDiscovery = false
	clusterConfig.Spec.DiscoveryConfig.EnableAdvertisement = false
	clusterConfig.Spec.DiscoveryConfig.AutoJoin = false
	clusterConfig.Spec.AdvertisementConfig.OutgoingConfig.EnableBroadcaster = false
}

// patchClusterConfig patches the controlled ClusterConfig applying the provided function.
func patchClusterConfig(ctx context.Context, changeFunc func(node *clusterconfigV1alpha1.ClusterConfig),
	initialClusterConfig *clusterconfigV1alpha1.ClusterConfig, client dynamic.Interface) error {
	original, err := json.Marshal(initialClusterConfig)
	if err != nil {
		klog.Error(err)
		return err
	}

	newClusterConfig := initialClusterConfig.DeepCopy()
	changeFunc(newClusterConfig)

	target, err := json.Marshal(newClusterConfig)
	if err != nil {
		klog.Error(err)
		return err
	}

	ops, err := jsonpatch.CreatePatch(original, target)
	if err != nil {
		klog.Error(err)
		return err
	}

	if len(ops) == 0 {
		// this avoids an empty patch of the node
		return nil
	}

	bytes, err := json.Marshal(ops)
	if err != nil {
		klog.Error(err)
		return err
	}

	r1 := client.Resource(clusterconfigV1alpha1.ClusterConfigGroupVersionResource)
	_, err = r1.Patch(ctx, newClusterConfig.Name, types.JSONPatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

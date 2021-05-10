package utils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterIDConfMap   = "cluster-id"
	configMapNamespace = "liqo"
)

func GetClusterID(c client.Client) (string, error) {

	configMap := &corev1.ConfigMap{}
	if err := c.Get(context.TODO(), types.NamespacedName{Namespace: configMapNamespace, Name: clusterIDConfMap}, configMap); err != nil {
		klog.Errorf("%s, unable to get ConfigMap '%s' in namespace '%s'", err, clusterIDConfMap, configMapNamespace)
		return "", err
	}

	clusterID := configMap.Data[clusterIDConfMap]
	klog.Infof("ClusterID is '%s'", clusterID)
	return clusterID, nil
}

package utils

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterIDConfMap = "cluster-id"
	// Todo: Is there a place where i can get this?
	configMapNamespace = "liqo"
)

func GetClusterID(c client.Client) (string, error) {

	configMaps := &corev1.ConfigMapList{}
	// possibility to specify "Liqo" namespace if it is possible to get it somewhere
	if err := c.List(context.TODO(), configMaps, client.InNamespace(configMapNamespace)); err != nil {
		klog.Error(err, "unable to get ConfigMapList")
		return "", err
	}
	if len(configMaps.Items) != 1 {
		err := fmt.Errorf("there are %d ConfigMaps at the moment, unstable condition", len(configMaps.Items))
		klog.Error(err)
		return "", err
	}

	clusterID := configMaps.Items[0].Data[clusterIDConfMap]
	klog.Infof("ClusterID is '%s'", clusterID)
	return clusterID, nil
}

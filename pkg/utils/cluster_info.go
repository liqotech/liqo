package utils

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
)

// GetClusterIDWithNativeClient returns clusterID using a kubernetes.Interface client.
func GetClusterIDWithNativeClient(ctx context.Context, nativeClient kubernetes.Interface, namespace string) (string, error) {
	cmClient := nativeClient.CoreV1().ConfigMaps(namespace)
	cm, err := cmClient.Get(ctx, consts.ClusterIDConfigMapName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	clusterID := cm.Data[consts.ClusterIDConfigMapKey]
	klog.Infof("ClusterID is '%s'", clusterID)
	return clusterID, nil
}

// GetClusterIDWithControllerClient returns clusterID using a client.Client client.
func GetClusterIDWithControllerClient(ctx context.Context, controllerClient client.Client, namespace string) (string, error) {
	configMap := &corev1.ConfigMap{}
	if err := controllerClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: consts.ClusterIDConfigMapName}, configMap); err != nil {
		klog.Errorf("%s, unable to get ConfigMap '%s' in namespace '%s'", err, consts.ClusterIDConfigMapName, namespace)
		return "", err
	}

	clusterID := configMap.Data[consts.ClusterIDConfigMapKey]
	klog.Infof("ClusterID is '%s'", clusterID)
	return clusterID, nil
}

// GetClusterIDFromNodeName returns the clusterID from a node name.
func GetClusterIDFromNodeName(nodeName string) string {
	return strings.TrimPrefix(nodeName, virtualKubelet.VirtualNodePrefix)
}

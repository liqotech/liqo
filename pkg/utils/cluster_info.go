package utils

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
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

// RetrieveNamespace tries to retrieve the name of the namespace where the process is executed.
// It tries to get the namespace:
// - Firstly, using the POD_NAMESPACE variable
// - Secondly, by looking for the namespace value contained in a mounted ServiceAccount (if any)
// Otherwise, it returns an empty string and an error.
func RetrieveNamespace() (string, error) {
	namespace, found := os.LookupEnv("POD_NAMESPACE")
	if !found {
		klog.Info("POD_NAMESPACE not set")
		data, err := ioutil.ReadFile(consts.ServiceAccountNamespacePath)
		if err != nil {
			return "", fmt.Errorf("unable to get namespace")
		}
		if namespace = strings.TrimSpace(string(data)); namespace == "" {
			return "", fmt.Errorf("unable to get namespace")
		}
	}
	return namespace, nil
}

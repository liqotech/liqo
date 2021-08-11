package utils

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/discovery"
)

// GetAPIServerURL retrieves the API server URL either from the configuration or selecting the IP address of a master node (with port 6443).
func GetAPIServerURL(apiServerConfigProvider ApiServerConfigProvider, clientset kubernetes.Interface) (string, error) {
	config := apiServerConfigProvider.GetAPIServerConfig()

	address := config.Address
	if address != "" {
		if !strings.HasPrefix(address, "https://") {
			address = fmt.Sprintf("https://%v", address)
		}
		return address, nil
	}

	return GetAPIServerAddressFromMasterNode(context.TODO(), clientset)
}

// GetAPIServerAddressFromMasterNode returns the API Server address using the IP of the
// master node of this cluster. The port is always defaulted to 6443.
func GetAPIServerAddressFromMasterNode(ctx context.Context,
	clientset kubernetes.Interface) (address string, err error) {
	nodes, err := getMasterNodes(ctx, clientset)
	if err != nil {
		klog.Error(err)
		return "", err
	}
	host, err := discovery.GetAddressFromNodeList(nodes.Items)
	if err != nil {
		klog.Error(err)
		return "", err
	}
	return fmt.Sprintf("https://%v:6443", host), nil
}

func getMasterNodes(ctx context.Context, clientset kubernetes.Interface) (*v1.NodeList, error) {
	labelSelectors := []string{
		"node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/master",
	}

	nodes := &v1.NodeList{}
	var err error
	for _, selector := range labelSelectors {
		nodes, err = clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			klog.Error(err)
			return nodes, err
		}
		if len(nodes.Items) != 0 {
			break
		}
	}

	if len(nodes.Items) == 0 {
		err = fmt.Errorf("no ApiServer.Address variable provided and no master node found, one of the two values must be present")
		klog.Error(err)
		return nodes, err
	}
	return nodes, nil
}

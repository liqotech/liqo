package kubeconfig

import (
	"context"
	"errors"
	"fmt"

	"github.com/liqotech/liqo/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	"k8s.io/klog"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"

	"github.com/liqotech/liqo/pkg/discovery"
)

// this function creates a kube-config file for a specified ServiceAccount.
func CreateKubeConfigFromServiceAccount(apiServerConfigProvider utils.ApiServerConfigProvider, clientset kubernetes.Interface, serviceAccount *corev1.ServiceAccount) (string, error) {
	secret, err := clientset.CoreV1().Secrets(serviceAccount.Namespace).Get(context.TODO(), serviceAccount.Secrets[0].Name, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	token := string(secret.Data["token"])

	server, err := GetApiServerURL(apiServerConfigProvider, clientset)
	if err != nil {
		klog.Error(err)
		return "", err
	}

	var caCrt []byte
	if apiServerConfigProvider.GetAPIServerConfig().TrustedCA {
		caCrt = nil
	} else {
		caCrt = secret.Data["ca.crt"]
	}

	cnf := kubeconfigutil.CreateWithToken(server, "service-cluster", serviceAccount.Name, caCrt, token)
	r, err := runtime.Encode(clientcmdlatest.Codec, cnf)
	if err != nil {
		klog.Error(err)
		return "", err
	}
	return string(r), nil
}

// Get the ApiServerURL.
// Retrieving the address in the order:
// 1. from the ClusterConfig
// 2. from the IP address of a master node
// And the port in the order:
// 1. from the ClusterConfig
// 2. defaults to 6443.
func GetApiServerURL(apiServerConfigProvider utils.ApiServerConfigProvider, clientset kubernetes.Interface) (string, error) {
	address := apiServerConfigProvider.GetAPIServerConfig().Address
	if address == "" {
		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		if err != nil {
			klog.Error(err)
			return "", err
		}
		if len(nodes.Items) == 0 {
			err = errors.New("no APISERVER env variable found and no master node found, one of the two values must be present")
			klog.Error(err)
			return "", err
		}
		address, err = discovery.GetAddressFromNodeList(nodes.Items)
		if err != nil {
			klog.Error(err)
			return "", err
		}
	}

	port := apiServerConfigProvider.GetAPIServerConfig().Port
	if port == "" {
		port = "6443"
	}

	return fmt.Sprintf("https://%v:%v", address, port), nil
}

// this function creates a kube-config file for a specified ServiceAccount.
func CreateKubeConfig(apiServerConfigProvider utils.ApiServerConfigProvider, clientset kubernetes.Interface, serviceAccountName string, namespace string) (string, error) {
	serviceAccount, err := clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), serviceAccountName, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	return CreateKubeConfigFromServiceAccount(apiServerConfigProvider, clientset, serviceAccount)
}

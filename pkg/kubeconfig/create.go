package kubeconfig

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	"k8s.io/klog"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"

	"github.com/liqotech/liqo/pkg/utils"
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
	config := apiServerConfigProvider.GetAPIServerConfig()

	address := config.Address
	if address != "" {
		if !strings.HasPrefix(address, "https://") {
			address = fmt.Sprintf("https://%v", address)
		}
		return address, nil
	}

	return utils.GetAPIServerAddressFromMasterNode(context.TODO(), clientset)
}

// this function creates a kube-config file for a specified ServiceAccount.
func CreateKubeConfig(apiServerConfigProvider utils.ApiServerConfigProvider, clientset kubernetes.Interface, serviceAccountName string, namespace string) (string, error) {
	serviceAccount, err := clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), serviceAccountName, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	return CreateKubeConfigFromServiceAccount(apiServerConfigProvider, clientset, serviceAccount)
}

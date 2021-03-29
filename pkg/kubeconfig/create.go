package kubeconfig

import (
	"context"
	"errors"
	"github.com/liqotech/liqo/pkg/clusterConfig"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	"k8s.io/klog"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
)

// this function creates a kube-config file for a specified ServiceAccount
func CreateKubeConfigFromServiceAccount(apiServerConfigProvider clusterConfig.ApiServerConfigProvider, clientset kubernetes.Interface, serviceAccount *corev1.ServiceAccount) (string, error) {
	secret, err := clientset.CoreV1().Secrets(serviceAccount.Namespace).Get(context.TODO(), serviceAccount.Secrets[0].Name, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	address := apiServerConfigProvider.GetApiServerConfig().Address
	if address == "" {
		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		if err != nil {
			return "", err
		}
		if len(nodes.Items) == 0 || len(nodes.Items[0].Status.Addresses) == 0 {
			err = errors.New("no APISERVER env variable found and no master node found, one of the two values must be present")
			klog.Error(err, err.Error())
			return "", err
		}
		address = nodes.Items[0].Status.Addresses[0].Address
	}

	port := apiServerConfigProvider.GetApiServerConfig().Port
	if port == "" {
		port = "6443"
	}

	token := string(secret.Data["token"])
	server := "https://" + address + ":" + port

	var caCrt []byte
	if apiServerConfigProvider.GetApiServerConfig().TrustedCA {
		caCrt = nil
	} else {
		caCrt = secret.Data["ca.crt"]
	}

	cnf := kubeconfigutil.CreateWithToken(server, "service-cluster", serviceAccount.Name, caCrt, token)
	r, err := runtime.Encode(clientcmdlatest.Codec, cnf)
	if err != nil {
		return "", err
	}
	return string(r), nil
}

// this function creates a kube-config file for a specified ServiceAccount
func CreateKubeConfig(apiServerConfigProvider clusterConfig.ApiServerConfigProvider, clientset kubernetes.Interface, serviceAccountName string, namespace string) (string, error) {
	serviceAccount, err := clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), serviceAccountName, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	return CreateKubeConfigFromServiceAccount(apiServerConfigProvider, clientset, serviceAccount)
}

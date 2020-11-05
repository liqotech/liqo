package kubeconfig

import (
	"context"
	"github.com/liqotech/liqo/pkg/apiServerUtils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
)

// this function creates a kube-config file for a specified ServiceAccount
func CreateKubeConfig(clientset kubernetes.Interface, serviceAccountName string, namespace string) (string, error) {
	serviceAccount, err := clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), serviceAccountName, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), serviceAccount.Secrets[0].Name, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	address, err := apiServerUtils.GetAddress(clientset)
	if err != nil {
		return "", err
	}

	port := apiServerUtils.GetPort()

	token := string(secret.Data["token"])
	server := "https://" + address + ":" + port

	cnf := kubeconfigutil.CreateWithToken(server, "service-cluster", serviceAccountName, secret.Data["ca.crt"], token)
	r, err := runtime.Encode(clientcmdlatest.Codec, cnf)
	if err != nil {
		return "", err
	}
	return string(r), nil
}

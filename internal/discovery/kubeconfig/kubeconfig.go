package kubeconfig

import (
	"context"
	"errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	"k8s.io/klog"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	"os"
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

	address, ok := os.LookupEnv("APISERVER")
	if !ok || address == "" {
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

	port, ok := os.LookupEnv("APISERVER_PORT")
	if !ok {
		port = "6443"
	}

	token := string(secret.Data["token"])
	server := "https://" + address + ":" + port

	cnf := kubeconfigutil.CreateWithToken(server, "service-cluster", serviceAccountName, secret.Data["ca.crt"], token)
	r, err := runtime.Encode(clientcmdlatest.Codec, cnf)
	if err != nil {
		return "", err
	}
	return string(r), nil
}

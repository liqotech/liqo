package auth_service

import (
	"context"
	"errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	"k8s.io/klog"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	"os"
)

// this function creates a kube-config file for a specified ServiceAccount
func (authService *AuthServiceCtrl) createKubeConfig(serviceAccount *v1.ServiceAccount) (string, error) {
	secret, err := authService.clientset.CoreV1().Secrets(authService.namespace).Get(context.TODO(), serviceAccount.Secrets[0].Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	address, ok := os.LookupEnv("APISERVER")
	if !ok || address == "" {
		nodes := authService.nodeInformer.GetStore().List()
		var node *v1.Node = nil
		for _, tmp := range nodes {
			n, ok := tmp.(*v1.Node)
			if !ok || n.Labels == nil {
				continue
			}
			if _, present := n.Labels["node-role.kubernetes.io/master"]; present && len(n.Status.Addresses) > 0 {
				node = n.DeepCopy()
				break
			}
		}

		if node == nil {
			err = errors.New("no APISERVER env variable found and no master node found, one of the two values must be present")
			klog.Error(err)
			return "", err
		}
		address = node.Status.Addresses[0].Address
	}

	port, ok := os.LookupEnv("APISERVER_PORT")
	if !ok {
		port = "6443"
	}

	token := string(secret.Data["token"])
	server := "https://" + address + ":" + port

	cnf := kubeconfigutil.CreateWithToken(server, "service-cluster", serviceAccount.Name, secret.Data["ca.crt"], token)
	r, err := runtime.Encode(clientcmdlatest.Codec, cnf)
	if err != nil {
		return "", err
	}
	return string(r), nil
}

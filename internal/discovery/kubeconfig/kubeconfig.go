package kubeconfig

import (
	"errors"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
)

// this function creates a kube-config file for a specified ServiceAccount
func CreateKubeConfig(serviceAccountName string, namespace string) (string, error) {
	clientset, _ := clients.NewK8sClient()
	serviceAccount, err := clientset.CoreV1().ServiceAccounts(namespace).Get(serviceAccountName, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	secret, err := clientset.CoreV1().Secrets(namespace).Get(serviceAccount.Secrets[0].Name, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	nodes, err := clientset.CoreV1().Nodes().List(v1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master",
	})
	if err != nil {
		return "", err
	}
	if len(nodes.Items) == 0 {
		return "", errors.New("no master node found")
	}

	token := string(secret.Data["token"])
	//caData := b64.StdEncoding.EncodeToString(secret.Data["ca.crt"])
	server := "https://" + nodes.Items[0].Status.Addresses[0].Address + ":6443"

	cnf := kubeconfigutil.CreateWithToken(server, "service-cluster", serviceAccountName, secret.Data["ca.crt"], token)
	r, err := runtime.Encode(clientcmdlatest.Codec, cnf)
	if err != nil {
		return "", err
	}
	return string(r), nil
}

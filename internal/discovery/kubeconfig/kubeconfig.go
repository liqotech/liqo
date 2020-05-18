package kubeconfig

import (
	b64 "encoding/base64"
	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)

func CreateKubeConfig(serviceAccountName string) (string, error) {
	clientset, _ := NewK8sClient()
	serviceAccount, err := clientset.CoreV1().ServiceAccounts(apiv1.NamespaceDefault).Get(serviceAccountName, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	secret, err := clientset.CoreV1().Secrets(apiv1.NamespaceDefault).Get(serviceAccount.Secrets[0].Name, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	nodes, err := clientset.CoreV1().Nodes().List(v1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master=true",
	})
	if err != nil {
		return "", err
	}

	token := string(secret.Data["token"])
	caData := b64.StdEncoding.EncodeToString(secret.Data["ca.crt"])
	server := "https://" + nodes.Items[0].Status.Addresses[0].Address + ":6443"

	tmp := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Config",
		"users": []interface{}{
			map[string]interface{}{
				"name": serviceAccountName,
				"user": map[string]interface{}{
					"token": token,
				},
			},
		},
		"clusters": []interface{}{
			map[string]interface{}{
				"cluster": map[string]interface{}{
					"certificate-authority-data": caData,
					"server":                     server,
				},
				"name": "service-cluster",
			},
		},
		"contexts": []interface{}{
			map[string]interface{}{
				"context": map[string]interface{}{
					"cluster": "service-cluster",
					"user":    serviceAccountName,
				},
				"name": serviceAccountName + "-context",
			},
		},
		"current-context": serviceAccountName + "-context",
	}
	bytes, _ := yaml.Marshal(tmp)
	return string(bytes), nil
}

func NewConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	}
	return config, err
}

func NewK8sClient() (*kubernetes.Clientset, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

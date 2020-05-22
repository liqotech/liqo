package kubeconfig

import (
	b64 "encoding/base64"
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

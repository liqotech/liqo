package discovery

import (
	b64 "encoding/base64"
	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"os"
)

func init() {
	dc := GetDiscoveryConfig()
	if dc.EnableAdvertisement {
		clientset, err := NewK8sClient()
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}

		cm, err := clientset.CoreV1().ConfigMaps(apiv1.NamespaceDefault).Get("credentials-provider-static-content", v1.GetOptions{})
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
		cm.Data["config.yaml"] = CreateKubeConfig()
		_, err = clientset.CoreV1().ConfigMaps(apiv1.NamespaceDefault).Update(cm)
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
	}
}

// create kubeconfig with no permission for foreign cluster
func CreateKubeConfig() string {
	clientset, _ := NewK8sClient()
	serviceAccount, err := clientset.CoreV1().ServiceAccounts(apiv1.NamespaceDefault).Get("unauth-user", v1.GetOptions{})
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	secret, err := clientset.CoreV1().Secrets(apiv1.NamespaceDefault).Get(serviceAccount.Secrets[0].Name, v1.GetOptions{})
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	nodes, err := clientset.CoreV1().Nodes().List(v1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master=true",
	})
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	token := string(secret.Data["token"])
	caData := b64.StdEncoding.EncodeToString(secret.Data["ca.crt"])
	server := "https://" + nodes.Items[0].Status.Addresses[0].Address + ":6443"

	tmp := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Config",
		"users": []interface{}{
			map[string]interface{}{
				"name": "unauth-user",
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
					"user":    "unauth-user",
				},
				"name": "unauth-user-context",
			},
		},
		"current-context": "unauth-user-context",
	}
	bytes, _ := yaml.Marshal(tmp)
	return string(bytes)
}

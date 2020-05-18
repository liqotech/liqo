package discovery

import (
	"crypto/sha1"
	b64 "encoding/base64"
	"encoding/hex"
	v1 "github.com/netgroup-polito/dronev2/api/discovery/v1"
	"io/ioutil"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"log"
	"net/http"
)

var (
	discoveryConfig *Config
)

func UpdateForeign(data []map[string]interface{}) {
	for _, txtData := range data {
		resp, err := http.Get(txtData["url"].(string))
		if err != nil {
			log.Println(err.Error())
			continue
		}
		config, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			log.Println(err.Error())
			continue
		}
		_, err = CreateForeignIfNotExists(config)
		if err != nil {
			log.Println(err.Error())
			continue
		}
	}
}

func GetClusterID(config []byte) (string, error) {
	kubeconfig := func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(config)
	}
	clientConfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfig)
	if err != nil {
		return "", err
	}
	hasher := sha1.New()
	hasher.Write([]byte(clientConfig.Host))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func CreateForeignIfNotExists(config []byte) (*v1.ForeignCluster, error) {
	client, _ := NewDiscoveryClient()
	clusterID, err := GetClusterID(config)
	if err != nil {
		return nil, err
	}

	if discoveryConfig == nil {
		discoveryConfig = GetDiscoveryConfig()
	}

	fc, err := client.ForeignClusters(apiv1.NamespaceDefault).Get(clusterID, metav1.GetOptions{})
	if err != nil && k8sErrors.IsNotFound(err) {
		// does not exists yet
		fc := v1.ForeignCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterID,
			},
			Spec: v1.ForeignClusterSpec{
				ClusterID:  clusterID,
				KubeConfig: b64.StdEncoding.EncodeToString(config),
				Federate:   discoveryConfig.AutoFederation,
			},
		}
		return client.ForeignClusters(apiv1.NamespaceDefault).Create(&fc)
	}

	return fc, nil
}

func LoadConfig(clusterID string) (*rest.Config, error) {
	client, _ := NewDiscoveryClient()
	fc, err := client.ForeignClusters(apiv1.NamespaceDefault).Get(clusterID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return fc.GetConfig()
}

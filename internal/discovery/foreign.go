package discovery

import (
	"crypto/sha1"
	b64 "encoding/base64"
	"encoding/hex"
	v1 "github.com/netgroup-polito/dronev2/api/discovery/v1"
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
	"io/ioutil"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"net/http"
)

var (
	discoveryConfig *Config
)

func UpdateForeign(data []*TxtData) {
	for _, txtData := range data {
		resp, err := http.Get(txtData.Url)
		if err != nil {
			Log.Error(err, err.Error())
			continue
		}
		config, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			Log.Error(err, err.Error())
			continue
		}
		_, err = createForeignIfNotExists(config)
		if err != nil {
			Log.Error(err, err.Error())
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

func createForeignIfNotExists(config []byte) (*v1.ForeignCluster, error) {
	client, err := clients.NewDiscoveryClient()
	if err != nil {
		return nil, err
	}
	clusterID, err := GetClusterID(config)
	if err != nil {
		return nil, err
	}

	if discoveryConfig == nil {
		discoveryConfig = GetDiscoveryConfig()
	}

	fc, err := client.ForeignClusters().Get(clusterID, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// does not exist yet
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
			return client.ForeignClusters().Create(&fc)
		}
		// other errors
		return nil, err
	}
	// already exists
	return fc, nil
}

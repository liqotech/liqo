package discovery

import (
	b64 "encoding/base64"
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

func (discovery *DiscoveryCtrl) UpdateForeign(data []*TxtData) {
	for _, txtData := range data {
		_, err := discovery.clientDiscovery.ForeignClusters().Get(txtData.ID, metav1.GetOptions{})
		if err == nil {
			// already exists
			continue
		}
		resp, err := http.Get(txtData.Config.Url)
		if err != nil {
			discovery.Log.Error(err, err.Error())
			continue
		}
		config, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			discovery.Log.Error(err, err.Error())
			continue
		}
		_, err = discovery.createForeign(config, txtData.ID)
		if err != nil {
			discovery.Log.Error(err, err.Error())
			continue
		}
		discovery.Log.Info("ForeignCluster " + txtData.ID + " created")
	}
}

func (discovery *DiscoveryCtrl) createForeign(config []byte, clusterID string) (*v1.ForeignCluster, error) {
	fc := v1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterID,
		},
		Spec: v1.ForeignClusterSpec{
			ClusterID:  clusterID,
			KubeConfig: b64.StdEncoding.EncodeToString(config),
			Federate:   discovery.config.AutoFederation,
		},
	}
	return discovery.clientDiscovery.ForeignClusters().Create(&fc)
}

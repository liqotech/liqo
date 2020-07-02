package discovery

import (
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func (discovery *DiscoveryCtrl) UpdateForeign(data []*TxtData) {
	for _, txtData := range data {
		_, err := discovery.clientDiscovery.ForeignClusters().Get(txtData.ID, metav1.GetOptions{})
		if err == nil {
			// already exists
			continue
		}
		_, err = discovery.createForeign(txtData)
		if err != nil {
			klog.Error(err, err.Error())
			continue
		}
		klog.Info("ForeignCluster " + txtData.ID + " created")
	}
}

func (discovery *DiscoveryCtrl) createForeign(txtData *TxtData) (*v1.ForeignCluster, error) {
	fc := &v1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: txtData.ID,
		},
		Spec: v1.ForeignClusterSpec{
			ClusterID: txtData.ID,
			Namespace: txtData.Namespace,
			Join:      discovery.Config.AutoJoin,
			ApiUrl:    txtData.ApiUrl,
		},
	}
	fc, err := discovery.clientDiscovery.ForeignClusters().Create(fc)
	if err != nil {
		return nil, err
	}
	return fc, err
}

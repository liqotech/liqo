package discovery

import (
	"errors"
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func (discovery *DiscoveryCtrl) UpdateForeign(data []*TxtData, sd *v1.SearchDomain) []*v1.ForeignCluster {
	createdForeign := []*v1.ForeignCluster{}
	for _, txtData := range data {
		if txtData.ID == discovery.ClusterId.GetClusterID() {
			// is local cluster
			continue
		}
		_, err := discovery.crdClient.Resource("foreignclusters").Get(txtData.ID, metav1.GetOptions{})
		if err == nil {
			// already exists
			continue
		}
		fc, err := discovery.createForeign(txtData, sd)
		if err != nil {
			klog.Error(err, err.Error())
			continue
		}
		klog.Info("ForeignCluster " + txtData.ID + " created")
		createdForeign = append(createdForeign, fc)
	}
	return createdForeign
}

func (discovery *DiscoveryCtrl) createForeign(txtData *TxtData, sd *v1.SearchDomain) (*v1.ForeignCluster, error) {
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
	if sd != nil {
		fc.Spec.Join = sd.Spec.AutoJoin
		fc.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: "discovery.liqo.io/v1",
				Kind:       "SearchDomain",
				Name:       sd.Name,
				UID:        sd.UID,
			},
		}
	}
	tmp, err := discovery.crdClient.Resource("foreignclusters").Create(fc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	fc, ok := tmp.(*v1.ForeignCluster)
	if !ok {
		return nil, errors.New("created object is not a ForeignCluster")
	}
	return fc, err
}

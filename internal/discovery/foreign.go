package discovery

import (
	"errors"
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func (discovery *DiscoveryCtrl) UpdateForeign(data []*TxtData, sd *v1.SearchDomain) []*v1.ForeignCluster {
	createdForeign := []*v1.ForeignCluster{}
	var discoveryType v1.DiscoveryType
	if sd == nil {
		discoveryType = v1.LanDiscovery
	} else {
		discoveryType = v1.WanDiscovery
	}
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
		fc, err := discovery.createForeign(txtData, sd, discoveryType)
		if err != nil {
			klog.Error(err, err.Error())
			continue
		}
		klog.Info("ForeignCluster " + txtData.ID + " created")
		createdForeign = append(createdForeign, fc)
	}
	if discoveryType == v1.LanDiscovery {
		_ = discovery.UpdateTtl(data)
	}
	return createdForeign
}

func (discovery *DiscoveryCtrl) UpdateTtl(txts []*TxtData) error {
	// find all ForeignCluster with discovery type LAN
	tmp, err := discovery.crdClient.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: "discovery-type=LAN",
	})
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	fcs, ok := tmp.(*v1.ForeignClusterList)
	if !ok {
		err = errors.New("retrieved object is not a ForeignClusterList")
		klog.Error(err, err.Error())
		return err
	}
	for _, fc := range fcs.Items {
		// find the ones that are not in the last retrieved list on LAN
		found := false
		for _, txt := range txts {
			if txt.ID == fc.Spec.ClusterID {
				found = true
				// if cluster TTL was decreased, reset it to default value
				if fc.Status.Ttl != 3 {
					fc.Status.Ttl = 3
					_, err = discovery.crdClient.Resource("foreignclusters").Update(fc.Name, &fc, metav1.UpdateOptions{})
					if err != nil {
						klog.Error(err, err.Error())
						continue
					}
				}
				break
			}
		}
		if !found {
			// if ForeignCluster is not in Txt list, reduce its TTL
			fc.Status.Ttl -= 1
			if fc.Status.Ttl <= 0 {
				// delete ForeignCluster
				err = discovery.crdClient.Resource("foreignclusters").Delete(fc.Name, metav1.DeleteOptions{})
				if err != nil {
					klog.Error(err, err.Error())
					continue
				}
			} else {
				// update ForeignCluster
				_, err = discovery.crdClient.Resource("foreignclusters").Update(fc.Name, &fc, metav1.UpdateOptions{})
				if err != nil {
					klog.Error(err, err.Error())
					continue
				}
			}
		}
	}
	return nil
}

func (discovery *DiscoveryCtrl) createForeign(txtData *TxtData, sd *v1.SearchDomain, discoveryType v1.DiscoveryType) (*v1.ForeignCluster, error) {
	fc := &v1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: txtData.ID,
		},
		Spec: v1.ForeignClusterSpec{
			ClusterID:     txtData.ID,
			Namespace:     txtData.Namespace,
			Join:          discovery.Config.AutoJoin,
			ApiUrl:        txtData.ApiUrl,
			DiscoveryType: discoveryType,
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
	if discoveryType == v1.LanDiscovery {
		// set TTL to default value
		fc.Status.Ttl = 3
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

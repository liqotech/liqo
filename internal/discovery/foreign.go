package discovery

import (
	"context"
	"errors"
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"strings"
)

// 1. checks if cluster ID is already known
// 2. if not exists, create it
// 3. else
//   3a. if IP is different set new IP and delete CA data
//   3b. else it is ok

func (discovery *DiscoveryCtrl) UpdateForeignLAN(data *discoveryData) {
	discoveryType := v1alpha1.LanDiscovery
	if data.TxtData.ID == discovery.ClusterId.GetClusterID() {
		// is local cluster
		return
	}

	err := retry.OnError(
		retry.DefaultRetry,
		func(err error) bool {
			return k8serror.IsConflict(err) || k8serror.IsAlreadyExists(err)
		},
		func() error {
			return discovery.createOrUpdate(data, nil, discoveryType, nil)
		})
	if err != nil {
		klog.Error(err)
	}
}

func (discovery *DiscoveryCtrl) UpdateForeignWAN(data []*TxtData, sd *v1alpha1.SearchDomain) []*v1alpha1.ForeignCluster {
	createdUpdatedForeign := []*v1alpha1.ForeignCluster{}
	discoveryType := v1alpha1.WanDiscovery
	for _, txtData := range data {
		if txtData.ID == discovery.ClusterId.GetClusterID() {
			// is local cluster
			continue
		}

		err := retry.OnError(
			retry.DefaultRetry,
			func(err error) bool {
				return k8serror.IsConflict(err) || k8serror.IsAlreadyExists(err)
			},
			func() error {
				return discovery.createOrUpdate(&discoveryData{ // TODO: discover auth service in wan discovery
					TxtData: txtData,
				}, sd, discoveryType, &createdUpdatedForeign)
			})
		if err != nil {
			continue
		}
	}
	return createdUpdatedForeign
}

func (discovery *DiscoveryCtrl) createOrUpdate(data *discoveryData, sd *v1alpha1.SearchDomain, discoveryType v1alpha1.DiscoveryType, createdUpdatedForeign *[]*v1alpha1.ForeignCluster) error {
	fc, err := discovery.GetForeignClusterByID(data.TxtData.ID)
	if k8serror.IsNotFound(err) {
		fc, err := discovery.createForeign(data, sd, discoveryType)
		if err != nil {
			klog.Error(err)
			return err
		}
		klog.Infof("ForeignCluster %s created", data.TxtData.ID)
		if createdUpdatedForeign != nil {
			*createdUpdatedForeign = append(*createdUpdatedForeign, fc)
		}
	} else if err == nil {
		var updated bool
		fc, updated, err = discovery.CheckUpdate(data, fc, discoveryType, sd)
		if err != nil {
			if !k8serror.IsConflict(err) {
				klog.Error(err)
			}
			return err
		}
		if updated {
			klog.Infof("ForeignCluster %s updated", data.TxtData.ID)
			if createdUpdatedForeign != nil {
				*createdUpdatedForeign = append(*createdUpdatedForeign, fc)
			}
		}
	} else {
		// unhandled errors
		klog.Error(err)
		return err
	}
	return nil
}

func (discovery *DiscoveryCtrl) createForeign(data *discoveryData, sd *v1alpha1.SearchDomain, discoveryType v1alpha1.DiscoveryType) (*v1alpha1.ForeignCluster, error) {
	fc := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: data.TxtData.ID,
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterIdentity: v1alpha1.ClusterIdentity{
				ClusterID:   data.TxtData.ID,
				ClusterName: data.TxtData.Name,
			},
			Namespace:     data.TxtData.Namespace,
			ApiUrl:        data.TxtData.ApiUrl,
			DiscoveryType: discoveryType,
		},
	}
	fc.LastUpdateNow()

	if sd != nil {
		fc.Spec.Join = sd.Spec.AutoJoin
		fc.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: "discovery.liqo.io/v1alpha1",
				Kind:       "SearchDomain",
				Name:       sd.Name,
				UID:        sd.UID,
			},
		}
	}
	if discoveryType == v1alpha1.LanDiscovery {
		// set TTL
		fc.Status.Ttl = data.TxtData.Ttl
	}
	tmp, err := discovery.crdClient.Resource("foreignclusters").Create(fc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}
	fc, ok := tmp.(*v1alpha1.ForeignCluster)
	if !ok {
		return nil, errors.New("created object is not a ForeignCluster")
	}
	return fc, err
}

// indicates that the remote cluster changed location, we have to reload all our infos about the remote cluster
func needsToDeleteRemoteResources(fc *v1alpha1.ForeignCluster, data *discoveryData) bool {
	return fc.Spec.ApiUrl != data.TxtData.ApiUrl || fc.Spec.Namespace != data.TxtData.Namespace
}

func (discovery *DiscoveryCtrl) CheckUpdate(data *discoveryData, fc *v1alpha1.ForeignCluster, discoveryType v1alpha1.DiscoveryType, searchDomain *v1alpha1.SearchDomain) (fcUpdated *v1alpha1.ForeignCluster, updated bool, err error) {
	needsToReload := needsToDeleteRemoteResources(fc, data)
	higherPriority := fc.HasHigherPriority(discoveryType) // the remote cluster didn't move, but we discovered it with an higher priority discovery type
	if needsToReload || higherPriority {
		// something is changed in ForeignCluster specs, update it
		fc.Spec.ApiUrl = data.TxtData.ApiUrl
		fc.Spec.Namespace = data.TxtData.Namespace
		fc.Spec.DiscoveryType = discoveryType
		if higherPriority && discoveryType == v1alpha1.LanDiscovery {
			// if the cluster was previously discovered with IncomingPeering discovery type, set join flag accordingly to LanDiscovery sets and set TTL
			fc.Spec.Join = discovery.Config.AutoJoin
			fc.Status.Ttl = data.TxtData.Ttl
		} else if searchDomain != nil && discoveryType == v1alpha1.WanDiscovery {
			fc.Spec.Join = searchDomain.Spec.AutoJoin
		}
		if needsToReload && fc.Status.Outgoing.CaDataRef != nil {
			// delete it only if the remote cluster moved
			err := discovery.crdClient.Client().CoreV1().Secrets(fc.Status.Outgoing.CaDataRef.Namespace).Delete(context.TODO(), fc.Status.Outgoing.CaDataRef.Name, metav1.DeleteOptions{})
			if err != nil {
				klog.Error(err)
				return nil, false, err
			}
			fc.Status.Outgoing.CaDataRef = nil
		}
		fc.LastUpdateNow()
		tmp, err := discovery.crdClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err)
			return nil, false, err
		}
		klog.V(4).Infof("TTL updated for ForeignCluster %v", fc.Name)
		fc, ok := tmp.(*v1alpha1.ForeignCluster)
		if !ok {
			err = errors.New("retrieved object is not a ForeignCluster")
			klog.Error(err)
			return nil, false, err
		}
		if needsToReload && fc.Status.Outgoing.Advertisement != nil {
			// delete it only if the remote cluster moved

			// changed ip in peered cluster, delete advertisement and wait for its recreation
			// TODO: find more sophisticated logic to not remove all resources on remote cluster
			advName := fc.Status.Outgoing.Advertisement.Name
			fc.Status.Outgoing.Advertisement = nil
			// updating it before adv delete will avoid us to set to false join flag
			tmp, err = discovery.crdClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
				return nil, false, err
			}
			fc, ok = tmp.(*v1alpha1.ForeignCluster)
			if !ok {
				err = errors.New("retrieved object is not a ForeignCluster")
				klog.Error(err)
				return nil, false, err
			}
			err = discovery.advClient.Resource("advertisements").Delete(advName, metav1.DeleteOptions{})
			if err != nil {
				klog.Error(err)
				return nil, false, err
			}
		}
		return fc, true, nil
	} else {
		// update "lastUpdate" annotation
		fc.LastUpdateNow()
		tmp, err := discovery.crdClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
		if err != nil {
			if !k8serror.IsConflict(err) {
				klog.Error(err)
			}
			return nil, false, err
		}
		var ok bool
		if fc, ok = tmp.(*v1alpha1.ForeignCluster); !ok {
			err = errors.New("retrieved object is not a ForeignCluster")
			klog.Error(err)
			return nil, false, err
		}
		return fc, false, nil
	}
}

func (discovery *DiscoveryCtrl) GetForeignClusterByID(clusterID string) (*v1alpha1.ForeignCluster, error) {
	tmp, err := discovery.crdClient.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: strings.Join([]string{"cluster-id", clusterID}, "="),
	})
	if err != nil {
		return nil, err
	}
	fcs, ok := tmp.(*v1alpha1.ForeignClusterList)
	if !ok || len(fcs.Items) == 0 {
		return nil, k8serror.NewNotFound(schema.GroupResource{
			Group:    v1alpha1.GroupVersion.Group,
			Resource: "foreignclusters",
		}, clusterID)
	}
	return &fcs.Items[0], nil
}

// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package discovery

import (
	"errors"
	"strings"

	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	discoveryPkg "github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

const defaultInsecureSkipTLSVerify = true

// updateForeignLAN updates a ForeignCluster discovered in the local network
// 1. checks if cluster ID is already known
// 2. if not exists, create it
// 3. else
//   3a. if IP is different set new IP and delete CA data
//   3b. else it is ok
func (discovery *Controller) updateForeignLAN(data *discoveryData) {
	discoveryType := discoveryPkg.LanDiscovery
	if data.ClusterInfo.ClusterID == discovery.LocalClusterID.GetClusterID() {
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

// UpdateForeignWAN updates the list of known foreign clusters:
// for each cluster retrieved with DNS discovery, if it is not the local cluster, check if it is already known, if not
// create it. In both cases update the ForeignCluster TTL
// This function also sets an owner reference and a label to the ForeignCluster pointing to the SearchDomain CR.
func (discovery *Controller) UpdateForeignWAN(data []*AuthData, sd *v1alpha1.SearchDomain) []*v1alpha1.ForeignCluster {
	createdUpdatedForeign := []*v1alpha1.ForeignCluster{}
	discoveryType := discoveryPkg.WanDiscovery
	for _, authData := range data {
		clusterInfo, err := discovery.getClusterInfo(defaultInsecureSkipTLSVerify, authData)
		if err != nil {
			klog.Error(err)
			continue
		}

		if clusterInfo.ClusterID == discovery.LocalClusterID.GetClusterID() {
			// is local cluster
			continue
		}

		data := *authData

		err = retry.OnError(
			retry.DefaultRetry,
			func(err error) bool {
				return k8serror.IsConflict(err) || k8serror.IsAlreadyExists(err)
			},
			func() error {
				return discovery.createOrUpdate(&discoveryData{
					AuthData:    &data,
					ClusterInfo: clusterInfo,
				}, sd, discoveryType, &createdUpdatedForeign)
			})
		if err != nil {
			klog.Error(err)
			continue
		}
	}
	return createdUpdatedForeign
}

func (discovery *Controller) createOrUpdate(data *discoveryData,
	sd *v1alpha1.SearchDomain, discoveryType discoveryPkg.Type, createdUpdatedForeign *[]*v1alpha1.ForeignCluster) error {
	fc, err := discovery.getForeignClusterByID(data.ClusterInfo.ClusterID)
	if k8serror.IsNotFound(err) {
		fc, err = discovery.createForeign(data, sd, discoveryType)
		if err != nil {
			klog.Error(err)
			return err
		}
		klog.Infof("ForeignCluster %s created", data.ClusterInfo.ClusterID)
		if createdUpdatedForeign != nil {
			*createdUpdatedForeign = append(*createdUpdatedForeign, fc)
		}
	}
	if err == nil {
		var updated bool
		fc, updated, err = discovery.checkUpdate(data, fc, discoveryType, sd)
		if err != nil {
			if !k8serror.IsConflict(err) {
				klog.Error(err)
			}
			return err
		}
		if updated {
			klog.Infof("ForeignCluster %s updated", data.ClusterInfo.ClusterID)
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

func (discovery *Controller) createForeign(
	data *discoveryData,
	sd *v1alpha1.SearchDomain, discoveryType discoveryPkg.Type) (*v1alpha1.ForeignCluster, error) {
	fc := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: data.ClusterInfo.ClusterID,
			Labels: map[string]string{
				discoveryPkg.DiscoveryTypeLabel: string(discoveryType),
				discoveryPkg.ClusterIDLabel:     data.ClusterInfo.ClusterID,
			},
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterIdentity: v1alpha1.ClusterIdentity{
				ClusterID:   data.ClusterInfo.ClusterID,
				ClusterName: data.ClusterInfo.ClusterName,
			},
			OutgoingPeeringEnabled: v1alpha1.PeeringEnabledAuto,
			IncomingPeeringEnabled: v1alpha1.PeeringEnabledAuto,
			ForeignAuthURL:         data.AuthData.getURL(),
			InsecureSkipTLSVerify:  pointer.BoolPtr(true),
		},
	}
	foreignclusterutils.LastUpdateNow(fc)

	if sd != nil {
		fc.Spec.OutgoingPeeringEnabled = v1alpha1.PeeringEnabledAuto
		fc.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: "discovery.liqo.io/v1alpha1",
				Kind:       "SearchDomain",
				Name:       sd.Name,
				UID:        sd.UID,
			},
		}
		if fc.Labels == nil {
			fc.Labels = map[string]string{}
		}
		fc.Labels[discoveryPkg.SearchDomainLabel] = sd.Name
	}
	// set TTL
	fc.Spec.TTL = int(data.AuthData.ttl)
	tmp, err := discovery.crdClient.Resource("foreignclusters").Create(fc, &metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	fc, ok := tmp.(*v1alpha1.ForeignCluster)
	if !ok {
		return nil, errors.New("created object is not a ForeignCluster")
	}
	return fc, err
}

func (discovery *Controller) checkUpdate(
	data *discoveryData, fc *v1alpha1.ForeignCluster,
	discoveryType discoveryPkg.Type,
	searchDomain *v1alpha1.SearchDomain) (fcUpdated *v1alpha1.ForeignCluster, updated bool, err error) {
	// the remote cluster didn't move, but we discovered it with an higher priority discovery type
	higherPriority := foreignclusterutils.HasHigherPriority(fc, discoveryType)
	if higherPriority {
		// something is changed in ForeignCluster specs, update it
		foreignclusterutils.SetDiscoveryType(fc, discoveryType)
		if higherPriority && discoveryType == discoveryPkg.LanDiscovery {
			// if the cluster was previously discovered with IncomingPeering discovery type, set join flag accordingly to LanDiscovery sets and set TTL
			fc.Spec.OutgoingPeeringEnabled = v1alpha1.PeeringEnabledAuto
			fc.Spec.TTL = int(data.AuthData.ttl)
		} else if searchDomain != nil && discoveryType == discoveryPkg.WanDiscovery {
			fc.Spec.OutgoingPeeringEnabled = v1alpha1.PeeringEnabledAuto
			fc.Spec.TTL = int(data.AuthData.ttl)
		}
		foreignclusterutils.LastUpdateNow(fc)
		tmp, err := discovery.crdClient.Resource("foreignclusters").Update(fc.Name, fc, &metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err)
			return nil, false, err
		}
		klog.V(4).Infof("TTL updated for ForeignCluster %v", fc.Name)
		var ok bool
		fc, ok = tmp.(*v1alpha1.ForeignCluster)
		if !ok {
			err = errors.New("retrieved object is not a ForeignCluster")
			klog.Error(err)
			return nil, false, err
		}
		return fc, true, nil
	}
	// update "lastUpdate" annotation
	foreignclusterutils.LastUpdateNow(fc)
	tmp, err := discovery.crdClient.Resource("foreignclusters").Update(fc.Name, fc, &metav1.UpdateOptions{})
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

func (discovery *Controller) getForeignClusterByID(clusterID string) (*v1alpha1.ForeignCluster, error) {
	tmp, err := discovery.crdClient.Resource("foreignclusters").List(&metav1.ListOptions{
		LabelSelector: strings.Join([]string{discoveryPkg.ClusterIDLabel, clusterID}, "="),
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
	return foreignclusterutils.GetOlderForeignCluster(fcs), nil
}

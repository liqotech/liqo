// Copyright 2019-2022 The Liqo Authors
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
	"context"
	"net/http"

	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	discoveryPkg "github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/discoverymanager/utils"
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
	ctx := context.TODO()

	discoveryType := discoveryPkg.LanDiscovery
	if data.ClusterInfo.ClusterID == discovery.LocalCluster.ClusterID {
		// is local cluster
		return
	}

	err := retry.OnError(
		retry.DefaultRetry,
		func(err error) bool {
			return k8serror.IsConflict(err) || k8serror.IsAlreadyExists(err)
		},
		func() error {
			return createOrUpdate(ctx, data, discovery.Client, nil, discoveryType, nil)
		})
	if err != nil {
		klog.Error(err)
	}
}

// UpdateForeignWAN updates the list of known foreign clusters:
// for each cluster retrieved with DNS discovery, if it is not the local cluster, check if it is already known, if not
// create it. In both cases update the ForeignCluster TTL
// This function also sets an owner reference and a label to the ForeignCluster pointing to the SearchDomain CR.
func UpdateForeignWAN(ctx context.Context, transport *http.Transport,
	cl client.Client, localCluster v1alpha1.ClusterIdentity,
	data []*AuthData, sd *v1alpha1.SearchDomain) []*v1alpha1.ForeignCluster {
	createdUpdatedForeign := []*v1alpha1.ForeignCluster{}
	discoveryType := discoveryPkg.WanDiscovery
	for _, authData := range data {
		clusterInfo, err := utils.GetClusterInfo(ctx, transport, authData.getURL())
		if err != nil {
			klog.Error(err)
			continue
		}

		if clusterInfo.ClusterID == localCluster.ClusterID {
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
				return createOrUpdate(ctx, &discoveryData{
					AuthData:    &data,
					ClusterInfo: clusterInfo,
				}, cl, sd, discoveryType, &createdUpdatedForeign)
			})
		if err != nil {
			klog.Error(err)
			continue
		}
	}
	return createdUpdatedForeign
}

func createOrUpdate(ctx context.Context, data *discoveryData, cl client.Client,
	sd *v1alpha1.SearchDomain, discoveryType discoveryPkg.Type, createdUpdatedForeign *[]*v1alpha1.ForeignCluster) error {
	fc, err := foreignclusterutils.GetForeignClusterByID(ctx, cl, data.ClusterInfo.ClusterID)
	if k8serror.IsNotFound(err) {
		fc, err = createForeign(ctx, cl, data, sd, discoveryType)
		if err != nil {
			klog.Error(err)
			return err
		}
		klog.Infof("ForeignCluster %s created", fc.Spec.ClusterIdentity)
		if createdUpdatedForeign != nil {
			*createdUpdatedForeign = append(*createdUpdatedForeign, fc)
		}
	}
	if err == nil {
		var updated bool
		fc, updated, err = checkUpdate(ctx, cl, data, fc, discoveryType, sd)
		if err != nil {
			if !k8serror.IsConflict(err) {
				klog.Error(err)
			}
			return err
		}
		if updated {
			klog.Infof("ForeignCluster %s updated", fc.Spec.ClusterIdentity)
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

func createForeign(
	ctx context.Context, cl client.Client, data *discoveryData,
	sd *v1alpha1.SearchDomain, discoveryType discoveryPkg.Type) (*v1alpha1.ForeignCluster, error) {
	identity := v1alpha1.ClusterIdentity{
		ClusterID:   data.ClusterInfo.ClusterID,
		ClusterName: data.ClusterInfo.ClusterName,
	}
	fc := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: foreignclusterutils.UniqueName(&identity),
			Labels: map[string]string{
				discoveryPkg.DiscoveryTypeLabel: string(discoveryType),
				discoveryPkg.ClusterIDLabel:     identity.ClusterID,
			},
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterIdentity:        identity,
			OutgoingPeeringEnabled: v1alpha1.PeeringEnabledAuto,
			IncomingPeeringEnabled: v1alpha1.PeeringEnabledAuto,
			ForeignAuthURL:         data.AuthData.getURL(),
			InsecureSkipTLSVerify:  pointer.BoolPtr(true),
		},
	}
	foreignclusterutils.LastUpdateNow(fc)

	if sd != nil {
		fc.Spec.OutgoingPeeringEnabled = v1alpha1.PeeringEnabledAuto
		fc.Labels[discoveryPkg.SearchDomainLabel] = sd.Name
		if err := controllerutil.SetOwnerReference(sd, fc, cl.Scheme()); err != nil {
			klog.Errorf("Failed to set foreign cluster owner reference: %v", err)
			return nil, err
		}
	}
	// set TTL
	fc.Spec.TTL = int(data.AuthData.ttl)

	if err := cl.Create(ctx, fc); err != nil {
		klog.Error(err)
		return nil, err
	}

	return fc, nil
}

func checkUpdate(
	ctx context.Context, cl client.Client,
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

		if err := cl.Update(ctx, fc); err != nil {
			klog.Error(err)
			return nil, false, err
		}

		klog.V(4).Infof("TTL updated for ForeignCluster %v", fc.Name)
		return fc, true, nil
	}

	// update "lastUpdate" annotation
	foreignclusterutils.LastUpdateNow(fc)
	if err := cl.Update(ctx, fc); err != nil {
		if !k8serror.IsConflict(err) {
			klog.Error(err)
		}
		return nil, false, err
	}

	return fc, false, nil
}

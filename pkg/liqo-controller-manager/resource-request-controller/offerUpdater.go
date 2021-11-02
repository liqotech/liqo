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

package resourcerequestoperator

import (
	"context"
	"fmt"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sync"
)

type ResourceReaderInterface interface {
	Start(ctx context.Context, group *sync.WaitGroup)
	// ReadResources returns the resources available for usage by the given cluster.
	ReadResources(clusterID string) corev1.ResourceList
	// RemoveClusterID removes the given clusterID from all internal structures.
	RemoveClusterID(clusterID string)
}

// OfferUpdater is a component that responds to ResourceRequests with the cluster's resources read from ResourceReader.
type OfferUpdater struct {
	ResourceReader ResourceReaderInterface
	OfferQueue

	client        client.Client
	homeCluster               discoveryv1alpha1.ClusterIdentity
	clusterLabels map[string]string
	scheme        *runtime.Scheme
	localRealStorageClassName string
	enableStorage             bool
}

func NewOfferUpdater(client client.Client, homeCluster discoveryv1alpha1.ClusterIdentity, clusterLabels map[string]string,
	scheme *runtime.Scheme, localRealStorageClassName string, enableStorage bool) *OfferUpdater {
	advertiser := &OfferUpdater{
		client: client,
		homeCluster: homeCluster,
		clusterLabels: clusterLabels,
		scheme: scheme,
		localRealStorageClassName: localRealStorageClassName,
		enableStorage: enableStorage,
	}
	advertiser.OfferQueue = NewOfferQueue(advertiser)
	return advertiser
}

func (a *OfferUpdater) Start(ctx context.Context, wg *sync.WaitGroup) {
	a.ResourceReader.Start(ctx, wg)
	a.OfferQueue.Start(ctx, wg)
}

func (u *OfferUpdater) CreateOrUpdateOffer(cluster discoveryv1alpha1.ClusterIdentity) (requeue bool, err error) {
	ctx := context.Background()
	list, err := u.getResourceRequest(ctx, cluster.ClusterID)
	if err != nil {
		return true, err
	} else if len(list.Items) != 1 {
		// invalid clusterID so return requeue = false. The clusterID will be removed from the workqueue and broadcaster maps.
		u.ResourceReader.RemoveClusterID(cluster.ClusterID)
		u.OfferQueue.RemoveClusterID(cluster.ClusterID)
		return false, fmt.Errorf("ClusterID %s is no longer valid. Deleting", cluster.ClusterName)
	}
	request := list.Items[0]
	resources := u.ResourceReader.ReadResources(cluster.ClusterID)
	if resourceIsEmpty(resources) {
		klog.Warningf("No resources for cluster %s, requeuing", cluster.ClusterName)
		return true, nil
	}
	offer := &sharingv1alpha1.ResourceOffer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.GetNamespace(),
			Name:      offerPrefix + u.homeCluster.ClusterID,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, u.client, offer, func() error {
		if offer.Labels != nil {
			offer.Labels[discovery.ClusterIDLabel] = request.Spec.ClusterIdentity.ClusterID
			offer.Labels[consts.ReplicationRequestedLabel] = "true"
			offer.Labels[consts.ReplicationDestinationLabel] = request.Spec.ClusterIdentity.ClusterID
		} else {
			offer.Labels = map[string]string{
				discovery.ClusterIDLabel:           request.Spec.ClusterIdentity.ClusterID,
				consts.ReplicationRequestedLabel:   "true",
				consts.ReplicationDestinationLabel: request.Spec.ClusterIdentity.ClusterID,
			}
		}
		offer.Spec.ClusterId = u.homeCluster.ClusterID
		offer.Spec.ResourceQuota.Hard = resources.DeepCopy()
		offer.Spec.Labels = u.clusterLabels

		offer.Spec.StorageClasses, err = u.getStorageClasses(ctx)
		if err != nil {
			return err
		}

		return controllerutil.SetControllerReference(&request, offer, u.scheme)
	})

	if err != nil {
		klog.Error(err)
		return true, err
	}
	klog.Infof("%s -> %s Offer: %s/%s", u.homeCluster.ClusterName, op, offer.Namespace, offer.Name)
	return false, nil
}

func (a *OfferUpdater) RemoveClusterID(clusterID string) {
	a.ResourceReader.RemoveClusterID(clusterID)
	a.OfferQueue.RemoveClusterID(clusterID)
}

// getResourceRequest returns the list of ResourceRequests for the given cluster.
func (a *OfferUpdater) getResourceRequest(ctx context.Context, clusterID string) (*discoveryv1alpha1.ResourceRequestList, error) {
	resourceRequestList := &discoveryv1alpha1.ResourceRequestList{}
	err := a.client.List(ctx, resourceRequestList, client.MatchingLabels{
		consts.ReplicationOriginLabel: clusterID,
	})
	if err != nil {
		return nil, err
	}

	return resourceRequestList, nil
}

func (u *OfferUpdater) getStorageClasses(ctx context.Context) ([]sharingv1alpha1.StorageType, error) {
	if !u.enableStorage {
		return []sharingv1alpha1.StorageType{}, nil
	}

	storageClassList := &storagev1.StorageClassList{}
	err := u.client.List(ctx, storageClassList)
	if err != nil {
		return nil, err
	}

	storageTypes := make([]sharingv1alpha1.StorageType, len(storageClassList.Items))
	for i := range storageClassList.Items {
		class := &storageClassList.Items[i]
		storageTypes[i].StorageClassName = class.GetName()

		// set the storage class as default if:
		// 1. it is the real storage class of the local cluster
		// 2. no local real storage class is set and it is the cluster default storage class
		if u.localRealStorageClassName == "" {
			if val, ok := class.Annotations["storageclass.kubernetes.io/is-default-class"]; ok && val == "true" {
				storageTypes[i].Default = true
			}
		} else if class.GetName() == u.localRealStorageClassName {
			storageTypes[i].Default = true
		}
	}

	return storageTypes, nil
}

// resourceIsEmpty checks if the ResourceList is empty.
func resourceIsEmpty(list corev1.ResourceList) bool {
	for _, val := range list {
		if !val.IsZero() {
			return false
		}
	}
	return true
}

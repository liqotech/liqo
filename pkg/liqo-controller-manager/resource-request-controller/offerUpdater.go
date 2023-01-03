// Copyright 2019-2023 The Liqo Authors
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
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	resourcemonitors "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller/resource-monitors"
)

// OfferUpdater is a component that responds to ResourceRequests with the cluster's resources read from ResourceReader.
type OfferUpdater struct {
	resourcemonitors.ResourceReader
	OfferQueue

	client                    client.Client
	homeCluster               discoveryv1alpha1.ClusterIdentity
	clusterLabels             map[string]string
	scheme                    *runtime.Scheme
	localRealStorageClassName string
	enableStorage             bool
	// currentResources maps the clusters that we intend to offer resources to, to the resource list that we last used
	// when issuing them a ResourceOffer.
	currentResources map[string]corev1.ResourceList
	// updateThresholdPercentage is the change in resources that triggers an update of ResourceOffers.
	updateThresholdPercentage uint

	clusterIdentityCache map[string]discoveryv1alpha1.ClusterIdentity
}

// NewOfferUpdater constructs a new OfferUpdater.
func NewOfferUpdater(ctx context.Context, k8sClient client.Client, homeCluster discoveryv1alpha1.ClusterIdentity,
	clusterLabels map[string]string, reader resourcemonitors.ResourceReader, updateThresholdPercentage uint,
	localRealStorageClassName string, enableStorage bool) *OfferUpdater {
	updater := &OfferUpdater{
		ResourceReader:            reader,
		client:                    k8sClient,
		homeCluster:               homeCluster,
		clusterLabels:             clusterLabels,
		scheme:                    k8sClient.Scheme(),
		localRealStorageClassName: localRealStorageClassName,
		enableStorage:             enableStorage,
		currentResources:          map[string]corev1.ResourceList{},
		updateThresholdPercentage: updateThresholdPercentage,
		clusterIdentityCache:      map[string]discoveryv1alpha1.ClusterIdentity{},
	}
	updater.OfferQueue = NewOfferQueue(updater)
	reader.Register(ctx, updater)
	return updater
}

// Start starts the OfferUpdater and blocks.
func (u *OfferUpdater) Start(ctx context.Context) error {
	return u.OfferQueue.Start(ctx)
}

// CreateOrUpdateOffer creates an offer into the given cluster, reading resources from the ResourceReader.
func (u *OfferUpdater) CreateOrUpdateOffer(cluster discoveryv1alpha1.ClusterIdentity) (requeue bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := GetResourceRequest(ctx, u.client, cluster.ClusterID)
	if err != nil {
		return true, err
	}
	if request == nil {
		// invalid clusterID so return requeue = false. The clusterID will be removed from the workqueue and
		// the resourcereader (in a daisy chain if there are multiple).
		err := u.ResourceReader.RemoveClusterID(ctx, cluster.ClusterID)
		if err != nil {
			return true, fmt.Errorf("error while removing cluster ID from external resource monitor: %w", err)
		}
		delete(u.currentResources, cluster.ClusterID)
		return false, fmt.Errorf("cluster %s is no longer valid and was deleted", cluster.ClusterName)
	}
	resources, err := u.ResourceReader.ReadResources(ctx, cluster.ClusterID)
	if err != nil {
		return true, fmt.Errorf("error while reading resources from external resource monitor: %w", err)
	}
	u.currentResources[cluster.ClusterID] = resources.DeepCopy()
	u.clusterIdentityCache[cluster.ClusterID] = cluster
	offer := &sharingv1alpha1.ResourceOffer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.GetNamespace(),
			Name:      getOfferName(u.homeCluster),
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
		offer.Spec.ClusterID = u.homeCluster.ClusterID
		offer.Spec.ResourceQuota.Hard = resources.DeepCopy()
		offer.Spec.Labels = u.clusterLabels

		offer.Spec.StorageClasses, err = u.getStorageClasses(ctx)
		if err != nil {
			return err
		}

		return controllerutil.SetControllerReference(request, offer, u.scheme)
	})

	if err != nil {
		return true, err
	}
	klog.Infof("%s -> %s Offer: %s/%s", u.homeCluster.ClusterName, op, offer.Namespace, offer.Name)
	return false, nil
}

// NotifyChange is used by the ResourceReader to notify that resources were changed for a single cluster
// identified by clusterID or for all clusters by passing resourcemonitors.AllClusterIDs.
func (u *OfferUpdater) NotifyChange(clusterID string) {
	if clusterID == resourcemonitors.AllClusterIDs {
		for clusterID := range u.currentResources {
			if u.shouldUpdate(clusterID) {
				u.OfferQueue.Push(u.clusterIdentityCache[clusterID])
			}
		}
	} else {
		u.OfferQueue.Push(u.clusterIdentityCache[clusterID])
	}
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

// SetThreshold sets the threshold for resource updates to trigger an update of the ResourceOffers.
func (u *OfferUpdater) SetThreshold(updateThresholdPercentage uint) {
	u.updateThresholdPercentage = updateThresholdPercentage
	u.NotifyChange(resourcemonitors.AllClusterIDs)
}

// shouldUpdate checks if the resources have changed by at least updateThresholdPercentage since the last update.
// checks are skipped if u.updateThresholdPercentage is 0.
func (u *OfferUpdater) shouldUpdate(clusterID string) bool {
	if u.updateThresholdPercentage == 0 {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	oldResources := u.currentResources[clusterID]
	newResources, err := u.ResourceReader.ReadResources(ctx, clusterID)
	if err != nil {
		klog.Errorf("Error while reading resources from external monitor: %w", err)
		// returns true when an error occurs in order to enqueue again the offer update, otherwise the update will be lost.
		return true
	}
	// Check for any resources removed
	for oldResourceName := range oldResources {
		if _, exists := newResources[oldResourceName]; !exists {
			return true
		}
	}
	// Check for any resources added
	for newResourceName := range newResources {
		if _, exists := oldResources[newResourceName]; !exists {
			return true
		}
	}
	for resourceName, newValue := range newResources {
		oldValue := oldResources[resourceName]
		absDiff := math.Abs(float64(newValue.Value() - oldValue.Value()))
		if int64(absDiff) > oldValue.Value()*int64(u.updateThresholdPercentage)/100 {
			return true
		}
	}

	return false
}

// GetResourceRequest returns ResourceRequest for the given cluster.
func GetResourceRequest(ctx context.Context, k8sClient client.Client, clusterID string) (
	*discoveryv1alpha1.ResourceRequest, error) {
	resourceRequestList := &discoveryv1alpha1.ResourceRequestList{}
	err := k8sClient.List(ctx, resourceRequestList,
		client.HasLabels{consts.ReplicationStatusLabel},
		client.MatchingLabels{consts.ReplicationOriginLabel: clusterID},
	)
	if err != nil {
		return nil, err
	}

	if len(resourceRequestList.Items) > 1 {
		return nil, fmt.Errorf("more than one resource request found for clusterID %s", clusterID)
	}
	if len(resourceRequestList.Items) == 0 {
		return nil, nil
	}
	return &resourceRequestList.Items[0], nil
}

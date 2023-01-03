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

package shadowpod

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharing "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// peeringCache is a cache that holds the peeringInfo for each peered cluster.
type peeringCache struct {
	peeringInfo sync.Map
	ready       bool
}

/**
 * PeeringCache methods
 */

// CacheRefresher is a wrapper function that receives a ShadowPodValidator
// and starts a PollImmediateInfinite timer to periodically refresh the cache.
func (spv *Validator) CacheRefresher(interval time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return wait.PollImmediateInfiniteWithContext(ctx, interval, spv.refreshCache)
	}
}

func (spv *Validator) initializeCache(ctx context.Context) (err error) {
	resourceOfferList := sharing.ResourceOfferList{}
	if err := spv.client.List(ctx, &resourceOfferList, &client.ListOptions{LabelSelector: liqolabels.LocalLabelSelector()}); err != nil {
		return err
	}
	klog.Infof("Cache initialization started")
	for i := range resourceOfferList.Items {
		ro := &resourceOfferList.Items[i]
		clusterID := ro.Labels[discovery.ClusterIDLabel]
		if clusterID == "" {
			klog.Warningf("ResourceOffer %s/%s has no cluster id", ro.Namespace, ro.Name)
			continue
		}
		clusterName := retrieveClusterName(ctx, spv.client, clusterID)
		klog.V(4).Infof("Generating PeeringInfo in cache for corresponding ResourceOffer %s", klog.KObj(ro))
		pi := createPeeringInfo(discoveryv1alpha1.ClusterIdentity{
			ClusterID:   clusterID,
			ClusterName: clusterName,
		}, ro.Spec.ResourceQuota.Hard)

		// Get the List of shadow pods running on the cluster with a given clusterID
		shadowPodList, err := spv.getShadowPodListByClusterID(ctx, clusterID)
		if err != nil {
			return err
		}

		klog.V(5).Infof("Found %d ShadowPods running on cluster %s", len(shadowPodList.Items), pi.clusterIdentity.String())
		pi.alignExistingShadowPods(shadowPodList)

		spv.PeeringCache.peeringInfo.Store(clusterID, pi)
	}
	klog.Infof("Cache initialization completed. Found %d peered clusters", len(resourceOfferList.Items))
	spv.PeeringCache.ready = true
	return nil
}

func (spv *Validator) refreshCache(ctx context.Context) (done bool, err error) {
	if !spv.PeeringCache.ready {
		err = spv.initializeCache(ctx)
		if err != nil {
			klog.Warning(err)
		}
		return false, nil
	}
	klog.V(4).Infof("Cache refresh started")
	// Cycling on all recorded peering in Cache
	spv.PeeringCache.peeringInfo.Range(
		func(key, value interface{}) bool {
			pi := value.(*peeringInfo)
			clusterID := key.(string)
			// Get the List of shadow pods running on the cluster
			shadowPodList, err := spv.getShadowPodListByClusterID(ctx, clusterID)
			if err != nil {
				klog.Warning(err)
				return true
			}
			klog.V(5).Infof("Found %d ShadowPods for cluster %q", len(shadowPodList.Items), pi.clusterIdentity.String())
			// Flush terminating ShadowPods and check the correct alignment between cluster and cache
			klog.V(5).Infof("Aligning ShadowPods for cluster %q", pi.clusterIdentity.String())
			pi.alignTerminatingOrNotExistingShadowPods(shadowPodList)
			return true
		},
	)

	klog.V(5).Infof("Aligning ResourceOffers - PeeringInfo")
	if err := spv.checkAlignmentResourceOfferPeeringInfo(ctx); err != nil {
		klog.Error(err)
		return false, nil
	}
	klog.V(4).Infof("Cache refresh completed")
	return false, nil
}

func (pi *peeringInfo) alignTerminatingOrNotExistingShadowPods(shadowPodList *vkv1alpha1.ShadowPodList) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	spMap := make(map[string]struct{})
	// Check on all cluster ShadowPods and saving a list of them in a temporary map
	for i := range shadowPodList.Items {
		nsname := types.NamespacedName{Name: shadowPodList.Items[i].Name, Namespace: shadowPodList.Items[i].Namespace}
		found := pi.checkAndAddShadowPods(&shadowPodList.Items[i], nsname)
		if !found {
			klog.Warningf("Warning: ShadowPod %s not found in cache, added", nsname.String())
		}
		spMap[nsname.String()] = struct{}{}
	}
	klog.V(5).Infof("Searching for terminated ShadowPodDescription to be removed from cache")
	// Alignment of all ShadowPodDescriptions in cache
	pi.alignTerminatingShadowPodDescriptions(spMap)
}

func (pi *peeringInfo) alignTerminatingShadowPodDescriptions(spMap map[string]struct{}) {
	for _, shadowPodDescription := range pi.shadowPods {
		// Check if the ShadowPod is in terminating phase in cache and has been already terminated/deleted from the cluster
		// if true ShadowPodDescription can be also deleted from the cache
		if _, stillPresent := spMap[shadowPodDescription.namespacedName.String()]; !stillPresent {
			if !shadowPodDescription.running {
				pi.removeShadowPod(shadowPodDescription)
				klog.V(5).Infof("ShadowPodDescription %s removed from cache", shadowPodDescription.namespacedName.String())
			} else if time.Since(shadowPodDescription.creationTimestamp) > 30*time.Second {
				pi.terminateShadowPod(shadowPodDescription)
				pi.removeShadowPod(shadowPodDescription)
			}
		}
	}
}

func (pi *peeringInfo) alignExistingShadowPods(shadowPodList *vkv1alpha1.ShadowPodList) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	for i := range shadowPodList.Items {
		nsname := types.NamespacedName{Name: shadowPodList.Items[i].Name, Namespace: shadowPodList.Items[i].Namespace}
		found := pi.checkAndAddShadowPods(&shadowPodList.Items[i], nsname)
		if !found {
			klog.V(4).Infof("ShadowPod %s added in cache", nsname.String())
		}
	}
}

func (pi *peeringInfo) checkAndAddShadowPods(shadowPod *vkv1alpha1.ShadowPod, nsname types.NamespacedName) (found bool) {
	_, found = pi.shadowPods[nsname.String()]
	if !found {
		// Errors are intentionally ignored here.
		spQuota, _ := getQuotaFromShadowPod(shadowPod, false)
		pi.addShadowPod(createShadowPodDescription(shadowPod.GetName(), shadowPod.GetNamespace(), shadowPod.GetUID(), *spQuota))
	}
	return
}

func (spv *Validator) checkAlignmentResourceOfferPeeringInfo(ctx context.Context) error {
	resourceOfferList := sharing.ResourceOfferList{}
	roMap := make(map[string]struct{})

	// Get the List of resource offers
	if err := spv.client.List(ctx, &resourceOfferList, &client.ListOptions{LabelSelector: liqolabels.LocalLabelSelector()}); err != nil {
		return err
	}

	// Check if there are new ResourceOffers in the system snapshot
	for i := range resourceOfferList.Items {
		ro := &resourceOfferList.Items[i]
		clusterID := ro.Labels[discovery.ClusterIDLabel]
		if clusterID == "" {
			return fmt.Errorf("ResourceOffer %q has no cluster id", ro.Name)
		}
		clusterName := retrieveClusterName(ctx, spv.client, clusterID)
		// Populating a map of valid  and existing ResourceOffers in the system snapshot
		roMap[clusterID] = struct{}{}

		// Check if the ResourceOffer is not present in the cache
		if newPI, found := spv.PeeringCache.peeringInfo.LoadOrStore(clusterID, createPeeringInfo(discoveryv1alpha1.ClusterIdentity{
			ClusterID:   clusterID,
			ClusterName: clusterName,
		}, ro.Spec.ResourceQuota.Hard)); !found {
			klog.V(4).Infof("ResourceOffer %q not found in cache, adding it", clusterName)
			// Get the List of ShadowPods running on the cluster
			shadowPodList, err := spv.getShadowPodListByClusterID(ctx, clusterID)
			if err != nil {
				return err
			}
			klog.V(5).Infof("Found %d ShadowPods running on cluster %s", len(shadowPodList.Items), newPI.(*peeringInfo).clusterIdentity.String())
			newPI.(*peeringInfo).alignExistingShadowPods(shadowPodList)
		}
	}

	// Check if PeeringInfos still have corresponding ResourceOffers
	spv.PeeringCache.peeringInfo.Range(
		func(key, value interface{}) bool {
			peeringInfo := value.(*peeringInfo)
			clusterID := key.(string)
			// If the corresponding ResourceOffer is not present anymore, remove the PeeringInfo from the cache
			if _, found := roMap[clusterID]; !found {
				klog.V(4).Infof("ResourceOffer %q not found in system snapshot, removing it from cache", peeringInfo.clusterIdentity.String())
				spv.PeeringCache.peeringInfo.Delete(clusterID)
			}
			return true
		},
	)
	return nil
}

func (pi *peeringInfo) alignResourceOfferUpdates(resources corev1.ResourceList) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	pi.updateQuotas(resources)
	klog.V(4).Infof("Quota of PeeringInfo for cluster %q has been updated", pi.clusterIdentity.String())
}

func retrieveClusterName(ctx context.Context, c client.Client, clusterID string) string {
	cluster, err := fcutils.GetForeignClusterByID(ctx, c, clusterID)
	if err != nil {
		klog.Warning(err)
		return ""
	}
	return cluster.Spec.ClusterIdentity.ClusterName
}

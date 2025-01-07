// Copyright 2019-2025 The Liqo Authors
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
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/getters"
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
	quotaList := offloadingv1beta1.QuotaList{}
	if err := spv.client.List(ctx, &quotaList); err != nil {
		return err
	}
	klog.Infof("Cache initialization started")
	for i := range quotaList.Items {
		q := &quotaList.Items[i]
		klog.V(4).Infof("Generating PeeringInfo in cache for corresponding Quota %s", klog.KObj(q))
		pi := createPeeringInfo(q.Spec.User, q.Spec.Resources)

		// Get the List of shadow pods running on the cluster with a given creator
		shadowPodList, err := getters.ListShadowPodsByCreator(ctx, spv.client, q.Spec.User)
		if err != nil {
			return err
		}

		klog.V(5).Infof("Found %d ShadowPods running for user %s", len(shadowPodList.Items), q.Spec.User)
		pi.alignExistingShadowPods(shadowPodList)

		spv.PeeringCache.peeringInfo.Store(q.Spec.User, pi)
	}
	klog.Infof("Cache initialization completed. Found %d Quotas", len(quotaList.Items))
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
			// Get the List of shadow pods running for a given creator
			shadowPodList, err := getters.ListShadowPodsByCreator(ctx, spv.client, pi.userName)
			if err != nil {
				klog.Warning(err)
				return true
			}
			klog.V(5).Infof("Found %d ShadowPods for user %q", len(shadowPodList.Items), pi.userName)
			// Flush terminating ShadowPods and check the correct alignment between users and cache
			klog.V(5).Infof("Aligning ShadowPods for user %q", pi.userName)
			pi.alignTerminatingOrNotExistingShadowPods(shadowPodList)
			return true
		},
	)

	klog.V(5).Infof("Aligning Quotas - PeeringInfo")
	if err := spv.checkAlignmentQuotaPeeringInfo(ctx); err != nil {
		klog.Error(err)
		return false, nil
	}
	klog.V(4).Infof("Cache refresh completed")
	return false, nil
}

func (pi *peeringInfo) alignTerminatingOrNotExistingShadowPods(shadowPodList *offloadingv1beta1.ShadowPodList) {
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

func (pi *peeringInfo) alignExistingShadowPods(shadowPodList *offloadingv1beta1.ShadowPodList) {
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

func (pi *peeringInfo) checkAndAddShadowPods(shadowPod *offloadingv1beta1.ShadowPod, nsname types.NamespacedName) (found bool) {
	_, found = pi.shadowPods[nsname.String()]
	if !found {
		// Errors are intentionally ignored here.
		spQuota, _ := getQuotaFromShadowPod(shadowPod, offloadingv1beta1.NoLimitsEnforcement)
		pi.addShadowPod(createShadowPodDescription(shadowPod.GetName(), shadowPod.GetNamespace(), shadowPod.GetUID(), *spQuota))
	}
	return
}

func (spv *Validator) checkAlignmentQuotaPeeringInfo(ctx context.Context) error {
	quotaList := offloadingv1beta1.QuotaList{}
	quotaMap := make(map[string]struct{})

	// Get the List of quotas
	if err := spv.client.List(ctx, &quotaList); err != nil {
		return err
	}

	// Check if there are new Quotas in the system snapshot
	for i := range quotaList.Items {
		quota := &quotaList.Items[i]
		// Populating a map of valid and existing Quotas in the system snapshot
		quotaMap[quota.Spec.User] = struct{}{}

		// Check if the Quota is not present in the cache
		if newPI, found := spv.PeeringCache.peeringInfo.LoadOrStore(quota.Spec.User,
			createPeeringInfo(quota.Spec.User, quota.Spec.Resources)); !found {
			klog.V(4).Infof("Quota for user %q not found in cache, adding it", quota.Spec.User)
			// Get the List of ShadowPods running for a given creator
			shadowPodList, err := getters.ListShadowPodsByCreator(ctx, spv.client, quota.Spec.User)
			if err != nil {
				return err
			}
			klog.V(5).Infof("Found %d ShadowPods running for user %s", len(shadowPodList.Items), quota.Spec.User)
			newPI.(*peeringInfo).alignExistingShadowPods(shadowPodList)
		}
	}

	// Check if PeeringInfos still have corresponding Quotas
	spv.PeeringCache.peeringInfo.Range(
		func(key, value interface{}) bool {
			peeringInfo := value.(*peeringInfo)
			userName := key.(string)
			// If the corresponding Quota is not present anymore, remove the PeeringInfo from the cache
			if _, found := quotaMap[userName]; !found {
				klog.V(4).Infof("Quota for user %q not found in system snapshot, removing it from cache", peeringInfo.userName)
				spv.PeeringCache.peeringInfo.Delete(userName)
			}
			return true
		},
	)
	return nil
}

func (pi *peeringInfo) alignQuotaUpdates(resources corev1.ResourceList) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	pi.updateQuotas(resources)
	klog.V(4).Infof("Quota of PeeringInfo for user %q has been updated", pi.userName)
}

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
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

// PeeringInfo is the struct that holds the information about the peering with a remote cluster.
type peeringInfo struct {
	userName   string
	shadowPods map[string]*Description
	totalQuota corev1.ResourceList
	usedQuota  corev1.ResourceList
	mu         sync.RWMutex
}

/**
 * PeeringInfo methods
 */

// createPeeringInfo creates a new PeeringInfo struct.
func createPeeringInfo(userName string, resources corev1.ResourceList) *peeringInfo {
	return &peeringInfo{
		userName:   userName,
		shadowPods: map[string]*Description{},
		totalQuota: resources,
		usedQuota:  generateQuotaPattern(resources),
	}
}

// getOrCreatePeeringInfo returns the PeeringInfo struct for the given userName. If it doesn't exist, it creates a new one.
func (pc *peeringCache) getOrCreatePeeringInfo(userName string, roQuota corev1.ResourceList) *peeringInfo {
	pi, found := pc.peeringInfo.LoadOrStore(userName, createPeeringInfo(userName, roQuota))
	if !found {
		klog.V(4).Infof("PeeringInfo not found for user %q, created...", userName)
		klog.V(5).Infof("New Quota limits for user %q %s", userName, quotaFormatter(pi.(*peeringInfo).totalQuota))
		return pi.(*peeringInfo)
	}
	pi.(*peeringInfo).alignQuotaUpdates(roQuota)
	return pi.(*peeringInfo)
}

// getPeeringInfo returns the PeeringInfo struct for the given userName. If it doesn't exist, it returns nil.
func (pc *peeringCache) getPeeringInfo(userName string) (*peeringInfo, bool) {
	pi, found := pc.peeringInfo.Load(userName)
	if !found {
		return nil, found
	}
	return pi.(*peeringInfo), found
}

func (pi *peeringInfo) getFreeQuota() corev1.ResourceList {
	freeQuota := corev1.ResourceList{}
	for key, val := range pi.totalQuota {
		tmpQuota := val.DeepCopy()
		tmpQuota.Sub(pi.usedQuota[key])
		freeQuota[key] = tmpQuota
	}
	return freeQuota
}

func (pi *peeringInfo) subUsedResources(resources corev1.ResourceList) {
	zero := resource.MustParse("0")
	for key, val := range resources {
		if prevUsed, ok := pi.usedQuota[key]; ok {
			prevUsed.Sub(val)
			// Check if there are some used quota problems. It should never be Negative
			if prevUsed.Cmp(zero) == -1 {
				klog.Warningf("Cache consistency problems: peering %q used quota is less than zero", pi.userName)
			}
			pi.usedQuota[key] = prevUsed
		}
	}
}

func (pi *peeringInfo) addUsedResources(resources corev1.ResourceList) {
	for key, val := range resources {
		if prevUsed, ok := pi.usedQuota[key]; ok {
			prevUsed.Add(val)
			pi.usedQuota[key] = prevUsed
		} else {
			pi.usedQuota[key] = val.DeepCopy()
		}
	}
}

func (pi *peeringInfo) addShadowPod(spd *Description) {
	pi.shadowPods[spd.namespacedName.String()] = spd
	pi.addUsedResources(spd.quota)
}

func (pi *peeringInfo) terminateShadowPod(spd *Description) {
	spd.terminate()
	pi.shadowPods[spd.namespacedName.String()] = spd
	pi.subUsedResources(spd.quota)
}

func (pi *peeringInfo) removeShadowPod(spd *Description) {
	delete(pi.shadowPods, spd.namespacedName.String())
}

func (pi *peeringInfo) testAndUpdateCreation(ctx context.Context, c client.Client,
	sp *offloadingv1beta1.ShadowPod, limitsEnforcement offloadingv1beta1.LimitsEnforcement, dryRun bool) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	spd, err := pi.getOrCreateShadowPodDescription(ctx, c, sp, limitsEnforcement)
	if err != nil {
		return err
	}

	klog.V(5).Infof("ShadowPod resource limits %s", quotaFormatter(spd.quota))

	klog.V(5).Infof("Cluster %q total quota %s", pi.userName, quotaFormatter(pi.totalQuota))
	klog.V(5).Infof("Cluster %q used quota %s", pi.userName, quotaFormatter(pi.usedQuota))
	klog.V(5).Infof("Cluster %q free quota %s", pi.userName, quotaFormatter(pi.getFreeQuota()))

	if err := pi.checkResources(spd); err != nil {
		return err
	}
	if !dryRun {
		pi.addShadowPod(spd)
		klog.V(5).Infof("Cluster %q updated total quota %s", pi.userName, quotaFormatter(pi.totalQuota))
		klog.V(5).Infof("Cluster %q updated used quota %s", pi.userName, quotaFormatter(pi.usedQuota))
		klog.V(5).Infof("Cluster %q updated free quota %s", pi.userName, quotaFormatter(pi.getFreeQuota()))
	}
	return nil
}

func (pi *peeringInfo) updateDeletion(sp *offloadingv1beta1.ShadowPod, dryRun bool) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	spd, err := pi.getShadowPodDescription(sp)
	if err != nil {
		return err
	}

	klog.V(5).Infof("ShadowPod resource limits %s", quotaFormatter(spd.quota))

	klog.V(5).Infof("Cluster %q total quota %s", pi.userName, quotaFormatter(pi.totalQuota))
	klog.V(5).Infof("Cluster %q used quota %s", pi.userName, quotaFormatter(pi.usedQuota))
	klog.V(5).Infof("Cluster %q free quota %s", pi.userName, quotaFormatter(pi.getFreeQuota()))

	if !dryRun {
		pi.terminateShadowPod(spd)
		klog.V(5).Infof("Cluster %q updated total quota %s", pi.userName, quotaFormatter(pi.totalQuota))
		klog.V(5).Infof("Cluster %q updated used quota %s", pi.userName, quotaFormatter(pi.usedQuota))
		klog.V(5).Infof("Cluster %q updated free quota %s", pi.userName, quotaFormatter(pi.getFreeQuota()))
	}

	return nil
}

func (pi *peeringInfo) checkResources(spd *Description) error {
	freePeeringQuota := pi.getFreeQuota()
	for key, val := range spd.quota {
		if freeQuota, ok := freePeeringQuota[key]; ok {
			if freeQuota.Cmp(val) < 0 {
				return fmt.Errorf("peering %s quota usage exceeded - free %s / requested %s",
					key, freeQuota.String(), val.String())
			}
		} else {
			return fmt.Errorf("%s quota limit not found for this peering", key)
		}
	}
	return nil
}

func (pi *peeringInfo) updateQuotas(newQuota corev1.ResourceList) {
	klog.V(5).Infof("Cluster %q old total quota %s", pi.userName, quotaFormatter(pi.totalQuota))
	pi.totalQuota = newQuota.DeepCopy()
	klog.V(5).Infof("Cluster %q new total quota %s", pi.userName, quotaFormatter(pi.totalQuota))
}

func generateQuotaPattern(quota corev1.ResourceList) corev1.ResourceList {
	zero := resource.MustParse("0")
	result := corev1.ResourceList{}
	for key := range quota {
		result[key] = zero.DeepCopy()
	}
	return result
}

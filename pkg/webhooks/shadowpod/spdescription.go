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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	quotav1 "k8s.io/apiserver/pkg/quota/v1"
	klog "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

// Description is a struct that contains the main informations about a shadow pod.
type Description struct {
	namespacedName    types.NamespacedName
	uid               types.UID
	quota             corev1.ResourceList
	running           bool
	creationTimestamp time.Time
}

func createShadowPodDescription(name, namespace string, uid types.UID, resources corev1.ResourceList) *Description {
	return &Description{
		namespacedName:    types.NamespacedName{Name: name, Namespace: namespace},
		uid:               uid,
		quota:             resources,
		running:           true,
		creationTimestamp: time.Now(),
	}
}

func (pi *peeringInfo) getOrCreateShadowPodDescription(ctx context.Context, c client.Client,
	sp *offloadingv1beta1.ShadowPod, limitsEnforcement offloadingv1beta1.LimitsEnforcement) (*Description, error) {
	nsname := types.NamespacedName{Name: sp.Name, Namespace: sp.Namespace}
	spQuota, err := getQuotaFromShadowPod(sp, limitsEnforcement)
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("ShadowPod %s quota %s", nsname.String(), quotaFormatter(*spQuota))
	spd, found := pi.shadowPods[nsname.String()]
	if found {
		if spd.running {
			return nil, fmt.Errorf("ShadowPod %s is already running", sp.GetName())
		}
		err := checkShadowPodExistence(ctx, c, nsname)
		if err != nil {
			return nil, err
		}
		// Removing the old ShadowPodDescription from the cache and creating a new one
		// Cache refreshing has not deleted it from cache
		pi.removeShadowPod(spd)
	}
	return createShadowPodDescription(sp.GetName(), sp.GetNamespace(), sp.GetUID(), *spQuota), nil
}

func (pi *peeringInfo) getShadowPodDescription(sp *offloadingv1beta1.ShadowPod) (*Description, error) {
	nsname := types.NamespacedName{Name: sp.Name, Namespace: sp.Namespace}
	spd, found := pi.shadowPods[nsname.String()]
	if !found {
		// If the shadow pod is not found, it means that it is already terminated and removed from the cache
		// A new deleting request should never be received for the same shadow pod
		// Anyway, in this case, an error is returned to anticipate the webhook response and avoid the cache quota usage update
		return nil, fmt.Errorf("ShadowPod %s not found (Maybe Cache problem)", sp.GetName())
	} else if spd.uid != sp.GetUID() {
		return nil, fmt.Errorf("ShadowPod %s: UID mismatch", sp.GetName())
	}
	return spd, nil
}

func (spd *Description) terminate() {
	spd.running = false
}

func checkShadowPodExistence(ctx context.Context, spvclient client.Client, namespacedName types.NamespacedName) error {
	sp := &offloadingv1beta1.ShadowPod{}
	err := spvclient.Get(ctx, namespacedName, sp)
	if err != nil {
		return client.IgnoreNotFound(err)
	}
	return fmt.Errorf("ShadowPod still exists in the system")
}

func getQuotaFromShadowPod(shadowpod *offloadingv1beta1.ShadowPod,
	limitsEnforcement offloadingv1beta1.LimitsEnforcement) (*corev1.ResourceList, error) {
	conResources := corev1.ResourceList{}
	initConResources := corev1.ResourceList{}

	// At least one container is required
	if shadowpod.Spec.Pod.Containers == nil {
		return nil, fmt.Errorf("ShadowPod %s has no containers defined", shadowpod.GetName())
	}

	// Calculating the sum of the resources of all containers
	for i := range shadowpod.Spec.Pod.Containers {
		// This flags are used to check if each container in range has CPU and Memory requests defined
		cpuFlag := false
		memoryFlag := false
		for key, value := range shadowpod.Spec.Pod.Containers[i].Resources.Requests {
			if key == corev1.ResourceCPU {
				cpuFlag = true
			}
			if key == corev1.ResourceMemory {
				memoryFlag = true
			}
			if prevResources, ok := conResources[key]; ok {
				prevResources.Add(value)
				conResources[key] = prevResources
			} else {
				conResources[key] = value.DeepCopy()
			}

			if limitsEnforcement == offloadingv1beta1.HardLimitsEnforcement {
				req := shadowpod.Spec.Pod.Containers[i].Resources.Requests[key]
				lim := shadowpod.Spec.Pod.Containers[i].Resources.Limits[key]
				if req.Cmp(lim) != 0 {
					return nil, fmt.Errorf("%s limits and requests are not equal for container %s",
						key, shadowpod.Spec.Pod.Containers[i].Name)
				}
			}
		}
		// If the container has no CPU or Memory requests defined and this kind of validation is required, an error is returned
		if limitsEnforcement == offloadingv1beta1.NoLimitsEnforcement {
			continue
		}
		if !cpuFlag || !memoryFlag {
			return nil, fmt.Errorf("CPU and/or memory requests not set for container %s", shadowpod.Spec.Pod.Containers[i].Name)
		}
	}

	// Calculating the max of each resource type between the init containers
	for i := range shadowpod.Spec.Pod.InitContainers {
		// This flags are used to check if each container in range has CPU and Memory requests defined
		cpuFlag := false
		memoryFlag := false
		for key, value := range shadowpod.Spec.Pod.InitContainers[i].Resources.Requests {
			if key == corev1.ResourceCPU {
				cpuFlag = true
			}
			if key == corev1.ResourceMemory {
				memoryFlag = true
			}
			if prevResources, ok := initConResources[key]; ok {
				if prevResources.Value() < value.Value() {
					initConResources[key] = value.DeepCopy()
				}
			} else {
				initConResources[key] = value.DeepCopy()
			}

			if limitsEnforcement == offloadingv1beta1.HardLimitsEnforcement {
				req := shadowpod.Spec.Pod.InitContainers[i].Resources.Requests[key]
				lim := shadowpod.Spec.Pod.InitContainers[i].Resources.Limits[key]
				if req.Cmp(lim) != 0 {
					return nil, fmt.Errorf("%s limits and requests are not equal for container %s",
						key, shadowpod.Spec.Pod.InitContainers[i].Name)
				}
			}
		}
		// If the init container has no CPU or Memory requests defined and this kind of validation is required, an error is returned
		if limitsEnforcement == offloadingv1beta1.NoLimitsEnforcement {
			continue
		}
		if !cpuFlag || !memoryFlag {
			return nil, fmt.Errorf("CPU and/or memory requests not set for initContainer %s",
				shadowpod.Spec.Pod.InitContainers[i].Name)
		}
	}
	result := quotav1.Max(conResources, initConResources)
	return &result, nil
}

func quotaFormatter(quota corev1.ResourceList) string {
	return fmt.Sprintf("[ cpu: %v, memory %v, storage: %v, ephemeral-storage: %v ]",
		quota.Cpu(), quota.Memory(), quota.Storage(), quota.StorageEphemeral())
}

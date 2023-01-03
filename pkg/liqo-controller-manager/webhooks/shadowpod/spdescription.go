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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	quotav1 "k8s.io/apiserver/pkg/quota/v1"
	klog "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
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

func (pi *peeringInfo) getOrCreateShadowPodDescription(ctx context.Context, c client.Client, sp *vkv1alpha1.ShadowPod) (*Description, error) {
	nsname := types.NamespacedName{Name: sp.Name, Namespace: sp.Namespace}
	spQuota, err := getQuotaFromShadowPod(sp, true)
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

func (pi *peeringInfo) getShadowPodDescription(sp *vkv1alpha1.ShadowPod) (*Description, error) {
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
	sp := &vkv1alpha1.ShadowPod{}
	err := spvclient.Get(ctx, namespacedName, sp)
	if err != nil {
		return client.IgnoreNotFound(err)
	}
	return fmt.Errorf("ShadowPod still exists in the system")
}

func getQuotaFromShadowPod(shadowpod *vkv1alpha1.ShadowPod, validate bool) (*corev1.ResourceList, error) {
	conResources := corev1.ResourceList{}
	initConResources := corev1.ResourceList{}

	// At least one container is required
	if shadowpod.Spec.Pod.Containers == nil {
		return nil, fmt.Errorf("ShadowPod %s has no containers defined", shadowpod.GetName())
	}

	// Calculating the sum of the resources of all containers
	for i := range shadowpod.Spec.Pod.Containers {
		// This flags are used to check if each container in range has CPU and Memory limits defined
		cpuFlag := false
		memoryFlag := false
		for key, value := range shadowpod.Spec.Pod.Containers[i].Resources.Limits {
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
		}
		// If the container has no CPU or Memory limits defined and this kind of validation is required, an error is returned
		if (!cpuFlag || !memoryFlag) && validate {
			return nil, fmt.Errorf("CPU and/or memory limits not set for container %s", shadowpod.Spec.Pod.Containers[i].Name)
		}
	}

	// Calculating the max of each resource type between the init containers
	for i := range shadowpod.Spec.Pod.InitContainers {
		// This flags are used to check if each container in range has CPU and Memory limits defined
		cpuFlag := false
		memoryFlag := false
		for key, value := range shadowpod.Spec.Pod.InitContainers[i].Resources.Limits {
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
		}
		// If the init container has no CPU or Memory limits defined and this kind of validation is required, an error is returned
		if (!cpuFlag || !memoryFlag) && validate {
			return nil, fmt.Errorf("CPU and/or memory limits not set for initContainer %s",
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

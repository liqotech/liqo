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

package mutate

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
)

// mutateShadowPodLabel mutates a pod object, adding the shadow pod label depending on whether it has been scheduled on a virtual node.
func mutateShadowPodLabel(ctx context.Context, c client.Client, pod *corev1.Pod) error {
	nodeName := pod.Spec.NodeName
	if nodeName == "" {
		klog.V(5).Infof("Skipping shadow pod label addition for pod %q, as not yet assigned to any node", klog.KObj(pod))
		return nil
	}

	var node corev1.Node
	if err := c.Get(ctx, types.NamespacedName{Name: nodeName}, &node); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to retrieve node %q hosting pod %q: %w", klog.KObj(&node), klog.KObj(pod), err)
		}

		// Do not raise an error in case the node is not found, as that would abort the creation of a pod assigned to a non-existing node
		klog.Warningf("Skipping shadow pod label addition for pod %q, as hosting node %q not found", klog.KObj(pod), klog.KObj(&node))
		return nil
	}

	if value, found := node.Labels[consts.TypeLabel]; !found || value != consts.TypeNode {
		klog.V(5).Infof("Skipping shadow pod label addition for pod %q, as assigned to non-liqo node %q", klog.KObj(pod), klog.KObj(&node))
		delete(pod.Labels, consts.LocalPodLabelKey)
		return nil
	}

	klog.V(5).Infof("Adding shadow pod label to pod %q, as assigned to liqo node %q", klog.KObj(pod), klog.KObj(&node))
	pod.Labels[consts.LocalPodLabelKey] = consts.LocalPodLabelValue
	return nil
}

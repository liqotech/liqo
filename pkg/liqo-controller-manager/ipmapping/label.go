// Copyright 2019-2024 The Liqo Authors
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

package ipmapping

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/remapping"
)

const (
	offloadedPodNameLabelKey      = "offloading.liqo.io/pod-name"
	offloadedPodNamespaceLabelKey = "offloading.liqo.io/pod-namespace"
)

func forgeIPLabels(pod *corev1.Pod) map[string]string {
	return map[string]string{
		remapping.IPCategoryTargetKey: remapping.IPCategoryTargetValueMapping,
		offloadedPodNameLabelKey:      pod.Name,
		offloadedPodNamespaceLabelKey: pod.Namespace,
	}
}

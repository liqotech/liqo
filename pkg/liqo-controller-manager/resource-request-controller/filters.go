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

package resourcerequestoperator

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// virtualNodesFilter filters only virtual nodes.
func virtualNodesFilter(options *metav1.ListOptions) {
	req, err := labels.NewRequirement(liqoconst.TypeLabel, selection.Equals, []string{liqoconst.TypeNode})
	if err != nil {
		return
	}
	options.LabelSelector = labels.NewSelector().Add(*req).String()
}

// noVirtualNodesFilter filters out virtual nodes.
func noVirtualNodesFilter(options *metav1.ListOptions) {
	req, err := labels.NewRequirement(liqoconst.TypeLabel, selection.NotEquals, []string{liqoconst.TypeNode})
	if err != nil {
		return
	}
	options.LabelSelector = labels.NewSelector().Add(*req).String()
}

// noShadowPodsFilter filters out shadow pods.
func noShadowPodsFilter(options *metav1.ListOptions) {
	req, err := labels.NewRequirement(liqoconst.LocalPodLabelKey, selection.NotEquals, []string{liqoconst.LocalPodLabelValue})
	if err != nil {
		return
	}
	options.LabelSelector = labels.NewSelector().Add(*req).String()
}

func isShadowPod(podToCheck *corev1.Pod) bool {
	return podToCheck.Labels[liqoconst.LocalPodLabelKey] == liqoconst.LocalPodLabelValue
}

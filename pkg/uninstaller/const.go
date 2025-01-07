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

package uninstaller

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// TickerInterval defines the check interval.
const TickerInterval = 5 * time.Second

// TickerTimeout defines the overall timeout to be waited.
const TickerTimeout = 5 * time.Minute

// ConditionsToCheck maps the number of conditions to be checked waiting for the unpeer.
const ConditionsToCheck = 1

type toCheckDeleted struct {
	gvr           schema.GroupVersionResource
	labelSelector metav1.LabelSelector
	phase         phase
}

type resultType struct {
	Resource toCheckDeleted
	Success  bool
}

type phase int

const (
	// PhaseUnpeering -> the peering is being teared down.
	PhaseUnpeering phase = iota
	// PhaseCleanup -> the final cleanup after unpeering is being performed.
	PhaseCleanup
)

var (
	toCheck = []toCheckDeleted{
		{
			gvr: corev1.SchemeGroupVersion.WithResource("nodes"),
			labelSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					consts.TypeLabel: consts.TypeNode,
				},
			},
			phase: PhaseUnpeering,
		},
		{
			gvr:           liqov1beta1.ForeignClusterGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
			phase:         PhaseCleanup,
		},
		{
			gvr:           offloadingv1beta1.NamespaceOffloadingGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
			phase:         PhaseCleanup,
		},
		{
			gvr:           networkingv1beta1.InternalNodeGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
			phase:         PhaseCleanup,
		},
		{
			gvr:           ipamv1alpha1.NetworkGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
			phase:         PhaseCleanup,
		},
		{
			gvr:           ipamv1alpha1.IPGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
			phase:         PhaseCleanup,
		},
	}
)

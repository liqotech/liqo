// Copyright 2019-2026 The Liqo Authors
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

package custom

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
)

// NewGVRReflector builds a Reflector for the given GroupVersionResource, either namespaced
// or cluster-scoped depending on the scope of the corresponding CRD (detected at provider
// startup through API discovery). Spec is synced local→remote, while status is synced remote→local
// only when statusSyncEnabled is true (i.e. the statusSync option is set and the CRD exposes
// the status subresource, verified at provider startup through API discovery).
// The default reflection type is AllowList when not specified.
func NewGVRReflector(gvr schema.GroupVersionResource, reflectorConfig *offloadingv1beta1.ReflectorConfig,
	statusSyncEnabled, namespaced bool) manager.Reflector {
	reflectionType := reflectorConfig.Type
	if reflectionType == "" {
		reflectionType = offloadingv1beta1.AllowList
	}

	name := fmt.Sprintf("CustomResource[%s]", gvr.String())
	if namespaced {
		return generic.NewReflector(name,
			NewNamespacedGVRReflector(gvr, statusSyncEnabled),
			generic.WithoutFallback(),
			reflectorConfig.NumWorkers,
			reflectionType,
			generic.ConcurrencyModeLeader)
	}

	// Cluster-scoped reflection is not associated with any namespace pair: a single
	// reflector instance is registered at startup, for the lifetime of the virtual-kubelet.
	return generic.NewClusterReflector(name,
		NewClusterGVRReflector(gvr, statusSyncEnabled),
		reflectorConfig.NumWorkers,
		reflectionType,
		generic.ConcurrencyModeLeader)
}

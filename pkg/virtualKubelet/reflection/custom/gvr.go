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

// NewGVRReflector builds a Reflector for the given GroupVersionResource.
// Spec is synced local→remote and status remote→local (OwnershipShared).
// The default reflection type is AllowList when not specified.
func NewGVRReflector(gvr schema.GroupVersionResource, reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	reflectionType := reflectorConfig.Type
	if reflectionType == "" {
		reflectionType = offloadingv1beta1.AllowList
	}

	name := fmt.Sprintf("CustomResource[%s]", gvr.String())
	return generic.NewReflector(name,
		NewNamespacedGVRReflector(gvr),
		generic.WithoutFallback(),
		reflectorConfig.NumWorkers,
		reflectionType,
		generic.ConcurrencyModeLeader)
}

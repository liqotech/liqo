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

package forge

import (
	"k8s.io/apimachinery/pkg/api/resource"

	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// VirtualKubeletOpts defines the custom options associated with the virtual kubelet deployment forging.
type VirtualKubeletOpts struct {
	// ContainerImage contains the virtual kubelet image name and tag.
	ContainerImage string
	// InitContainerImage contains the virtual kubelet init-container image name and tag.
	InitContainerImage string
	// DisableCertGeneration allows to disable the virtual kubelet certificate generation by means
	// of the init container (used for logs/exec capabilities).
	DisableCertGeneration bool
	ExtraAnnotations      map[string]string
	ExtraLabels           map[string]string
	ExtraArgs             []string
	NodeExtraAnnotations  argsutils.StringMap
	NodeExtraLabels       argsutils.StringMap
	RequestsCPU           resource.Quantity
	LimitsCPU             resource.Quantity
	RequestsRAM           resource.Quantity
	LimitsRAM             resource.Quantity
}

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

package kind

import (
	"context"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install kind command.
type Options struct {
	*kubeadm.Options
}

// New initializes a new Provider object.
func New(o *install.Options) install.Provider {
	return &Options{Options: &kubeadm.Options{Options: o}}
}

// Name returns the name of the provider.
func (o *Options) Name() string { return "kind" }

// Examples returns the examples string for the given provider.
func (o *Options) Examples() string {
	return `Examples:
  $ {{ .Executable }} install kind --cluster-labels region=europe,environment=staging
`
}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(ctx context.Context) error {
	// Disable the defaulting to the kubeconfig value and the sanity check, as using a localhost address.
	o.DisableAPIServerSanityChecks = true
	o.DisableAPIServerDefaulting = true

	return o.Options.Initialize(ctx)
}

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{
		"networking": map[string]interface{}{
			"fabric": map[string]interface{}{
				"config": map[string]interface{}{
					"gatewayMasqueradeBypass": true,
				},
			},
		},
	}
}

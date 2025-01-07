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

package k3s

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install k3s command.
type Options struct {
	*install.Options
}

// New initializes a new Provider object.
func New(o *install.Options) install.Provider {
	return &Options{Options: o}
}

// Name returns the name of the provider.
func (o *Options) Name() string { return "k3s" }

// Examples returns the examples string for the given provider.
func (o *Options) Examples() string {
	return `Examples:
  $ {{ .Executable }} install k3s --api-server-url https://liqo.example.local:6443 \
      --cluster-labels region=europe,environment=staging \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
or
  $ {{ .Executable }} install k3s --api-server-url https://liqo.example.local:6443 \
      --cluster-labels region=europe,environment=staging \
      --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
`
}

// RegisterFlags registers the flags for the given provider.
func (o *Options) RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.APIServer, "api-server-url", "", "The Kubernetes API Server URL (defaults to the one specified in the kubeconfig)")
	cmd.Flags().StringVar(&o.PodCIDR, "pod-cidr", "10.42.0.0/16", "The Pod CIDR of the cluster")
	cmd.Flags().StringVar(&o.ServiceCIDR, "service-cidr", "10.43.0.0/16", "The Service CIDR of the cluster")
}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(_ context.Context) error {
	// Typically, the URL refers to a localhost address.
	o.DisableAPIServerSanityChecks = true
	return nil
}

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{
		"networking": map[string]interface{}{
			"fabric": map[string]interface{}{
				"config": map[string]interface{}{
					"nftablesMonitor": false,
				},
			},
		},
	}
}

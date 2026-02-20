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

package rke2

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install rke2 command.
type Options struct {
	*install.Options
}

// New initializes a new Provider object.
func New(o *install.Options) install.Provider {
	return &Options{Options: o}
}

// Name returns the name of the provider.
func (o *Options) Name() string { return "rke2" }

// Examples returns the examples string for the given provider.
func (o *Options) Examples() string {
	return `Examples:
  $ {{ .Executable }} install rke2 --api-server-url https://liqo.example.local:9345 \
      --cluster-labels region=us-west,environment=production \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
or
  $ {{ .Executable }} install rke2 --api-server-url https://liqo.example.local:9345 \
      --cluster-labels region=us-west,environment=production \
      --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
or (with out-of-band peering for restricted networks)
  $ {{ .Executable }} install rke2 --api-server-url https://liqo.example.local:9345 \
      --cluster-id my-rke2-cluster \
      --cluster-labels region=us-west,environment=production
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
	// RKE2 API server typically runs on port 9345 and may use localhost addresses.
	// Disable API server sanity checks to support these scenarios.
	o.DisableAPIServerSanityChecks = true
	return nil
}

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{
		"networking": map[string]interface{}{
			"fabric": map[string]interface{}{
				"config": map[string]interface{}{
					// RKE2 uses nftables by default, but monitoring can cause issues
					// in some environments, similar to K3s
					"nftablesMonitor": false,
				},
			},
		},
	}
}

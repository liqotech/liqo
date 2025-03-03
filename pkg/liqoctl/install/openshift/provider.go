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

package openshift

import (
	"context"
	"fmt"

	configv1api "github.com/openshift/api/config/v1"
	configv1 "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install openshift command.
type Options struct {
	*install.Options
}

// New initializes a new Provider object.
func New(o *install.Options) install.Provider {
	return &Options{Options: o}
}

// Name returns the name of the provider.
func (o *Options) Name() string { return "openshift" }

// Examples returns the examples string for the given provider.
func (o *Options) Examples() string {
	return `Examples:
  $ {{ .Executable }} install openshift --cluster-labels region=europe,environment=staging \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
`
}

// RegisterFlags registers the flags for the given provider.
func (o *Options) RegisterFlags(_ *cobra.Command) {}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(ctx context.Context) error {
	configv1client, err := configv1.NewForConfig(o.RESTConfig)
	if err != nil {
		return fmt.Errorf("unable to create OpenShift client: %w", err)
	}

	networkConfig, err := configv1client.ConfigV1().Networks().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to retrieve OpenShift network configuration: %w", err)
	}

	return o.parseNetworkConfig(networkConfig)
}

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{
		"openshiftConfig": map[string]interface{}{
			"enabled": true,
		},
		"networking": map[string]interface{}{
			"gatewayTemplates": map[string]interface{}{
				"wireguard": map[string]interface{}{
					"implementation": "userspace",
				},
			},
		},
	}
}

func (o *Options) parseNetworkConfig(networkConfig *configv1api.Network) error {
	switch len(networkConfig.Status.ClusterNetwork) {
	case 0:
		return fmt.Errorf("no cluster network found")
	case 1:
		clusterNetwork := &networkConfig.Status.ClusterNetwork[0]
		o.PodCIDR = clusterNetwork.CIDR
	default:
		return fmt.Errorf("multiple cluster networks found")
	}

	switch len(networkConfig.Status.ServiceNetwork) {
	case 0:
		return fmt.Errorf("no service network found")
	case 1:
		o.ServiceCIDR = networkConfig.Status.ServiceNetwork[0]
	default:
		return fmt.Errorf("multiple service networks found")
	}

	return nil
}

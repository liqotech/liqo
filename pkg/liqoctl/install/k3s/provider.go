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

package k3s

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

const (
	providerPrefix = "k3s"

	defaultPodCIDR     = "10.42.0.0/16"
	defaultServiceCIDR = "10.43.0.0/16"

	podCidrFlag     = "pod-cidr"
	serviceCidrFlag = "service-cidr"
	apiServerFlag   = "api-server"
)

type k3sProvider struct {
	provider.GenericProvider
}

// NewProvider initializes a new K3S provider struct.
func NewProvider() provider.InstallProviderInterface {
	return &k3sProvider{
		GenericProvider: provider.GenericProvider{
			ClusterLabels: map[string]string{
				consts.ProviderClusterLabel: providerPrefix,
			},
		},
	}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *k3sProvider) ValidateCommandArguments(flags *pflag.FlagSet) (err error) {
	return k.ValidateCommandArguments(flags)
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (k *k3sProvider) ExtractChartParameters(ctx context.Context, config *rest.Config, args *provider.CommonArguments) error {
	return k.ExtractChartParameters(ctx, config, args)
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *k3sProvider) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": k.APIServer,
	}
	values["auth"] = map[string]interface{}{
		"service": map[string]interface{}{
			"type": "NodePort",
		},
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR":     k.ServiceCIDR,
			"podCIDR":         k.PodCIDR,
			"reservedSubnets": installutils.GetInterfaceSlice(k.ReservedSubnets),
		},
	}
	values["gateway"] = map[string]interface{}{
		"service": map[string]interface{}{
			"type": "NodePort",
		},
	}
	values["discovery"] = map[string]interface{}{
		"config": map[string]interface{}{
			"clusterLabels": installutils.GetInterfaceMap(k.ClusterLabels),
			"clusterName":   k.ClusterName,
		},
	}
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(command *cobra.Command) {
	flags := command.Flags()

	flags.String(podCidrFlag, defaultPodCIDR, "The Pod CIDR for your cluster (optional)")
	flags.String(serviceCidrFlag, defaultServiceCIDR, "The Service CIDR for your cluster (optional)")
	flags.String(apiServerFlag, "", "Your cluster API Server URL (optional)")
}

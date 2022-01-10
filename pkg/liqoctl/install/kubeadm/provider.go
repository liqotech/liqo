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

package kubeadm

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// NewProvider initializes a new Kubeadm struct.
func NewProvider() provider.InstallProviderInterface {
	return &Kubeadm{
		GenericProvider: provider.GenericProvider{
			ClusterLabels: map[string]string{
				consts.ProviderClusterLabel: providerPrefix,
			},
		},
	}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *Kubeadm) ValidateCommandArguments(flags *flag.FlagSet) (err error) {
	return nil
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (k *Kubeadm) ExtractChartParameters(ctx context.Context, config *rest.Config, _ *provider.CommonArguments) error {
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Unable to create client: %s", err)
		return err
	}

	k.K8sClient = k8sClient
	k.Config = config
	k.APIServer = config.Host

	k.PodCIDR, k.ServiceCIDR, err = retrieveClusterParameters(ctx, k.K8sClient)
	if err != nil {
		return err
	}
	return nil
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *Kubeadm) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": k.APIServer,
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR":     k.ServiceCIDR,
			"podCIDR":         k.PodCIDR,
			"reservedSubnets": installutils.GetInterfaceSlice(k.ReservedSubnets),
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
}

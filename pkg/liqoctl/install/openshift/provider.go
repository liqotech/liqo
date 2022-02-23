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

package openshift

import (
	"context"
	"fmt"

	configv1api "github.com/openshift/api/config/v1"
	configv1 "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

const (
	providerPrefix = "openshift"
)

type openshiftProvider struct {
	provider.GenericProvider

	k8sClient kubernetes.Interface
	config    *rest.Config

	apiServer   string
	serviceCIDR string
	podCIDR     string
}

// NewProvider initializes a new OpenShift provider struct.
func NewProvider() provider.InstallProviderInterface {
	return &openshiftProvider{
		GenericProvider: provider.GenericProvider{
			ClusterLabels: map[string]string{
				consts.ProviderClusterLabel: providerPrefix,
			},
		},
	}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *openshiftProvider) ValidateCommandArguments(flags *flag.FlagSet) (err error) {
	return nil
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (k *openshiftProvider) ExtractChartParameters(ctx context.Context, config *rest.Config, _ *provider.CommonArguments) error {
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Unable to create client: %s", err)
		return err
	}

	k.k8sClient = k8sClient
	k.config = config
	k.apiServer = config.Host

	configv1client, err := configv1.NewForConfig(config)
	if err != nil {
		fmt.Printf("Unable to create OpenShift client: %s", err)
		return err
	}

	networkConfig, err := configv1client.ConfigV1().Networks().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}

	return k.parseNetworkConfig(networkConfig)
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *openshiftProvider) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": k.apiServer,
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR":     k.serviceCIDR,
			"podCIDR":         k.podCIDR,
			"reservedSubnets": installutils.GetInterfaceSlice(k.ReservedSubnets),
		},
	}
	values["discovery"] = map[string]interface{}{
		"config": map[string]interface{}{
			"clusterLabels": installutils.GetInterfaceMap(k.ClusterLabels),
			"clusterName":   k.ClusterName,
		},
	}
	values["route"] = map[string]interface{}{
		"pod": map[string]interface{}{
			"extraArgs": []interface{}{
				"--route.vxlan-vtep-port=5050",
			},
		},
	}
	values["openshiftConfig"] = map[string]interface{}{
		"enable": true,
	}
}

func (k *openshiftProvider) parseNetworkConfig(networkConfig *configv1api.Network) error {
	switch len(networkConfig.Status.ClusterNetwork) {
	case 0:
		return fmt.Errorf("no cluster network found")
	case 1:
		clusterNetwork := &networkConfig.Status.ClusterNetwork[0]
		k.podCIDR = clusterNetwork.CIDR
	default:
		return fmt.Errorf("multiple cluster networks found")
	}

	switch len(networkConfig.Status.ServiceNetwork) {
	case 0:
		return fmt.Errorf("no service network found")
	case 1:
		k.serviceCIDR = networkConfig.Status.ServiceNetwork[0]
	default:
		return fmt.Errorf("multiple service networks found")
	}

	return nil
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(command *cobra.Command) {
}

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

package kind

import (
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// NewProvider initializes a new Kind struct.
func NewProvider() provider.InstallProviderInterface {
	return &Kind{
		Kubeadm: kubeadm.Kubeadm{
			GenericProvider: provider.GenericProvider{
				ClusterLabels: map[string]string{
					consts.ProviderClusterLabel: providerPrefix,
				},
			},
		},
	}
}

// UpdateChartValues patches the values map with the values required for the selected cluster. Differently from the
// kubeadm provider, this provider overrides just this methods to modify the resulting values.
func (k *Kind) UpdateChartValues(values map[string]interface{}) {
	values["auth"] = map[string]interface{}{
		"service": map[string]interface{}{
			"type": "NodePort",
		},
		"config": map[string]interface{}{
			"enableAuthentication": false,
		},
	}
	values["gateway"] = map[string]interface{}{
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
	if k.LanDiscovery == nil {
		lanDiscovery := true
		k.LanDiscovery = &lanDiscovery
	}
	values["discovery"] = map[string]interface{}{
		"config": map[string]interface{}{
			"clusterLabels":       installutils.GetInterfaceMap(k.ClusterLabels),
			"clusterName":         k.ClusterName,
			"enableAdvertisement": *k.LanDiscovery,
			"enableDiscovery":     *k.LanDiscovery,
		},
	}
}

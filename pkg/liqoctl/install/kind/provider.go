package kind

import (
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
)

// NewProvider initializes a new Kind struct.
func NewProvider() provider.InstallProviderInterface {
	return &Kind{}
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
			"serviceCIDR": k.ServiceCIDR,
			"podCIDR":     k.PodCIDR,
		},
	}
}

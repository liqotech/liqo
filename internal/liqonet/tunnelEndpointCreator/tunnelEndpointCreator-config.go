package tunnelEndpointCreator

import (
	"os"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

func (tec *TunnelEndpointCreator) setNetParameters(config *configv1alpha1.ClusterConfig) {
	podCIDR := config.Spec.LiqonetConfig.PodCIDR
	serviceCIDR := config.Spec.LiqonetConfig.ServiceCIDR
	if tec.PodCIDR != podCIDR {
		if err := tec.IPManager.SetPodCIDR(podCIDR); err != nil {
			klog.Error(err)
		}
		klog.Infof("PodCIDR set to %s", podCIDR)
		tec.PodCIDR = podCIDR
	}
	if tec.ServiceCIDR != serviceCIDR {
		if err := tec.IPManager.SetServiceCIDR(serviceCIDR); err != nil {
			klog.Error(err)
		}
		klog.Infof("ServiceCIDR set to %s", serviceCIDR)
		tec.ServiceCIDR = serviceCIDR
	}
	externalCIDR, err := tec.IPManager.GetExternalCIDR(liqonetutils.GetMask(podCIDR))
	if err != nil {
		klog.Error(err)
	}
	if tec.ExternalCIDR != externalCIDR {
		klog.Infof("ExternalCIDR set to %s", externalCIDR)
		tec.ExternalCIDR = externalCIDR
	}
}

// Helper func that returns a true if the subnet slice passed as first parameter
// contains the subnet passed as second parameter. Otherwise it returns false.
func (tec *TunnelEndpointCreator) subnetSliceContains(subnetSlice []string, network string) bool {
	return slice.ContainsString(subnetSlice, network)
}

// Helper func that removes a subnet from the configuration file.
func (tec *TunnelEndpointCreator) removeReservedSubnet(network string) {
	tec.ReservedSubnets = slice.RemoveString(tec.ReservedSubnets, network)
}

// Helper func that adds a subnet from the configuration file.
func (tec *TunnelEndpointCreator) addReservedSubnet(network string) {
	tec.ReservedSubnets = append(tec.ReservedSubnets, network)
}

// Helper func that removes a network pool from the configuration file.
func (tec *TunnelEndpointCreator) removeNetworkPool(network string) {
	tec.AdditionalPools = slice.RemoveString(tec.AdditionalPools, network)
}

// Helper func that adds a network pool from the configuration file.
func (tec *TunnelEndpointCreator) addNetworkPool(network string) {
	tec.AdditionalPools = append(tec.AdditionalPools, network)
}

// getDifferences returns a boolean telling if the 2 slices received as parameters differs and eventually it returns
// a slice containing networks not present in the 1st slice and present in the 2nd and another one containing
// networks present in the 1st and not present in the 2nd.
func (tec *TunnelEndpointCreator) getDifferences(currentConfig, newConfig []string) (addedSubnets,
	removedSubnets []string,
	differs bool) {
	//If the configuration is the same return
	if reflect.DeepEqual(currentConfig, newConfig) {
		return
	}

	addedSubnets = make([]string, 0)
	removedSubnets = make([]string, 0)
	differs = true

	for _, network := range newConfig {
		if contained := tec.subnetSliceContains(currentConfig, network); !contained {
			addedSubnets = append(addedSubnets, network)
		}
	}

	for _, network := range currentConfig {
		if contained := tec.subnetSliceContains(newConfig, network); !contained {
			removedSubnets = append(removedSubnets, network)
		}
	}
	return
}

func (tec *TunnelEndpointCreator) updateReservedSubnets(reservedSubnets []string) error {
	addedSubnets, removedSubnets, differs := tec.getDifferences(tec.ReservedSubnets, reservedSubnets)

	if !differs {
		return nil
	}

	//here we start to remove subnets from the reserved slice
	if len(removedSubnets) > 0 {
		for _, subnet := range removedSubnets {
			//free subnet in ipam
			klog.Infof("Freeing reserved subnet %s", subnet)
			if err := tec.IPManager.FreeReservedSubnet(subnet); err != nil {
				klog.Error(err)
			}
			//remove the subnet from the reserved ones
			tec.removeReservedSubnet(subnet)
		}
	}
	if len(addedSubnets) > 0 {
		for _, subnet := range addedSubnets {
			klog.Infof("Reserving subnet %s", subnet)
			if err := tec.IPManager.AcquireReservedSubnet(subnet); err != nil {
				klog.Error(err)
			}
			tec.addReservedSubnet(subnet)
		}
	}
	return nil
}

func (tec *TunnelEndpointCreator) updatePools(additionalPools []string) error {
	addedSubnets, removedSubnets, differs := tec.getDifferences(tec.AdditionalPools, additionalPools)

	if !differs {
		return nil
	}

	//here we start to remove network pools from ipam configuration
	if len(removedSubnets) > 0 {
		for _, subnet := range removedSubnets {
			//remove network pool in ipam
			klog.Infof("Removing network pool %s", subnet)
			if err := tec.IPManager.RemoveNetworkPool(subnet); err != nil {
				return err
			}
			//remove the subnet from the reserved ones
			tec.removeNetworkPool(subnet)
		}
	}
	if len(addedSubnets) > 0 {
		for _, subnet := range addedSubnets {
			klog.Infof("Adding network pool %s", subnet)
			if err := tec.IPManager.AddNetworkPool(subnet); err != nil {
				return err
			}
			tec.addNetworkPool(subnet)
		}
	}
	return nil
}

func (tec *TunnelEndpointCreator) getReservedSubnets(config *configv1alpha1.ClusterConfig) []string {
	reservedSubnets := make([]string, 0)
	liqonetConfig := config.Spec.LiqonetConfig
	// Cast CIDR to normal string and append
	for _, cidr := range liqonetConfig.ReservedSubnets {
		reservedSubnets = append(reservedSubnets, string(cidr))
	}
	return reservedSubnets
}

func (tec *TunnelEndpointCreator) getAdditionalPools(config *configv1alpha1.ClusterConfig) []string {
	additionalPools := make([]string, 0)
	liqonetConfig := config.Spec.LiqonetConfig
	// Cast CIDR to normal string and append
	for _, cidr := range liqonetConfig.AdditionalPools {
		additionalPools = append(additionalPools, string(cidr))
	}
	return additionalPools
}

// WatchConfiguration start watcher of clusterconfig resource.
func (tec *TunnelEndpointCreator) WatchConfiguration(config *rest.Config, gv *schema.GroupVersion) {
	config.ContentConfig.GroupVersion = gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	CRDclient, err := crdclient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	go utils.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		reservedSubnets := tec.getReservedSubnets(configuration)
		additionalPools := tec.getAdditionalPools(configuration)
		tec.setNetParameters(configuration)
		err = tec.updateReservedSubnets(reservedSubnets)
		if err != nil {
			klog.Error(err)
			return
		}
		err = tec.updatePools(additionalPools)
		if err != nil {
			klog.Error(err)
			return
		}
		if !tec.cfgConfigured {
			tec.WaitConfig.Done()
			klog.Infof("called done on waitgroup")
			tec.cfgConfigured = true
		}
	}, CRDclient, "")
}

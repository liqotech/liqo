package tunnelEndpointCreator

import (
	"os"
	"reflect"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterConfig"
	"github.com/liqotech/liqo/pkg/crdClient"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/slice"
)

func (tec *TunnelEndpointCreator) SetNetParameters(config *configv1alpha1.ClusterConfig) {
	podCIDR := config.Spec.LiqonetConfig.PodCIDR
	serviceCIDR := config.Spec.LiqonetConfig.ServiceCIDR
	if tec.PodCIDR != podCIDR {
		klog.Infof("setting podCIDR to %s", podCIDR)
		tec.PodCIDR = podCIDR
	}
	if tec.ServiceCIDR != serviceCIDR {
		klog.Infof("setting serviceCIDR to %s", serviceCIDR)
		tec.ServiceCIDR = serviceCIDR
	}
}

// Helper func that returns a true if the subnet slice passed as first parameter contains the subnet passed as second parameter. Otherwise it returns false
func (tec *TunnelEndpointCreator) subnetSliceContains(subnetSlice []string, network string) bool {
	return slice.ContainsString(subnetSlice, network, nil)
}

// Helper func that removes a subnet from the configuration file
func (tec *TunnelEndpointCreator) removeSubnetFromConfig(network string) {
	tec.ReservedSubnets = slice.RemoveString(tec.ReservedSubnets, network, nil)
}

// Helper func that adds a subnet from the configuration file
func (tec *TunnelEndpointCreator) addSubnetFromConfig(network string) {
	tec.ReservedSubnets = append(tec.ReservedSubnets, network)
}

func (tec *TunnelEndpointCreator) UpdateConfiguration(reservedSubnets []string) error {
	addedSubnets := make([]string, 0)
	removedSubnets := make([]string, 0)
	//If the configuration is the same return
	if reflect.DeepEqual(reservedSubnets, tec.ReservedSubnets) {
		//klog.Infof("no changes were made at the configuration")
		return nil
	}
	//save the newly added subnets in the configuration
	for _, network := range reservedSubnets {
		if contained := tec.subnetSliceContains(tec.ReservedSubnets, network); !contained {
			addedSubnets = append(addedSubnets, network)
			klog.Infof("New subnet %s to be reserved is added to the configuration file", network)
		}
	}
	//save the removed subnets from the configuration
	for _, network := range tec.ReservedSubnets {
		if contained := tec.subnetSliceContains(reservedSubnets, network); !contained {
			removedSubnets = append(removedSubnets, network)
			klog.Infof("Subnet %s is removed from the configuration file", network)
		}
	}

	//here we start to remove subnets from the reserved slice
	if len(removedSubnets) > 0 {
		for _, subnet := range removedSubnets {
			//free subnet in ipam
			klog.Infof("Freeing reserved subnet %s", subnet)
			if err := tec.IPManager.FreeReservedSubnet(subnet); err != nil {
				return err
			}
			//remove the subnet from the reserved ones
			tec.removeSubnetFromConfig(subnet)
		}
	}
	if len(addedSubnets) > 0 {
		for _, subnet := range addedSubnets {
			klog.Infof("Reserving subnet %s", subnet)
			if err := tec.IPManager.AcquireReservedSubnet(subnet); err != nil {
				return err
			}
			tec.addSubnetFromConfig(subnet)
		}
	}
	return nil
}

func (tec *TunnelEndpointCreator) GetConfiguration(config *configv1alpha1.ClusterConfig) ([]string, error) {
	reservedSubnets := make([]string, 0)
	liqonetConfig := config.Spec.LiqonetConfig
	reservedSubnets = append(reservedSubnets, liqonetConfig.PodCIDR)
	reservedSubnets = append(reservedSubnets, liqonetConfig.ServiceCIDR)
	return reservedSubnets, nil
}

func (tec *TunnelEndpointCreator) WatchConfiguration(config *rest.Config, gv *schema.GroupVersion) {
	config.ContentConfig.GroupVersion = gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	CRDclient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	go clusterConfig.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		reservedSubnets, err := tec.GetConfiguration(configuration)
		if err != nil {
			klog.Error(err)
			return
		}
		err = tec.UpdateConfiguration(reservedSubnets)
		if err != nil {
			klog.Error(err)
			return
		}
		tec.SetNetParameters(configuration)
		if !tec.cfgConfigured {
			tec.WaitConfig.Done()
			klog.Infof("called done on waitgroup")
			tec.cfgConfigured = true
		}
		/*if !tec.RunningWatchers {
			tec.ForeignClusterStartWatcher <- true
		}*/

	}, CRDclient, "")
}

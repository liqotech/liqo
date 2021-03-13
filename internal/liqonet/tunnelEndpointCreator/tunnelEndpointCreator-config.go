package tunnelEndpointCreator

import (
	"context"
	"fmt"
	"os"
	"reflect"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterConfig"
	"github.com/liqotech/liqo/pkg/crdClient"
	liqonetOperator "github.com/liqotech/liqo/pkg/liqonet"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

//it returns the subnets used by the foreign clusters
//get the list of all tunnelEndpoint CR and saves the address space assigned to the
//foreign cluster.
func (tec *TunnelEndpointCreator) GetClustersSubnets() (map[string]string, error) {
	ctx := context.Background()
	var err error
	var tunEndList netv1alpha1.TunnelEndpointList
	subnets := make(map[string]string)

	//if the error is ErrCacheNotStarted we retry until the chaches are ready
	chacheChan := make(chan struct{})
	started := tec.Manager.GetCache().WaitForCacheSync(chacheChan)
	if !started {
		return nil, fmt.Errorf("unable to sync caches")
	}

	err = tec.Client.List(ctx, &tunEndList, &client.ListOptions{})
	if err != nil {
		klog.Errorf("unable to get the list of tunnelEndpoint custom resources -> %s", err)
		return nil, err
	}
	//if the list is empty return a nil slice and nil error
	if tunEndList.Items == nil {
		return nil, nil
	}
	for _, tunEnd := range tunEndList.Items {
		if tunEnd.Status.LocalRemappedPodCIDR != "" && tunEnd.Status.LocalRemappedPodCIDR != DefaultPodCIDRValue {
			subnets[tunEnd.Spec.ClusterID] = tunEnd.Status.LocalRemappedPodCIDR
			klog.Infof("subnet %s already reserved for cluster %s", tunEnd.Status.LocalRemappedPodCIDR, tunEnd.Spec.ClusterID)
		} else if tunEnd.Status.LocalRemappedPodCIDR == DefaultPodCIDRValue {
			subnets[tunEnd.Spec.ClusterID] = tunEnd.Spec.PodCIDR
			klog.Infof("subnet %s already reserved for cluster %s", tunEnd.Spec.PodCIDR, tunEnd.Spec.ClusterID)
		}
	}
	return subnets, nil
}

func (tec *TunnelEndpointCreator) InitConfiguration(reservedSubnets []string, clusterSubnets map[string]string) error {
	if err := tec.IPManager.Init(reservedSubnets, liqonetOperator.Pools, clusterSubnets); err != nil {
		klog.Errorf("an error occurred while initializing the IP manager -> err")
		return err
	}
	tec.ReservedSubnets = reservedSubnets
	return nil
}

// Helper func that returns a true if the subnet slice passed as first parameter contains the subnet passed as second parameter. Otherwise it returns false
func (tec *TunnelEndpointCreator) subnetSliceContains(subnetSlice []string, network string) bool {
	return slice.ContainsString(subnetSlice, network, nil)
}

// Helper func that remove a subnet from the configuration file
func (tec *TunnelEndpointCreator) removeSubnetFromConfig(network string) []string {
	return slice.RemoveString(tec.ReservedSubnets, network, nil)
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
			if err := tec.IPManager.FreeReservedSubnet(subnet); err != nil {
				return err
			}
			//remove the subnet from the reserved ones
			tec.removeSubnetFromConfig(subnet)
			klog.Infof("Freeing reserved subnet %s", subnet)
		}
	}
	if len(addedSubnets) > 0 {
		for _, subnet := range addedSubnets {
			klog.Infof("New subnet %s has to be reserved", subnet)
			if err := tec.IPManager.AcquireReservedSubnet(subnet); err != nil {
				return err
			}
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
		//this section is executed at start-up time
		if !tec.IpamConfigured {
			//get subnets used by foreign clusters
			clusterSubnets, err := tec.GetClustersSubnets()
			if err != nil {
				klog.Error(err)
				return
			}
			if err := tec.InitConfiguration(reservedSubnets, clusterSubnets); err != nil {
				klog.Error(err)
				return
			}
			tec.IpamConfigured = true
		} else {
			if err := tec.UpdateConfiguration(reservedSubnets); err != nil {
				klog.Error(err)
				return
			}
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

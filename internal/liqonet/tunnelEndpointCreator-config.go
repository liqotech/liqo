package controllers

import (
	"context"
	"fmt"
	configv1alpha1 "github.com/liqoTech/liqo/api/config/v1alpha1"
	liqonetv1 "github.com/liqoTech/liqo/api/liqonet/v1"
	"github.com/liqoTech/liqo/pkg/clusterConfig"
	"github.com/liqoTech/liqo/pkg/crdClient"
	liqonetOperator "github.com/liqoTech/liqo/pkg/liqonet"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"net"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func (r *TunnelEndpointCreator) GetConfiguration(config *configv1alpha1.ClusterConfig) (map[string]*net.IPNet, error) {
	correctlyParsed := true
	reservedSubnets := make(map[string]*net.IPNet)
	liqonetConfig := config.Spec.LiqonetConfig
	//check that the reserved subnets are in the right format
	for _, subnet := range liqonetConfig.ReservedSubnets {
		_, sn, err := net.ParseCIDR(subnet)
		if err != nil {
			klog.Errorf("an error occurred while parsing configuration: %s", err)
			correctlyParsed = false
		} else {
			klog.Infof("subnet %s correctly added to the reserved subnets", sn.String())
			reservedSubnets[sn.String()] = sn
		}
	}
	if !correctlyParsed {
		return nil, fmt.Errorf("the reserved subnets list is not in the correct format")
	}
	return reservedSubnets, nil
}

//it returns the subnets used by the foreign clusters
//get the list of all tunnelEndpoint CR and saves the address space assigned to the
//foreign cluster.
func (r *TunnelEndpointCreator) GetClustersSubnets() (map[string]*net.IPNet, error) {
	ctx := context.Background()
	var err error
	var tunEndList liqonetv1.TunnelEndpointList
	subnets := make(map[string]*net.IPNet)
	//if the error is ErrCacheNotStarted we retry until the chaches are ready
	for {
		err = r.Client.List(ctx, &tunEndList, &client.ListOptions{})
		if err == nil {
			break
		} else if _, ok := err.(*cache.ErrCacheNotStarted); ok {
			klog.Infof("%s: waiting for the caches to start", err)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		klog.Errorf("unable to get the list of tunnelEndpoint custom resources -> %s", err)
		return nil, err
	}
	//if the list is empty return a nil slice and nil error
	if tunEndList.Items == nil {
		return nil, nil
	}
	for _, tunEnd := range tunEndList.Items {
		if tunEnd.Status.LocalRemappedPodCIDR != "" && tunEnd.Status.LocalRemappedPodCIDR != defualtPodCIDRValue {
			_, sn, err := net.ParseCIDR(tunEnd.Status.LocalRemappedPodCIDR)
			if err != nil {
				klog.Errorf("an error occurred while parsing the following cidr %s: %s", tunEnd.Status.LocalRemappedPodCIDR, err)
				return nil, err
			}
			subnets[sn.String()] = sn
			klog.Infof("subnet %s already reserved for cluster %s", tunEnd.Status.LocalRemappedPodCIDR, tunEnd.Spec.ClusterID)
		} else if tunEnd.Status.LocalRemappedPodCIDR == defualtPodCIDRValue {
			_, sn, err := net.ParseCIDR(tunEnd.Spec.PodCIDR)
			if err != nil {
				klog.Errorf("an error occurred while parsing the following cidr %s: %s", tunEnd.Spec.PodCIDR, err)
				return nil, err
			}
			subnets[sn.String()] = sn
			klog.Infof("subnet %s already reserved for cluster %s", tunEnd.Spec.PodCIDR, tunEnd.Spec.ClusterID)
		}
	}
	return subnets, nil
}

func (r *TunnelEndpointCreator) InitConfiguration(reservedSubnets map[string]*net.IPNet, clusterSubnets map[string]*net.IPNet) error {
	var isError = false
	//here we check that there are no conflicts between the configuration and the already used subnets
	for _, usedSubnet := range clusterSubnets {
		if liqonetOperator.VerifyNoOverlap(reservedSubnets, usedSubnet) {
			klog.Infof("there is a conflict between a reserved subnet given by the configuration and subnet used by another cluster. Please consider to remove the one of the conflicting subnets")
			isError = true
		}
	}
	//if no conflicts or errors occurred then we start the IPAM
	if !isError {
		//here we acquire the lock of the mutex
		r.Mutex.Lock()
		defer r.Mutex.Unlock()
		if err := r.IPManager.Init(); err != nil {
			klog.Errorf("an error occurred while initializing the IP manager -> err")
			return err
		}
		//here we populate the used subnets with the reserved subnets and the subnets used by clusters
		for _, value := range reservedSubnets {
			r.IPManager.UsedSubnets[value.String()] = value
		}

		for _, value := range clusterSubnets {
			r.IPManager.UsedSubnets[value.String()] = value
		}

		//we remove all the free subnets that have conflicts with the used subnets
		for _, subnet := range r.IPManager.FreeSubnets {
			if ovelaps := liqonetOperator.VerifyNoOverlap(r.IPManager.UsedSubnets, subnet); ovelaps {
				delete(r.IPManager.FreeSubnets, subnet.String())
				//we add it to a new map, if the reserved ip is removed from the config then the conflicting subnets can be inserted in the free pool of subnets
				r.IPManager.ConflictingSubnets[subnet.String()] = subnet
				klog.Infof("removing subnet %s from the free pool", subnet.String())
			}
		}
		r.IsConfigured = true
		r.ReservedSubnets = reservedSubnets
	} else {
		return fmt.Errorf("there are conflicts between the reserved subnets given in the configuration and the already used subnets in the tunnelEndpoint CRs")
	}
	return nil
}

func (r *TunnelEndpointCreator) UpdateConfiguration(reservedSubnets map[string]*net.IPNet) error {
	var addedSubnets, removedSubnets map[string]*net.IPNet
	addedSubnets = make(map[string]*net.IPNet)
	removedSubnets = make(map[string]*net.IPNet)
	//If the configuration is the same return
	if reflect.DeepEqual(reservedSubnets, r.ReservedSubnets) {
		klog.Infof("no changes were made at the configuration")
		return nil
	}
	//save the newly added subnets in the configuration
	for _, values := range reservedSubnets {
		if _, ok := r.ReservedSubnets[values.String()]; !ok {
			addedSubnets[values.String()] = values
			klog.Infof("new subnet to be reserved is added to the configuration file: %s", values.String())
		}
	}
	//save the removed subnets from the configuration
	for _, values := range r.ReservedSubnets {
		if _, ok := reservedSubnets[values.String()]; !ok {
			removedSubnets[values.String()] = values
			klog.Infof("a reserved subnet is removed from the configuration file: %s", values.String())
		}
	}
	//here we start to remove subnets from the reserved map
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	if len(removedSubnets) > 0 {
		for _, subnet := range removedSubnets {
			//remove the subnet from the used ones
			delete(r.IPManager.UsedSubnets, subnet.String())
			//remove the subnet from the reserved ones
			delete(r.ReservedSubnets, subnet.String())
			klog.Infof("removing subnet %s from the reserved list", subnet.String())
		}
		//check if there is any allocatable subnet in conflicting ones and add them to free subnets
		for _, subnet := range r.IPManager.ConflictingSubnets {
			if overlaps := liqonetOperator.VerifyNoOverlap(r.IPManager.UsedSubnets, subnet); !overlaps {
				delete(r.IPManager.ConflictingSubnets, subnet.String())
				//we add it to the allocation pool
				r.IPManager.FreeSubnets[subnet.String()] = subnet
				klog.Infof("adding subnet %s to the free pool", subnet.String())
			}
		}
	}
	if len(addedSubnets) > 0 {
		newReservedNet := false
		allocatedSubnets := make(map[string]*net.IPNet)
		//separate the allocated subnets from the reserved subnets
		for _, subnet := range r.IPManager.UsedSubnets {
			if _, ok := r.ReservedSubnets[subnet.String()]; !ok {
				allocatedSubnets[subnet.String()] = subnet
			}
		}
		for _, subnet := range addedSubnets {
			//check if the subnet which has been asked to be reserved does not have conflicts with the subnets used to remap the peering clusters
			if overlaps := liqonetOperator.VerifyNoOverlap(allocatedSubnets, subnet); !overlaps {
				r.ReservedSubnets[subnet.String()] = subnet
				r.IPManager.UsedSubnets[subnet.String()] = subnet
				newReservedNet = true
				klog.Infof("subnet correctly added to the reserved list: %s", subnet.String())
			} else {
				klog.Errorf("subnet not added to the reserved list due to conflicts with already allocated IPs: %s", subnet.String())
			}
		}
		//if a new subnet was added to the reserved list then remove all the nets in the free pool that have conflicts
		if newReservedNet {
			for _, subnet := range r.IPManager.FreeSubnets {
				if overlaps := liqonetOperator.VerifyNoOverlap(r.IPManager.UsedSubnets, subnet); overlaps {
					delete(r.IPManager.FreeSubnets, subnet.String())
					//we add it to a new map, if the reserved ip is removed from the config then the conflicting subnets can be inserted in the free pool of subnets
					r.IPManager.ConflictingSubnets[subnet.String()] = subnet
					klog.Infof("removing subnet from the free pool: %s", subnet.String())
				}
			}
		}
	}
	return nil
}

func (r *TunnelEndpointCreator) WatchConfiguration(config *rest.Config, gv *schema.GroupVersion) {
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

		//this section is executed at start-up time
		if !r.IsConfigured {
			//get the reserved subnets from che configuration CRD
			reservedSubnets, err := r.GetConfiguration(configuration)
			if err != nil {
				klog.Error(err)
				return
			}
			//get subnets used by foreign clusters
			clusterSubnets, err := r.GetClustersSubnets()
			if err != nil {
				klog.Error(err)
				return
			}
			if err := r.InitConfiguration(reservedSubnets, clusterSubnets); err != nil {
				klog.Error(err)
				return
			}
			r.Configured <- true
		} else {
			//get the reserved subnets from che configuration CRD
			reservedSubnets, err := r.GetConfiguration(configuration)
			if err != nil {
				klog.Error(err)
				return
			}
			if err := r.UpdateConfiguration(reservedSubnets); err != nil {
				klog.Error(err)
				return
			}
		}

	}, CRDclient, "")
}

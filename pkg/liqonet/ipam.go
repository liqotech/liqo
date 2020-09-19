package liqonet

import (
	"fmt"
	"github.com/apparentlymart/go-cidr/cidr"
	"k8s.io/klog"
	"net"
)

type Ipam interface {
	Init() error
	GetNewSubnetPerCluster(network *net.IPNet, clusterID string) (*net.IPNet, error)
	RemoveReservedSubnet(clusterID string)
}

type IpManager struct {
	UsedSubnets        map[string]*net.IPNet
	FreeSubnets        map[string]*net.IPNet
	ConflictingSubnets map[string]*net.IPNet
	SubnetPerCluster   map[string]*net.IPNet
}

func (ip IpManager) Init() error {
	//TODO: remove the hardcoded value of the CIDRBlock
	CIDRBlock := "10.0.0.0/16"
	//the first /16 subnet in 10/8 cidr block
	_, subnet, err := net.ParseCIDR("10.0.0.0/16")
	if err != nil {
		klog.Errorf("unable to parse the first subnet %s: %s", CIDRBlock, err)
		return err
	}
	//The first subnet /16 is added to the FreeSubnets
	ip.FreeSubnets[subnet.String()] = subnet
	//here we divide the CIDRBlock 10.0.0.0/8 in 256 /16 subnets
	for i := 0; i < 255; i++ {
		subnet, _ = cidr.NextSubnet(subnet, 16)
		ip.FreeSubnets[subnet.String()] = subnet
	}
	return nil
}

//for a given cluster it returns an error if no subnets are available
//a new subnet if the original pod Cidr of the cluster has conflicts
//the existing subnet allocated to the cluster if already called this function
//original network if no conflicts are present.
func (ip IpManager) GetNewSubnetPerCluster(network *net.IPNet, clusterID string) (*net.IPNet, error) {
	//first check if we already have assigned a subnet to the cluster
	if _, ok := ip.SubnetPerCluster[clusterID]; ok {
		return ip.SubnetPerCluster[clusterID], nil
	}
	//check if the given network has conflicts with any of the used subnets
	if flag := VerifyNoOverlap(ip.UsedSubnets, network); flag {
		//if there are conflicts then get a free subnet from the pool and return it
		//return also a "true" value for the bool
		if subnet, err := ip.getNextSubnet(); err != nil {
			return nil, err
		} else {
			ip.reserveSubnet(subnet, clusterID)
			klog.Infof("%s -> NAT enabled, remapping original subnet %s to new subnet %s", clusterID, network.String(), subnet.String())
			return subnet, nil
		}
	}
	ip.reserveSubnet(network, clusterID)
	klog.Infof("%s -> NAT not needed, using original subnet %s", clusterID, network.String())

	return network, nil
}

func (ip *IpManager) getNextSubnet() (*net.IPNet, error) {
	if len(ip.FreeSubnets) == 0 {
		return nil, fmt.Errorf("no more available subnets to allocate")
	}
	var availableSubnet *net.IPNet
	for _, subnet := range ip.FreeSubnets {
		availableSubnet = subnet
		break
	}
	return availableSubnet, nil
}

//add the network to the UsedSubnets and remove the subnets in free subnets that overlap with the network
func (ip IpManager) reserveSubnet(network *net.IPNet, clusterID string) {
	ip.UsedSubnets[network.String()] = network
	for _, net := range ip.FreeSubnets {
		if bool := VerifyNoOverlap(ip.UsedSubnets, net); bool {
			ip.ConflictingSubnets[net.String()] = net
			delete(ip.FreeSubnets, net.String())
		}
	}
	//add the very same subnet to the
	ip.SubnetPerCluster[clusterID] = network
}

func (ip IpManager) RemoveReservedSubnet(clusterID string) {
	subnet, ok := ip.SubnetPerCluster[clusterID]
	if !ok {
		return
	}
	//remove the subnet from the used ones
	delete(ip.UsedSubnets, subnet.String())
	delete(ip.SubnetPerCluster, clusterID)
	//check if there are subnets in the conflicting map that can be made available in to the free pool
	for _, net := range ip.ConflictingSubnets {
		if overlap := VerifyNoOverlap(ip.UsedSubnets, net); !overlap {
			delete(ip.ConflictingSubnets, net.String())
			ip.FreeSubnets[net.String()] = net
		}
	}
}

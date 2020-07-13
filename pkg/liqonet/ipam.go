package liqonet

import (
	"fmt"
	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/go-logr/logr"
	"net"
)

type Ipam interface {
	Init() error
	GetNewSubnetPerCluster(network *net.IPNet, clusterID string) (*net.IPNet, error)
	RemoveReservedSubnet(clusterID string)
}

type IpManager struct {
	UsedSubnets      map[string]*net.IPNet
	FreeSubnets      map[string]*net.IPNet
	SubnetPerCluster map[string]*net.IPNet
	Initialized      bool
	Log              logr.Logger
}

func (ip IpManager) Init() error {
	//TODO: remove the hardcoded value of the CIDRBlock
	CIDRBlock := "10.0.0.0/16"
	//the first /16 subnet in 10/8 cidr block
	_, subnet, err := net.ParseCIDR("10.0.0.0/16")
	if err != nil {
		ip.Log.Error(err, "unable to parse the first subnet %s :%v", CIDRBlock, err)
		return err
	}
	//first we get podCIDR and clusterCIDR
	podCIDR, err := GetClusterPodCIDR()
	if err != nil {
		ip.Log.Error(err, "unable to retrieve podCIDR from environment variable")
		return err
	}
	clusterCIDR, err := GetClusterCIDR()
	if err != nil {
		ip.Log.Error(err, "unable to retrieve clusterCIDR from environment variable")
		return err
	}
	//we parse podCIDR and clusterCIDR
	_, clusterNet, err := net.ParseCIDR(clusterCIDR)
	if err != nil {
		return fmt.Errorf("an error occured while parsing clusterCIDR %s :%v", clusterCIDR, err)
	}
	_, podNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return fmt.Errorf("an error occured while parsing podCIDR %s :%v", podCIDR, err)
	}
	//The first subnet /16 is added to the FreeSubnets
	ip.FreeSubnets[subnet.String()] = subnet
	//here we divide the CIDRBlock 10.0.0.0/8 in 256 /16 subnets
	for i := 0; i < 255; i++ {
		subnet, _ = cidr.NextSubnet(subnet, 16)
		ip.FreeSubnets[subnet.String()] = subnet
	}
	//clusterCIDR and podCIDR are added to the UsedSubnets
	ip.UsedSubnets[clusterNet.String()] = clusterNet
	ip.UsedSubnets[podNet.String()] = podNet

	//we remove all the subnets that have conflicts with the podCidr and clusterCidr from FreeSubnets
	for _, net := range ip.FreeSubnets {
		if bool := VerifyNoOverlap(ip.UsedSubnets, net); bool {
			delete(ip.FreeSubnets, net.String())
		}
	}
	return nil
}

//for a given cluster it returns an error if no subnets are available
//a new subnet if the original pod Cidr of the cluster has conflicts
//the existing subnet allocated to the cluster if already called this function
//nil for the new subnet if no conflicts are present.
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
			ip.Log.Info("Reserved: ", "subnet", subnet.String(), "for cluster", clusterID)
			return subnet, nil
		}
	}
	ip.SubnetPerCluster[clusterID] = network
	return nil, nil
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

//add the network to the UsedSubnets and remove of the subnets in free subnets that overlap with the network
func (ip IpManager) reserveSubnet(network *net.IPNet, clusterID string) {
	ip.UsedSubnets[network.String()] = network
	for _, net := range ip.FreeSubnets {
		if bool := VerifyNoOverlap(ip.UsedSubnets, net); bool {
			if _, ok := ip.UsedSubnets[net.String()]; !ok {
				ip.UsedSubnets[net.String()] = net
				delete(ip.FreeSubnets, net.String())
			} else {
				delete(ip.FreeSubnets, net.String())
			}
		}
	}
	ip.SubnetPerCluster[clusterID] = network
}

func (ip IpManager) RemoveReservedSubnet(clusterID string) {

	subnet := ip.SubnetPerCluster[clusterID]

	ip.FreeSubnets[subnet.String()] = subnet
	delete(ip.UsedSubnets, subnet.String())
	delete(ip.SubnetPerCluster, clusterID)
	ip.Log.Info("Removing", "subnet", subnet.String(), "reserved to cluster", clusterID)
}

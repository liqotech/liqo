package liqonet

import (
	"fmt"
	"strconv"
	"strings"

	goipam "github.com/metal-stack/go-ipam"
	"inet.af/netaddr"
	"k8s.io/klog"
)

/* IPAM Interface */
type Ipam interface {
	Init(reservedNetworks, networkPool []string, subnetPerCluster map[string]string) error
	GetSubnetPerCluster(network, clusterID string) (string, error)
	FreeSubnetPerCluster(clusterID string) error
	AcquireReservedSubnet(network string) error
	FreeReservedSubnet(network string) error
}

/* IPAM implementation */
type IPAM struct {
	/* Map that store the network allocated for a given remote cluster */
	SubnetPerCluster map[string]string
	/* Set of networks from which IPAM takes new networks. */
	pools []string
	ipam  goipam.Ipamer
}

/* NewIPAM returns a IPAM instance */
func NewIPAM() *IPAM {
	liqoIPAM := IPAM{
		SubnetPerCluster: make(map[string]string),
		pools:            make([]string, 0),
		ipam:             goipam.New(),
	}
	return &liqoIPAM
}

/* Constant slice containing private IPv4 networks */
var Pools = []string{
	"10.0.0.0/8",
	"192.168.0.0/16",
	"172.16.0.0/12",
}

/* Init receives a set of networks that will be marked as used, and a slice of pools from which it will allocate subnets for remote clusters */
func (liqoIPAM *IPAM) Init(reservedNetworks []string, networkPool []string, subnetPerCluster map[string]string) error {
	klog.Infof("IPAM init...")
	/* Set network pools */
	klog.Infof("Set up pool list..")
	for _, network := range networkPool {
		if _, err := liqoIPAM.ipam.NewPrefix(network); err != nil {
			return fmt.Errorf("failed to create a new prefix for network %s", network)
		}
		liqoIPAM.pools = append(liqoIPAM.pools, network)
		klog.Infof("Pool %s has been successfully added to the pool list", network)
	}
	/* Acquire reserved networks */
	klog.Infof("Acquire reserved networks..")
	for _, network := range reservedNetworks {
		if err := liqoIPAM.AcquireReservedSubnet(network); err != nil {
			return err
		}
	}
	/* Acquire all networks previously assigned to clusters */
	klog.Infof("Acquire networks assigned to clusters..")
	for cluster, network := range subnetPerCluster {
		if err := liqoIPAM.AcquireReservedSubnet(network); err != nil {
			return fmt.Errorf("cannot reserve network %s previously assigned to cluster %s: %w", network, cluster, err)
		}
		liqoIPAM.SubnetPerCluster[cluster] = network
		klog.Infof("Reserved network %s previously assigned to cluster %s", network, cluster)
	}
	return nil
}

/* AcquireReservedNetwork marks as used the network received as parameter */
func (liqoIPAM *IPAM) AcquireReservedSubnet(reservedNetwork string) error {
	klog.Infof("Request to reserve network %s has been received", reservedNetwork)
	klog.Infof("Checking if network %s overlaps with any cluster network", reservedNetwork)
	if cluster, overlaps := liqoIPAM.overlapsWithCluster(reservedNetwork); overlaps {
		return fmt.Errorf("network %s cannot be reserved because it overlaps with network %s allocated to cluster %s", reservedNetwork, liqoIPAM.SubnetPerCluster[cluster], cluster)
	}
	klog.Infof("Network %s does not overlap with any cluster network", reservedNetwork)
	klog.Infof("Checking if network %s belongs to any pool", reservedNetwork)
	pool, ok, err := liqoIPAM.getPoolFromNetwork(reservedNetwork)
	if err != nil {
		return err
	}
	if ok && reservedNetwork == pool {
		klog.Errorf("Network %s is equal to a pool", reservedNetwork)
		return fmt.Errorf("network %s is equal to a pool so it cannot be reserved", reservedNetwork)
	}
	if ok && reservedNetwork != pool {
		klog.Infof("Network %s is contained in pool %s", reservedNetwork, pool)
		if _, err := liqoIPAM.ipam.AcquireSpecificChildPrefix(pool, reservedNetwork); err != nil {
			klog.Infof("Network %s has already been reserved", reservedNetwork)
			return nil
		}
		klog.Infof("Network %s has just been reserved", reservedNetwork)
		return nil
	}
	klog.Infof("Network %s is not contained in any pool", reservedNetwork)
	if _, err := liqoIPAM.ipam.NewPrefix(reservedNetwork); err != nil {
		klog.Infof("Network %s has already been reserved", reservedNetwork)
		return nil
	}
	klog.Infof("Network %s has just been reserved.", reservedNetwork)
	return nil
}

func (liqoIPAM *IPAM) overlapsWithCluster(network string) (string, bool) {
	for cluster, clusterSubnet := range liqoIPAM.SubnetPerCluster {
		if err := liqoIPAM.ipam.PrefixesOverlapping([]string{clusterSubnet}, []string{network}); err != nil {
			//overlaps
			return cluster, true
		}
	}
	return "", false
}

/* Function that receives a network as parameter and returns the pool to which this network belongs to. The second return parameter is a boolean: it is false if the network does not belong to any pool */
func (liqoIPAM *IPAM) getPoolFromNetwork(network string) (string, bool, error) {
	var poolIPset netaddr.IPSetBuilder
	// Build IPSet for new network
	ipprefix, err := netaddr.ParseIPPrefix(network)
	if err != nil {
		return "", false, err
	}
	for _, pool := range liqoIPAM.pools {
		// Build IPSet for pool
		c, err := netaddr.ParseIPPrefix(pool)
		if err != nil {
			return "", false, err
		}
		poolIPset.AddPrefix(c)
		// Check if the pool contains network
		if poolIPset.IPSet().ContainsPrefix(ipprefix) {
			return pool, true, nil
		}
	}
	return "", false, nil
}

/* GetSubnetPerCluster tries to reserve the network received as parameter for cluster clusterID. If it cannot allocate the network itself, GetSubnetPerCluster maps it to a new network. The network returned can be the original network, or the mapped network */
func (liqoIPAM *IPAM) GetSubnetPerCluster(network, clusterID string) (string, error) {
	var mappedNetwork string
	if value, ok := liqoIPAM.SubnetPerCluster[clusterID]; ok {
		return value, nil
	}
	klog.Infof("Network %s allocation request for cluster %s", network, clusterID)
	_, err := liqoIPAM.ipam.NewPrefix(network)
	if err != nil && !strings.Contains(err.Error(), "overlaps") {
		/* Overlapping is not considered an error in this context. */
		return "", fmt.Errorf("cannot reserve network %s:%w", network, err)
	}

	if err == nil {
		klog.Infof("Network %s successfully assigned for cluster %s", network, clusterID)
		liqoIPAM.SubnetPerCluster[clusterID] = network
		return network, nil
	}
	/* Since NewPrefix failed, network belongs to a pool or it has been already reserved */
	klog.Infof("Cannot allocate network %s, checking if it belongs to any pool...", network)
	pool, ok, err := liqoIPAM.getPoolFromNetwork(network)
	if err != nil {
		return "", err
	}
	if ok && network == pool {
		klog.Infof("Network %s is equal to a pool, looking for a mapping..", network)
		mappedNetwork, ok = liqoIPAM.mapNetwork(network)
		if !ok {
			return "", fmt.Errorf("Cannot assign any network to cluster %s", clusterID)
		}
		klog.Infof("Network %s successfully mapped to network %s", mappedNetwork, network)
		klog.Infof("Network %s successfully assigned to cluster %s", mappedNetwork, clusterID)
		liqoIPAM.SubnetPerCluster[clusterID] = mappedNetwork
		return mappedNetwork, nil
	}
	if ok && network != pool {
		klog.Infof("Network %s belongs to pool %s, trying to acquire it...", network, pool)
		_, err := liqoIPAM.ipam.AcquireSpecificChildPrefix(pool, network)
		if err != nil && !strings.Contains(err.Error(), "is not available") {
			/* Uknown error, return */
			return "", fmt.Errorf("cannot acquire prefix %s from prefix %s: %w", network, pool, err)
		}
		if err == nil {
			klog.Infof("Network %s successfully assigned to cluster %s", network, clusterID)
			liqoIPAM.SubnetPerCluster[clusterID] = network
			return network, nil
		}
		/* Network is not available, need a mapping */
		klog.Infof("Cannot acquire network %s from pool %s", network, pool)
	}
	/* Network is already reserved, need a mapping */
	klog.Infof("Looking for a mapping for network %s...", network)
	mappedNetwork, ok = liqoIPAM.mapNetwork(network)
	if !ok {
		return "", fmt.Errorf("Cannot assign any network to cluster %s", clusterID)
	}
	klog.Infof("Network %s successfully mapped to network %s", mappedNetwork, network)
	klog.Infof("Network %s successfully assigned to cluster %s", mappedNetwork, clusterID)
	liqoIPAM.SubnetPerCluster[clusterID] = mappedNetwork
	return mappedNetwork, nil
}

// mapNetwork allocates a suitable (same mask length) network used to map the network received as first parameter which probably cannot be used due to some collision with existing networks
func (liqoIPAM *IPAM) mapNetwork(network string) (string, bool) {
	for _, pool := range liqoIPAM.pools {
		klog.Infof("Trying to acquire a child prefix from prefix %s (mask length=%d)", pool, getMask(network))
		if mappedNetwork, err := liqoIPAM.ipam.AcquireChildPrefix(pool, getMask(network)); err == nil {
			klog.Infof("Network %s has been mapped to network %s", network, mappedNetwork)
			return mappedNetwork.String(), true
		}
	}
	return "", false
}

/* Helper function to get a mask from a net.IPNet */
func getMask(network string) uint8 {
	stringMask := network[len(network)-2:]
	mask, _ := strconv.ParseInt(stringMask, 10, 8)
	return uint8(mask)
}

/* FreeReservedSubnet marks as free a reserved subnet */
func (liqoIPAM *IPAM) FreeReservedSubnet(network string) error {
	var p *goipam.Prefix
	if p = liqoIPAM.ipam.PrefixFrom(network); p == nil {
		return fmt.Errorf("network %s is already available", network)
	}
	//Network exists, try to release it as a child prefix
	if err := liqoIPAM.ipam.ReleaseChildPrefix(p); err != nil {
		klog.Infof("Cannot release subnet %s previously allocated from the pools", network)
		// It is not a child prefix, then it is a parent prefix, so delete it
		if _, err := liqoIPAM.ipam.DeletePrefix(network); err != nil {
			klog.Errorf("Cannot delete prefix %s", network)
			return fmt.Errorf("cannot remove subnet %s", network)
		}
	}
	return nil
}

/* FreeSubnetPerCluster marks as free the network previously allocated for cluster clusterID */
func (liqoIPAM *IPAM) FreeSubnetPerCluster(clusterID string) error {
	var subnet string
	var ok bool
	if subnet, ok = liqoIPAM.SubnetPerCluster[clusterID]; !ok {
		//Network does not exists
		return nil
	}
	if err := liqoIPAM.FreeReservedSubnet(subnet); err != nil {
		return err
	}
	delete(liqoIPAM.SubnetPerCluster, clusterID)
	return nil
}

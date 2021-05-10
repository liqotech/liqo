package liqonet

import (
	"fmt"
	"strings"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"

	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/util/slice"

	goipam "github.com/metal-stack/go-ipam"
	"inet.af/netaddr"
	"k8s.io/klog"
)

/* IPAM Interface. */
type Ipam interface {
	// GetSubnetsPerCluster stores and reserves PodCIDR and ExternalCIDR for a remote cluster.
	GetSubnetsPerCluster(podCidr, externalCIDR, clusterID string) (string, string, error)
	// FreeSubnetsPerCluster deletes and frees PodCIDR and ExternalCIDR for a remote cluster.
	FreeSubnetsPerCluster(clusterID string) error
	// AcquireReservedSubnet reserves a network
	AcquireReservedSubnet(network string) error
	// FreeReservedSubnet frees a network
	FreeReservedSubnet(network string) error
	// AddNetworkPool adds a network to the set of network pools
	AddNetworkPool(network string) error
	// RemoveNetworkPool removes a network from the set of network pools
	RemoveNetworkPool(network string) error
	// AddExternalCIDRPerCluster stores (without reserving) an ExternalCIDR for a remote cluster
	AddExternalCIDRPerCluster(network, clusterID string) error
	// RemoveExternalCIDRPerCluster deletes an ExternalCIDR for a cluster
	RemoveExternalCIDRPerCluster(clusterID string) error
	// GetClusterExternalCIDR eventually choose and returns the local cluster's ExternalCIDR
	GetClusterExternalCIDR(mask uint8) (string, error)
}

/* IPAM implementation. */
type IPAM struct {
	ipam        goipam.Ipamer
	ipamStorage IpamStorage
}

/* NewIPAM returns a IPAM instance. */
func NewIPAM() *IPAM {
	return &IPAM{}
}

/* Constant slice containing private IPv4 networks. */
var Pools = []string{
	"10.0.0.0/8",
	"192.168.0.0/16",
	"172.16.0.0/12",
}

/* Init uses the Ipam resource to retrieve and allocate reserved networks. */
func (liqoIPAM *IPAM) Init(pools []string, dynClient dynamic.Interface) error {
	var err error
	// Set up storage
	liqoIPAM.ipamStorage, err = NewIPAMStorage(dynClient)
	if err != nil {
		return fmt.Errorf("cannot set up storage for ipam:%w", err)
	}
	liqoIPAM.ipam = goipam.NewWithStorage(liqoIPAM.ipamStorage)

	// Get resource
	ipamPools, err := liqoIPAM.ipamStorage.getPools()
	if err != nil {
		return fmt.Errorf("cannot get Ipam config: %w", err)
	}

	// Have network pools been already set? If not, take them from caller
	if len(ipamPools) == 0 {
		for _, network := range pools {
			if _, err := liqoIPAM.ipam.NewPrefix(network); err != nil {
				return fmt.Errorf("failed to create a new prefix for network %s", network)
			}
			ipamPools = append(ipamPools, network)
			klog.Infof("Pool %s has been successfully added to the pool list", network)
		}
		err = liqoIPAM.ipamStorage.updatePools(ipamPools)
		if err != nil {
			return fmt.Errorf("cannot set pools: %w", err)
		}
	}
	return nil
}

// reservePoolInHalves handles the special case in which a network pool has to be entirely reserved
// Since AcquireSpecificChildPrefix would return an error, reservePoolInHalves acquires the two
// halves of the network pool.
func (liqoIPAM *IPAM) reservePoolInHalves(pool string) error {
	klog.Infof("Network %s is equal to a network pool, acquiring first half..", pool)
	mask := GetMask(pool)
	mask++
	_, err := liqoIPAM.ipam.AcquireChildPrefix(pool, mask)
	if err != nil {
		return fmt.Errorf("cannot acquire first half of pool %s", pool)
	}
	klog.Infof("Acquiring second half..")
	_, err = liqoIPAM.ipam.AcquireChildPrefix(pool, mask)
	if err != nil {
		return fmt.Errorf("cannot acquire second half of pool %s", pool)
	}
	klog.Infof("Network %s has successfully been reserved", pool)
	return nil
}

/* AcquireReservedNetwork marks as used the network received as parameter. */
func (liqoIPAM *IPAM) AcquireReservedSubnet(reservedNetwork string) error {
	klog.Infof("Request to reserve network %s has been received", reservedNetwork)
	cluster, overlaps, err := liqoIPAM.overlapsWithCluster(reservedNetwork)
	if err != nil {
		return fmt.Errorf("cannot acquire network %s:%w", reservedNetwork, err)
	}
	if overlaps {
		return fmt.Errorf("network %s cannot be reserved because it overlaps with network of cluster %s",
			reservedNetwork, cluster)
	}
	pool, ok, err := liqoIPAM.getPoolFromNetwork(reservedNetwork)
	if err != nil {
		return err
	}
	if ok && reservedNetwork == pool {
		return liqoIPAM.reservePoolInHalves(pool)
	}
	if ok && reservedNetwork != pool {
		if _, err := liqoIPAM.ipam.AcquireSpecificChildPrefix(pool, reservedNetwork); err != nil {
			return fmt.Errorf("cannot reserve network %s:%w", reservedNetwork, err)
		}
		klog.Infof("Network %s has successfully been reserved", reservedNetwork)
		return nil
	}
	if _, err := liqoIPAM.ipam.NewPrefix(reservedNetwork); err != nil {
		return fmt.Errorf("cannot reserve network %s:%w", reservedNetwork, err)
	}
	klog.Infof("Network %s has successfully been reserved.", reservedNetwork)
	return nil
}

func (liqoIPAM *IPAM) overlapsWithNetwork(newNetwork, network string) (overlaps bool, err error) {
	if network == "" {
		return
	}
	if err = liqoIPAM.ipam.PrefixesOverlapping([]string{network}, []string{newNetwork}); err != nil {
		//overlaps
		overlaps = true
		err = nil
		return
	}
	return
}

func (liqoIPAM *IPAM) overlapsWithCluster(network string) (overlappingCluster string, overlaps bool, err error) {
	// Get cluster subnets
	clusterSubnets, err := liqoIPAM.ipamStorage.getClusterSubnets()
	if err != nil {
		err = fmt.Errorf("cannot get Ipam config: %w", err)
		return
	}
	for cluster, clusterSubnet := range clusterSubnets {
		overlaps, err = liqoIPAM.overlapsWithNetwork(network, clusterSubnet.PodCIDR)
		if err != nil {
			return
		}
		if overlaps {
			overlappingCluster = cluster
			return
		}
		overlaps, err = liqoIPAM.overlapsWithNetwork(network, clusterSubnet.LocalExternalCIDR)
		if err != nil {
			return
		}
		if overlaps {
			overlappingCluster = cluster
			return
		}
		overlaps, err = liqoIPAM.overlapsWithNetwork(network, clusterSubnet.RemoteExternalCIDR)
		if err != nil {
			return
		}
		if overlaps {
			overlappingCluster = cluster
			return
		}
	}
	return overlappingCluster, overlaps, err
}

func (liqoIPAM *IPAM) overlapsWithPool(network string) (overlappingPool string, overlaps bool, err error) {
	// Get resource
	pools, err := liqoIPAM.ipamStorage.getPools()
	if err != nil {
		err = fmt.Errorf("cannot get Ipam config: %w", err)
		return
	}
	for _, pool := range pools {
		overlaps, err = liqoIPAM.overlapsWithNetwork(network, pool)
		if err != nil {
			return
		}
		if overlaps {
			overlappingPool = pool
			return
		}
	}
	return
}

/* Function that receives a network as parameter and returns the pool to which this network belongs to. */
func (liqoIPAM *IPAM) getPoolFromNetwork(network string) (networkPool string, success bool, err error) {
	var poolIPset netaddr.IPSetBuilder
	var c netaddr.IPPrefix
	// Get resource
	pools, err := liqoIPAM.ipamStorage.getPools()
	if err != nil {
		err = fmt.Errorf("cannot get Ipam config: %w", err)
		return
	}
	// Build IPSet for new network
	ipprefix, err := netaddr.ParseIPPrefix(network)
	if err != nil {
		return
	}
	for _, pool := range pools {
		// Build IPSet for pool
		c, err = netaddr.ParseIPPrefix(pool)
		if err != nil {
			return
		}
		poolIPset.AddPrefix(c)
		// Check if the pool contains network
		if poolIPset.IPSet().ContainsPrefix(ipprefix) {
			networkPool = pool
			success = true
			return
		}
	}
	return
}

func (liqoIPAM *IPAM) clusterSubnetEqualToPool(pool string) (string, error) {
	klog.Infof("Network %s is equal to a pool, looking for a mapping..", pool)
	mappedNetwork, err := liqoIPAM.getNetworkFromPool(GetMask(pool))
	if err != nil {
		klog.Infof("Mapping not found, acquiring the entire network pool..")
		err = liqoIPAM.reservePoolInHalves(pool)
		if err != nil {
			return "", fmt.Errorf("no networks available")
		}
		return pool, nil
	}
	return mappedNetwork, nil
}

// getOrRemapNetwork first tries to acquire the received network.
// If conflicts are found, a new mapped network is returned.
func (liqoIPAM *IPAM) getOrRemapNetwork(network string) (string, error) {
	var mappedNetwork string
	klog.Infof("Allocating network %s", network)
	// First try to get a new Prefix
	_, err := liqoIPAM.ipam.NewPrefix(network)

	if err != nil && !strings.Contains(err.Error(), "overlaps") {
		// Return if get an error that is not an overlapping error
		return "", fmt.Errorf("cannot reserve network %s:%w", network, err)
	}
	if err == nil {
		// New Prefix succeeded, return
		return network, nil
	}
	// NewPrefix failed, network overlaps with a network pool or with a reserved network
	pool, ok, err := liqoIPAM.getPoolFromNetwork(network)
	if err != nil {
		return "", err
	}
	if ok && network == pool {
		/* getPodCidr could behave as AcquireReservedSubnet does in this condition, but in this case
		is better to look first for a mapping rather than acquire the entire network pool.
		Consider the impact of having a network pool n completely filled and multiple clusters asking for
		networks in n. This would create the necessity of nat-ting the traffic towards these clusters. */
		mappedNetwork, err = liqoIPAM.clusterSubnetEqualToPool(pool)
		if err != nil {
			return "", err
		}
		klog.Infof("Network %s successfully mapped to network %s", network, mappedNetwork)
		return mappedNetwork, nil
	}
	if ok && network != pool {
		_, err := liqoIPAM.ipam.AcquireSpecificChildPrefix(pool, network)
		if err != nil && !strings.Contains(err.Error(), "is not available") {
			/* Unknown error, return */
			return "", fmt.Errorf("cannot acquire prefix %s from prefix %s: %w", network, pool, err)
		}
		if err == nil {
			return network, nil
		}
	}
	/* Network is already reserved, need a mapping */
	mappedNetwork, err = liqoIPAM.getNetworkFromPool(GetMask(network))
	if err != nil {
		return "", err
	}
	klog.Infof("Network %s successfully mapped to network %s", network, mappedNetwork)
	return mappedNetwork, nil
}

/*
GetSubnetsPerCluster receives a PodCIDR, and a Cluster ID and returns a PodCIDR and an ExternalCIDR.
The PodCIDR can be either the received one or a new one, if conflicts have been found.
The same happens for ExternalCIDR.
*/
func (liqoIPAM *IPAM) GetSubnetsPerCluster(
	podCidr,
	externalCIDR,
	clusterID string) (mappedPodCIDR, mappedExternalCIDR string, err error) {
	var exists bool
	// Get subnets of clusters
	clusterSubnets, err := liqoIPAM.ipamStorage.getClusterSubnets()
	if err != nil {
		return "", "", fmt.Errorf("cannot get Ipam config: %w", err)
	}

	// Check existence
	subnets, exists := clusterSubnets[clusterID]
	if exists && subnets.PodCIDR != "" && subnets.LocalExternalCIDR != "" {
		return subnets.PodCIDR, subnets.LocalExternalCIDR, nil
	}

	// Check if podCidr is a valid CIDR
	err = IsValidCidr(podCidr)
	if err != nil {
		return "", "", fmt.Errorf("PodCidr is an invalid CIDR:%w", err)
	}

	klog.Infof("Cluster networks allocation request received: %s", clusterID)

	// Get PodCidr
	mappedPodCIDR, err = liqoIPAM.getOrRemapNetwork(podCidr)
	if err != nil {
		return "", "", fmt.Errorf("cannot get a PodCIDR for cluster %s:%w", clusterID, err)
	}

	klog.Infof("PodCIDR %s has been assigned to cluster %s", mappedPodCIDR, clusterID)

	// Check if externalCIDR is a valid CIDR
	err = IsValidCidr(externalCIDR)
	if err != nil {
		return "", "", fmt.Errorf("ExternalCIDR is an invalid CIDR:%w", err)
	}

	// Get ExternalCIDR
	mappedExternalCIDR, err = liqoIPAM.getOrRemapNetwork(externalCIDR)
	if err != nil {
		_ = liqoIPAM.FreeReservedSubnet(mappedPodCIDR)
		return "", "", fmt.Errorf("cannot get an ExternalCIDR for cluster %s:%w", clusterID, err)
	}

	klog.Infof("ExternalCIDR %s has been assigned to cluster %s", mappedExternalCIDR, clusterID)

	if !exists {
		// Create cluster network configuration
		subnets = netv1alpha1.Subnets{
			PodCIDR:            mappedPodCIDR,
			LocalExternalCIDR:  mappedExternalCIDR,
			RemoteExternalCIDR: "",
		}
	} else {
		// Update cluster network configuration
		subnets.PodCIDR = mappedPodCIDR
		subnets.LocalExternalCIDR = mappedExternalCIDR
	}
	clusterSubnets[clusterID] = subnets

	// Push it in clusterSubnets
	if err := liqoIPAM.ipamStorage.updateClusterSubnets(clusterSubnets); err != nil {
		_ = liqoIPAM.FreeReservedSubnet(mappedPodCIDR)
		_ = liqoIPAM.FreeReservedSubnet(externalCIDR)
		return "", "", fmt.Errorf("cannot update cluster subnets:%w", err)
	}
	return mappedPodCIDR, mappedExternalCIDR, nil
}

// getNetworkFromPool returns a network with mask length equal to mask taken by a network pool.
func (liqoIPAM *IPAM) getNetworkFromPool(mask uint8) (string, error) {
	// Get network pools
	pools, err := liqoIPAM.ipamStorage.getPools()
	if err != nil {
		return "", fmt.Errorf("cannot get network pools: %w", err)
	}
	// For each pool, try to get a network with mask length mask
	for _, pool := range pools {
		if mappedNetwork, err := liqoIPAM.ipam.AcquireChildPrefix(pool, mask); err == nil {
			klog.Infof("Acquired network %s", mappedNetwork)
			return mappedNetwork.String(), nil
		}
	}
	return "", fmt.Errorf("no networks available")
}

func (liqoIPAM *IPAM) freePoolInHalves(pool string) error {
	// Get halves mask length
	mask := GetMask(pool)
	mask++

	// Get first half CIDR
	halfCidr, err := SetMask(pool, mask)
	if err != nil {
		return err
	}

	klog.Infof("Network %s is equal to a network pool, freeing first half..", pool)
	err = liqoIPAM.ipam.ReleaseChildPrefix(liqoIPAM.ipam.PrefixFrom(halfCidr))
	if err != nil {
		return fmt.Errorf("cannot free first half of pool %s", pool)
	}

	// Get second half CIDR
	halfCidr, err = Next(halfCidr)
	if err != nil {
		return err
	}
	klog.Infof("Freeing second half..")
	err = liqoIPAM.ipam.ReleaseChildPrefix(liqoIPAM.ipam.PrefixFrom(halfCidr))
	if err != nil {
		return fmt.Errorf("cannot free second half of pool %s", pool)
	}
	klog.Infof("Network %s has successfully been freed", pool)
	return nil
}

/* FreeReservedSubnet marks as free a reserved subnet. */
func (liqoIPAM *IPAM) FreeReservedSubnet(network string) error {
	var p *goipam.Prefix

	// Check existence
	if p = liqoIPAM.ipam.PrefixFrom(network); p == nil {
		return nil
	}

	// Check if it is equal to a network pool
	pool, ok, err := liqoIPAM.getPoolFromNetwork(network)
	if err != nil {
		return err
	}
	if ok && pool == network {
		return liqoIPAM.freePoolInHalves(pool)
	}

	// Try to release it as a child prefix
	if err := liqoIPAM.ipam.ReleaseChildPrefix(p); err != nil {
		klog.Infof("Cannot release subnet %s previously allocated from the pools", network)
		// It is not a child prefix, then it is a parent prefix, so delete it
		if _, err := liqoIPAM.ipam.DeletePrefix(network); err != nil {
			klog.Errorf("Cannot delete prefix %s", network)
			return fmt.Errorf("cannot remove subnet %s", network)
		}
	}
	klog.Infof("Network %s has just been freed", network)
	return nil
}

// eventuallyDeleteClusterSubnet deletes cluster entry from cluster subnets if all fields are deleted (empty string).
func (liqoIPAM *IPAM) eventuallyDeleteClusterSubnet(clusterID string,
	clusterSubnets map[string]netv1alpha1.Subnets) error {
	// Get entry of cluster
	subnets := clusterSubnets[clusterID]

	// Check is all field are the empty string
	if subnets.PodCIDR == "" && subnets.LocalExternalCIDR == "" && subnets.RemoteExternalCIDR == "" {
		// Delete entry
		delete(clusterSubnets, clusterID)
	}
	// Update
	if err := liqoIPAM.ipamStorage.updateClusterSubnets(clusterSubnets); err != nil {
		return err
	}
	return nil
}

/* FreeSubnetPerCluster marks as free the network previously allocated for cluster clusterID. */
func (liqoIPAM *IPAM) FreeSubnetsPerCluster(clusterID string) error {
	var subnets netv1alpha1.Subnets
	var exists bool
	// Get cluster subnets
	clusterSubnets, err := liqoIPAM.ipamStorage.getClusterSubnets()
	if err != nil {
		return fmt.Errorf("cannot get cluster subnets: %w", err)
	}
	subnets, exists = clusterSubnets[clusterID]
	if !exists || subnets.PodCIDR == "" || subnets.LocalExternalCIDR == "" {
		//Networks do not exist
		return nil
	}
	// Free PodCidr
	if err := liqoIPAM.FreeReservedSubnet(subnets.PodCIDR); err != nil {
		return err
	}
	subnets.PodCIDR = ""

	// Free ExternalCidr
	if err := liqoIPAM.FreeReservedSubnet(subnets.LocalExternalCIDR); err != nil {
		return err
	}
	subnets.LocalExternalCIDR = ""

	clusterSubnets[clusterID] = subnets

	klog.Infof("Networks assigned to cluster %s have just been freed", clusterID)

	if err := liqoIPAM.eventuallyDeleteClusterSubnet(clusterID, clusterSubnets); err != nil {
		return err
	}
	return nil
}

// AddNetworkPool adds a network to the set of network pools.
func (liqoIPAM *IPAM) AddNetworkPool(network string) error {
	// Get resource
	ipamPools, err := liqoIPAM.ipamStorage.getPools()
	if err != nil {
		return fmt.Errorf("cannot get Ipam config: %w", err)
	}
	// Check overlapping with existing pools
	// Either this and the following checks are carried out also within NewPrefix.
	// Perform them here permits a more detailed error description.
	pool, overlaps, err := liqoIPAM.overlapsWithPool(network)
	if err != nil {
		return fmt.Errorf("cannot establish if new network pool overlaps with existing network pools:%w", err)
	}
	if overlaps {
		return fmt.Errorf("cannot add new network pool %s because it overlaps with existing network pool %s", network, pool)
	}
	// Check overlapping with cluster subnets
	cluster, overlaps, err := liqoIPAM.overlapsWithCluster(network)
	if err != nil {
		return fmt.Errorf("cannot establish if new network pool overlaps with a reserved subnet:%w", err)
	}
	if overlaps {
		return fmt.Errorf("cannot add network pool %s because it overlaps with network of cluster %s", network, cluster)
	}
	// Add network pool
	_, err = liqoIPAM.ipam.NewPrefix(network)
	if err != nil {
		return fmt.Errorf("cannot add network pool %s:%w", network, err)
	}
	ipamPools = append(ipamPools, network)
	klog.Infof("Network pool %s added to IPAM", network)
	// Update configuration
	err = liqoIPAM.ipamStorage.updatePools(ipamPools)
	if err != nil {
		return fmt.Errorf("cannot update Ipam configuration:%w", err)
	}
	return nil
}

// RemoveNetworkPool removes a network from the set of network pools.
func (liqoIPAM *IPAM) RemoveNetworkPool(network string) error {
	// Get resource
	ipamPools, err := liqoIPAM.ipamStorage.getPools()
	if err != nil {
		return fmt.Errorf("cannot get Ipam config: %w", err)
	}
	// Get cluster subnets
	clusterSubnets, err := liqoIPAM.ipamStorage.getClusterSubnets()
	if err != nil {
		return fmt.Errorf("cannot get cluster subnets: %w", err)
	}
	// Check existence
	if exists := slice.ContainsString(ipamPools, network, nil); !exists {
		return fmt.Errorf("network %s is not a network pool", network)
	}
	// Cannot remove a default one
	if contains := slice.ContainsString(Pools, network, nil); contains {
		return fmt.Errorf("cannot remove a default network pool")
	}
	// Check overlapping with cluster networks
	cluster, overlaps, err := liqoIPAM.overlapsWithCluster(network)
	if err != nil {
		return fmt.Errorf("cannot check if network pool %s overlaps with cluster networks:%w", network, err)
	}
	if overlaps {
		return fmt.Errorf("cannot remove network pool %s because it overlaps with network %s of cluster %s",
			network, clusterSubnets[cluster], cluster)
	}
	// Release it
	_, err = liqoIPAM.ipam.DeletePrefix(network)
	if err != nil {
		return fmt.Errorf("cannot remove network pool %s:%w", network, err)
	}
	// Delete it
	var i int
	for index, value := range ipamPools {
		if value == network {
			i = index
			break
		}
	}
	if i == (len(ipamPools) - 1) {
		ipamPools = ipamPools[:len(ipamPools)-1]
	} else {
		copy(ipamPools[i:], ipamPools[i+1:])
		ipamPools = ipamPools[:len(ipamPools)-1]
	}
	err = liqoIPAM.ipamStorage.updatePools(ipamPools)
	if err != nil {
		return fmt.Errorf("cannot update Ipam configuration:%w", err)
	}
	klog.Infof("Network pool %s has just been removed", network)
	return nil
}

// AddExternalCIDRPerCluster stores (without reserving) an ExternalCIDR for a remote cluster
// since this network is used in the remote cluster.
func (liqoIPAM *IPAM) AddExternalCIDRPerCluster(network, clusterID string) error {
	var exists bool
	var subnets netv1alpha1.Subnets
	// Get cluster subnets
	clusterSubnets, err := liqoIPAM.ipamStorage.getClusterSubnets()
	if err != nil {
		return fmt.Errorf("cannot get cluster subnets: %w", err)
	}
	// Check existence
	subnets, exists = clusterSubnets[clusterID]
	if exists && subnets.RemoteExternalCIDR != "" {
		return nil
	}

	// Set remote ExternalCIDR
	if exists {
		subnets.RemoteExternalCIDR = network
	} else {
		subnets = netv1alpha1.Subnets{
			PodCIDR:            "",
			LocalExternalCIDR:  "",
			RemoteExternalCIDR: network,
		}
	}
	clusterSubnets[clusterID] = subnets
	klog.Infof("Remote ExternalCIDR of cluster %s set to %s", clusterID, network)
	// Push it in clusterSubnets
	if err := liqoIPAM.ipamStorage.updateClusterSubnets(clusterSubnets); err != nil {
		return fmt.Errorf("cannot update cluster subnets:%w", err)
	}
	return nil
}

// RemoveExternalCIDRPerCluster deletes an ExternalCIDR for a cluster.
func (liqoIPAM *IPAM) RemoveExternalCIDRPerCluster(clusterID string) error {
	var exists bool
	var subnets netv1alpha1.Subnets
	// Get cluster subnets
	clusterSubnets, err := liqoIPAM.ipamStorage.getClusterSubnets()
	if err != nil {
		return fmt.Errorf("cannot get cluster subnets: %w", err)
	}
	// Check existence
	subnets, exists = clusterSubnets[clusterID]
	if !exists || subnets.RemoteExternalCIDR == "" {
		return nil
	}

	// Unset remote ExternalCIDR
	subnets.RemoteExternalCIDR = ""
	clusterSubnets[clusterID] = subnets

	klog.Infof("Remote ExternalCIDR of cluster %s deleted", clusterID)
	if err := liqoIPAM.eventuallyDeleteClusterSubnet(clusterID, clusterSubnets); err != nil {
		return err
	}
	return nil
}

// GetClusterExternalCIDR eventually choose and returns the local cluster's ExternalCIDR.
func (liqoIPAM *IPAM) GetClusterExternalCIDR(mask uint8) (string, error) {
	var externalCIDR string
	var err error
	// Get cluster ExternalCIDR
	externalCIDR, err = liqoIPAM.ipamStorage.getExternalCIDR()
	if err != nil {
		return "", fmt.Errorf("cannot get ExternalCIDR: %w", err)
	}
	if externalCIDR != "" {
		return externalCIDR, nil
	}
	if externalCIDR, err = liqoIPAM.getNetworkFromPool(mask); err != nil {
		return "", fmt.Errorf("cannot allocate an ExternalCIDR:%w", err)
	}
	if err := liqoIPAM.ipamStorage.updateExternalCIDR(externalCIDR); err != nil {
		_ = liqoIPAM.FreeReservedSubnet(externalCIDR)
		return "", fmt.Errorf("cannot update ExternalCIDR:%w", err)
	}
	return externalCIDR, nil
}

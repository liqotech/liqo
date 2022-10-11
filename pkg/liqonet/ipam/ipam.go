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

package ipam

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	goipam "github.com/metal-stack/go-ipam"
	grpc "google.golang.org/grpc"
	"inet.af/netaddr"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	liqoneterrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/natmappinginflater"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

// Ipam Interface.
type Ipam interface {
	/* GetSubnetsPerCluster receives PodCIDR and ExternalCIDR of a remote cluster and checks if
	those networks generate conflicts with other networks(reserved ones or even PodCIDR and
	ExternalCIDR of other clusters). If no conflicts are found, networks are reserved so that
	they cannot be used by any other cluster. In this way IPAM guarrantees that traffic toward these
	networks is directed to only one remote cluster. If conflicts are found, received networks are
	ignored and they are substituted with a new network chosen by the IPAM. These new network are
	reserved as well. The remapping mechanism can be applied on:
	- PodCIDR
	- ExternalCIDR
	- Both.
	*/
	GetSubnetsPerCluster(podCidr, externalCIDR, clusterID string) (string, string, error)
	// RemoveClusterConfig deletes the IPAM configuration of a remote cluster,
	// by freeing networks and removing data structures related to that cluster.
	RemoveClusterConfig(clusterID string) error
	// AcquireReservedSubnet reserves a network.
	AcquireReservedSubnet(network string) error
	// FreeReservedSubnet frees a network.
	FreeReservedSubnet(network string) error
	// AddNetworkPool adds a network to the set of default network pools.
	AddNetworkPool(network string) error
	// RemoveNetworkPool removes a network from the set of network pools.
	RemoveNetworkPool(network string) error
	/* AddLocalSubnetsPerCluster stores the PodCIDR and the ExternalCIDR used in the remote cluster to
	map the local cluster subnets. Since those networks are used in the remote cluster
	this function must not reserve it. If the remote cluster has not remapped
	a local subnet, then CIDR value should be equal to "None". */
	AddLocalSubnetsPerCluster(podCIDR, externalCIDR, clusterID string) error
	GetExternalCIDR(mask uint8) (string, error)
	// SetPodCIDR sets the cluster PodCIDR.
	SetPodCIDR(podCIDR string) error
	// SetServiceCIDR sets the cluster ServiceCIDR.
	SetServiceCIDR(serviceCIDR string) error
	// Terminate function enforces a graceful termination of the IPAM module.
	Terminate()
	// SetSpecificNatMapping sets a specific NAT mapping.
	SetSpecificNatMapping(oldIPLocal, oldIP, newIP, clusterID string) error
	IpamServer
}

// IPAM implementation.
type IPAM struct {
	ipam               goipam.Ipamer
	ipamStorage        IpamStorage
	natMappingInflater natmappinginflater.Interface
	grpcServer         *grpc.Server
	mutex              sync.Mutex
	UnimplementedIpamServer
}

// NewIPAM returns a IPAM instance.
func NewIPAM() *IPAM {
	return &IPAM{}
}

// Pools is a constant slice containing private IPv4 networks.
var Pools = []string{
	"10.0.0.0/8",
	"192.168.0.0/16",
	"172.16.0.0/12",
}

const emptyCIDR = ""

// Init uses the Ipam resource to retrieve and allocate reserved networks.
func (liqoIPAM *IPAM) Init(pools []string, dynClient dynamic.Interface, listeningPort int) error {
	var err error
	// Set up storage
	liqoIPAM.ipamStorage, err = NewIPAMStorage(dynClient)
	if err != nil {
		return fmt.Errorf("cannot set up storage for ipam: %w", err)
	}
	liqoIPAM.ipam = goipam.NewWithStorage(liqoIPAM.ipamStorage)

	// Get resource
	ipamPools := liqoIPAM.ipamStorage.getPools()

	// Have network pools been already set? If not, take them from caller
	if len(ipamPools) == 0 {
		for _, network := range pools {
			if _, err := liqoIPAM.ipam.NewPrefix(context.TODO(), network); err != nil {
				return fmt.Errorf("failed to create a new prefix for network %s: %w", network, err)
			}
			ipamPools = append(ipamPools, network)
			klog.Infof("Pool %s has been successfully added to the pool list", network)
		}
		err = liqoIPAM.ipamStorage.updatePools(ipamPools)
		if err != nil {
			return fmt.Errorf("cannot set pools: %w", err)
		}
	}
	if listeningPort > 0 {
		err = liqoIPAM.initRPCServer(listeningPort)
		if err != nil {
			return fmt.Errorf("cannot start gRPC server: %w", err)
		}
	}

	liqoIPAM.natMappingInflater = natmappinginflater.NewInflater(dynClient)
	return nil
}

// Terminate function stops the gRPC server.
func (liqoIPAM *IPAM) Terminate() {
	// Stop GRPC server
	liqoIPAM.grpcServer.GracefulStop()
}

func (liqoIPAM *IPAM) initRPCServer(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s%d", "0.0.0.0:", port))
	if err != nil {
		return err
	}
	liqoIPAM.grpcServer = grpc.NewServer()
	RegisterIpamServer(liqoIPAM.grpcServer, liqoIPAM)
	go func() {
		err := liqoIPAM.grpcServer.Serve(lis)
		if err != nil {
			klog.Error(err)
		}
	}()
	return nil
}

// reservePoolInHalves handles the special case in which a network pool has to be entirely reserved
// Since AcquireSpecificChildPrefix would return an error, reservePoolInHalves acquires the two
// halves of the network pool.
func (liqoIPAM *IPAM) reservePoolInHalves(pool string) error {
	klog.Infof("Network %s is equal to a network pool, acquiring first half..", pool)
	mask := liqonetutils.GetMask(pool)
	mask++
	_, err := liqoIPAM.ipam.AcquireChildPrefix(context.TODO(), pool, mask)
	if err != nil {
		return fmt.Errorf("cannot acquire first half of pool %s: %w", pool, err)
	}
	klog.Infof("Acquiring second half..")
	_, err = liqoIPAM.ipam.AcquireChildPrefix(context.TODO(), pool, mask)
	if err != nil {
		return fmt.Errorf("cannot acquire second half of pool %s: %w", pool, err)
	}
	klog.Infof("Network %s has successfully been reserved", pool)
	return nil
}

// AcquireReservedSubnet marks as used the network received as parameter.
func (liqoIPAM *IPAM) AcquireReservedSubnet(reservedNetwork string) error {
	klog.Infof("Request to reserve network %s has been received", reservedNetwork)
	cluster, overlaps, err := liqoIPAM.overlapsWithCluster(reservedNetwork)
	if err != nil {
		return fmt.Errorf("cannot acquire network %s: %w", reservedNetwork, err)
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
		if _, err := liqoIPAM.ipam.AcquireSpecificChildPrefix(context.TODO(), pool, reservedNetwork); err != nil {
			return fmt.Errorf("cannot reserve network %s: %w", reservedNetwork, err)
		}
		klog.Infof("Network %s has successfully been reserved", reservedNetwork)
		return nil
	}
	if _, err := liqoIPAM.ipam.NewPrefix(context.TODO(), reservedNetwork); err != nil {
		return fmt.Errorf("cannot reserve network %s: %w", reservedNetwork, err)
	}
	klog.Infof("Network %s has successfully been reserved.", reservedNetwork)
	return nil
}

// MarkAsAcquiredReservedSubnet marks as used the network received as parameter.
func (liqoIPAM *IPAM) MarkAsAcquiredReservedSubnet(reservedNetwork string) error {
	klog.Infof("Request to reserve network %s has been received", reservedNetwork)

	pool, ok, err := liqoIPAM.getPoolFromNetwork(reservedNetwork)
	if err != nil {
		return err
	}
	if ok && reservedNetwork == pool {
		klog.Infof("reserving subnet %s in two halves...", reservedNetwork)
		for _, half := range liqonetutils.SplitNetwork(reservedNetwork) {
			if !liqoIPAM.isAcquired(half) {
				if _, err := liqoIPAM.ipam.AcquireSpecificChildPrefix(context.TODO(), pool, half); err != nil {
					return fmt.Errorf("cannot reserve network %s: %w", reservedNetwork, err)
				}
			}
			klog.Infof("half %s for subnet %s successfully acquired", half, reservedNetwork)
		}
	}
	if ok && reservedNetwork != pool {
		if !liqoIPAM.isAcquired(reservedNetwork) {
			if _, err := liqoIPAM.ipam.AcquireSpecificChildPrefix(context.TODO(), pool, reservedNetwork); err != nil {
				return fmt.Errorf("cannot reserve network %s: %w", reservedNetwork, err)
			}
		}
		klog.Infof("Network %s has successfully been reserved", reservedNetwork)
		return nil
	}
	if !liqoIPAM.isAcquired(reservedNetwork) {
		if _, err := liqoIPAM.ipam.NewPrefix(context.TODO(), reservedNetwork); err != nil {
			return fmt.Errorf("cannot reserve network %s: %w", reservedNetwork, err)
		}
	}
	klog.Infof("Network %s has successfully been reserved.", reservedNetwork)
	return nil
}

func (liqoIPAM *IPAM) overlapsWithNetwork(newNetwork, network string) (overlaps bool, err error) {
	if network == "" {
		return
	}
	if err = goipam.PrefixesOverlapping([]string{network}, []string{newNetwork}); err != nil && strings.Contains(err.Error(), "overlaps") {
		// overlaps
		overlaps = true
		err = nil
		return
	}
	return
}

func (liqoIPAM *IPAM) overlapsWithCluster(network string) (overlappingCluster string, overlaps bool, err error) {
	var overlapsWithPodCIDR bool
	var overlapsWithExternalCIDR bool
	// Get cluster subnets
	clusterSubnets := liqoIPAM.ipamStorage.getClusterSubnets()
	for cluster, subnets := range clusterSubnets {
		overlapsWithPodCIDR, err = liqoIPAM.overlapsWithNetwork(network, subnets.RemotePodCIDR)
		if err != nil {
			return
		}
		overlapsWithExternalCIDR, err = liqoIPAM.overlapsWithNetwork(network, subnets.RemoteExternalCIDR)
		if err != nil {
			return
		}
		if overlapsWithPodCIDR || overlapsWithExternalCIDR {
			overlaps = true
			overlappingCluster = cluster
			return
		}
	}
	return overlappingCluster, overlaps, err
}

func (liqoIPAM *IPAM) overlapsWithPool(network string) (overlappingPool string, overlaps bool, err error) {
	// Get resource
	pools := liqoIPAM.ipamStorage.getPools()
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

func (liqoIPAM *IPAM) overlapsWithReserved(network string) (overlappingReserved string, overlaps bool, err error) {
	reserved := liqoIPAM.ipamStorage.getReservedSubnets()
	for _, r := range reserved {
		if overlaps, err = liqoIPAM.overlapsWithNetwork(network, r); err != nil {
			return
		}

		if overlaps {
			overlappingReserved = r
			return
		}
	}
	return
}

// hasBeenAcquired checks for a given network if it has been acquired by checking if a prefix equal to
// the network exists.
func (liqoIPAM *IPAM) isAcquired(network string) bool {
	if p := liqoIPAM.ipam.PrefixFrom(context.TODO(), network); p != nil {
		return true
	}
	return false
}

// Function that receives a network as parameter and returns the pool to which this network belongs to.
func (liqoIPAM *IPAM) getPoolFromNetwork(network string) (networkPool string, success bool, err error) {
	var poolIPset netaddr.IPSetBuilder
	var c netaddr.IPPrefix
	// Get resource
	pools := liqoIPAM.ipamStorage.getPools()
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
		var ipSet *netaddr.IPSet
		ipSet, err = poolIPset.IPSet()
		if err != nil {
			return
		}
		if ipSet.ContainsPrefix(ipprefix) {
			networkPool = pool
			success = true
			return
		}
	}
	return
}

func (liqoIPAM *IPAM) clusterSubnetEqualToPool(pool string) (string, error) {
	klog.Infof("Network %s is equal to a pool, looking for a mapping..", pool)
	mappedNetwork, err := liqoIPAM.getNetworkFromPool(liqonetutils.GetMask(pool))
	if err != nil {
		klog.Infof("Mapping not found, acquiring the entire network pool..")
		err = liqoIPAM.reservePoolInHalves(pool)
		if err != nil {
			return "", fmt.Errorf("no networks available: %w", err)
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
	_, err := liqoIPAM.ipam.NewPrefix(context.TODO(), network)

	if err != nil && !strings.Contains(err.Error(), "overlaps") {
		// Return if get an error that is not an overlapping error
		return "", fmt.Errorf("cannot reserve network %s: %w", network, err)
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
		_, err := liqoIPAM.ipam.AcquireSpecificChildPrefix(context.TODO(), pool, network)
		if err != nil && !strings.Contains(err.Error(), "is not available") {
			/* Unknown error, return */
			return "", fmt.Errorf("cannot acquire prefix %s from prefix %s: %w", network, pool, err)
		}
		if err == nil {
			return network, nil
		}
	}
	/* Network is already reserved, need a mapping */
	mappedNetwork, err = liqoIPAM.getNetworkFromPool(liqonetutils.GetMask(network))
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
	clusterSubnets := liqoIPAM.ipamStorage.getClusterSubnets()

	// Check existence
	subnets, exists := clusterSubnets[clusterID]
	if exists && subnets.RemotePodCIDR != "" && subnets.RemoteExternalCIDR != "" {
		return subnets.RemotePodCIDR, subnets.RemoteExternalCIDR, nil
	}

	// Check if podCidr is a valid CIDR
	err = liqonetutils.IsValidCIDR(podCidr)
	if err != nil {
		return "", "", fmt.Errorf("PodCidr is an invalid CIDR: %w", err)
	}

	klog.Infof("Cluster networks allocation request received: %s", clusterID)

	// Get PodCidr
	mappedPodCIDR, err = liqoIPAM.getOrRemapNetwork(podCidr)
	if err != nil {
		return "", "", fmt.Errorf("cannot get a PodCIDR for cluster %s: %w", clusterID, err)
	}

	klog.Infof("PodCIDR %s has been assigned to cluster %s", mappedPodCIDR, clusterID)

	// Check if externalCIDR is a valid CIDR
	err = liqonetutils.IsValidCIDR(externalCIDR)
	if err != nil {
		return "", "", fmt.Errorf("ExternalCIDR is an invalid CIDR: %w", err)
	}

	// Get ExternalCIDR
	mappedExternalCIDR, err = liqoIPAM.getOrRemapNetwork(externalCIDR)
	if err != nil {
		_ = liqoIPAM.FreeReservedSubnet(mappedPodCIDR)
		return "", "", fmt.Errorf("cannot get an ExternalCIDR for cluster %s: %w", clusterID, err)
	}

	klog.Infof("ExternalCIDR %s has been assigned to cluster %s", mappedExternalCIDR, clusterID)

	if !exists {
		// Create cluster network configuration
		subnets = netv1alpha1.Subnets{
			LocalNATPodCIDR:      "",
			RemotePodCIDR:        mappedPodCIDR,
			RemoteExternalCIDR:   mappedExternalCIDR,
			LocalNATExternalCIDR: "",
		}
	} else {
		// Update cluster network configuration
		subnets.RemotePodCIDR = mappedPodCIDR
		subnets.RemoteExternalCIDR = mappedExternalCIDR
	}
	clusterSubnets[clusterID] = subnets

	// Push it in clusterSubnets
	if err := liqoIPAM.ipamStorage.updateClusterSubnets(clusterSubnets); err != nil {
		_ = liqoIPAM.FreeReservedSubnet(mappedPodCIDR)
		_ = liqoIPAM.FreeReservedSubnet(mappedExternalCIDR)
		return "", "", fmt.Errorf("cannot update cluster subnets: %w", err)
	}
	return mappedPodCIDR, mappedExternalCIDR, nil
}

// getNetworkFromPool returns a network with mask length equal to mask taken by a network pool.
func (liqoIPAM *IPAM) getNetworkFromPool(mask uint8) (string, error) {
	// Get network pools
	pools := liqoIPAM.ipamStorage.getPools()
	// For each pool, try to get a network with mask length mask
	for _, pool := range pools {
		if mappedNetwork, err := liqoIPAM.ipam.AcquireChildPrefix(context.TODO(), pool, mask); err == nil {
			klog.Infof("Acquired network %s", mappedNetwork)
			return mappedNetwork.String(), nil
		}
	}
	return "", fmt.Errorf("no networks available")
}

func (liqoIPAM *IPAM) freePoolInHalves(pool string) error {
	var err error

	// Get halves mask length
	mask := liqonetutils.GetMask(pool)
	mask++

	// Get first half CIDR
	halfCidr := liqonetutils.SetMask(pool, mask)

	klog.Infof("Network %s is equal to a network pool, freeing first half..", pool)
	if prefix := liqoIPAM.ipam.PrefixFrom(context.TODO(), halfCidr); prefix != nil {
		err = liqoIPAM.ipam.ReleaseChildPrefix(context.TODO(), prefix)
		if err != nil {
			return fmt.Errorf("cannot free first half of pool %s", pool)
		}
	}

	// Get second half CIDR
	halfCidr = liqonetutils.Next(halfCidr)
	if err != nil {
		return err
	}
	klog.Infof("Freeing second half..")
	if prefix := liqoIPAM.ipam.PrefixFrom(context.TODO(), halfCidr); prefix != nil {
		err = liqoIPAM.ipam.ReleaseChildPrefix(context.TODO(), prefix)
		if err != nil {
			return fmt.Errorf("cannot free second half of pool %s", pool)
		}
	}

	klog.Infof("Network %s has successfully been freed", pool)
	return nil
}

// FreeReservedSubnet marks as free a reserved subnet.
func (liqoIPAM *IPAM) FreeReservedSubnet(network string) error {
	var p *goipam.Prefix

	// Check existence
	if p = liqoIPAM.ipam.PrefixFrom(context.TODO(), network); p == nil {
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
	if err := liqoIPAM.ipam.ReleaseChildPrefix(context.TODO(), p); err != nil {
		klog.Infof("Cannot release subnet %s previously allocated from the pools", network)
		// It is not a child prefix, then it is a parent prefix, so delete it
		if _, err := liqoIPAM.ipam.DeletePrefix(context.TODO(), network); err != nil {
			klog.Errorf("Cannot delete prefix %s", network)
			return fmt.Errorf("cannot remove subnet %s: %w", network, err)
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
	if subnets.RemotePodCIDR == "" &&
		subnets.LocalNATPodCIDR == "" &&
		subnets.RemoteExternalCIDR == "" &&
		subnets.LocalNATExternalCIDR == "" {
		// Delete entry
		delete(clusterSubnets, clusterID)
	}
	// Update
	if err := liqoIPAM.ipamStorage.updateClusterSubnets(clusterSubnets); err != nil {
		return err
	}
	return nil
}

// RemoveClusterConfig frees remote PodCIDR and ExternalCIDR and
// deletes local subnets for the remote cluster.
func (liqoIPAM *IPAM) RemoveClusterConfig(clusterID string) error {
	var subnets netv1alpha1.Subnets
	var subnetsExist, natMappingsPerClusterConfigured bool

	if clusterID == "" {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.ClusterIDLabelName,
			Reason:    liqoneterrors.StringNotEmpty,
		}
	}

	// Get cluster subnets
	clusterSubnets := liqoIPAM.ipamStorage.getClusterSubnets()

	// Get NatMappingsConfigured map
	natMappingsConfigured := liqoIPAM.ipamStorage.getNatMappingsConfigured()

	subnets, subnetsExist = clusterSubnets[clusterID]
	_, natMappingsPerClusterConfigured = natMappingsConfigured[clusterID]
	if !subnetsExist && !natMappingsPerClusterConfigured {
		// Nothing to be done here
		return nil
	}

	// If an error happened after the following if, there is no need of
	// re-executing the following block.
	if subnetsExist {
		// Free PodCidr
		if err := liqoIPAM.FreeReservedSubnet(subnets.RemotePodCIDR); err != nil {
			return err
		}

		// Free ExternalCidr
		if err := liqoIPAM.FreeReservedSubnet(subnets.RemoteExternalCIDR); err != nil {
			return err
		}
		klog.Infof("Networks assigned to cluster %s have just been freed", clusterID)

		delete(clusterSubnets, clusterID)
		if err := liqoIPAM.ipamStorage.updateClusterSubnets(clusterSubnets); err != nil {
			return fmt.Errorf("cannot update clusterSubnets: %w", err)
		}
	}

	// Terminate NatMappings
	if err := liqoIPAM.terminateNatMappingsPerCluster(clusterID); err != nil {
		return fmt.Errorf("unable to terminate NAT mappings for cluster %s: %w", clusterID, err)
	}

	delete(natMappingsConfigured, clusterID)
	// Update natMappingsConfigured
	natMappingsConfigured[clusterID] = netv1alpha1.ConfiguredCluster{}
	if err := liqoIPAM.ipamStorage.updateNatMappingsConfigured(natMappingsConfigured); err != nil {
		return fmt.Errorf("unable to update NatMappingsConfigured: %w", err)
	}
	return nil
}

// initNatMappingsPerCluster is a wrapper for inflater InitNatMappingsPerCluster.
func (liqoIPAM *IPAM) initNatMappingsPerCluster(clusterID string, subnets netv1alpha1.Subnets) error {
	// InitNatMappingsPerCluster does need the Pod CIDR used in home cluster for remote pods (subnets.RemotePodCIDR)
	// and the ExternalCIDR used in remote cluster for local exported resources.
	localExternalCIDR := liqoIPAM.ipamStorage.getExternalCIDR()
	var externalCIDR string
	if subnets.LocalNATExternalCIDR == consts.DefaultCIDRValue {
		// Remote cluster has not remapped home ExternalCIDR
		externalCIDR = localExternalCIDR
	} else {
		externalCIDR = subnets.LocalNATExternalCIDR
	}

	if err := liqoIPAM.natMappingInflater.InitNatMappingsPerCluster(subnets.RemotePodCIDR, externalCIDR, clusterID); err != nil {
		return err
	}

	// Acquire the IP used to remap the wireguard interface IP
	// if an error occurs, it means that the IP has been previously acquired by the network manager,
	// so the process can continue without exiting.

	// Set a NAT rule which allows remote clusters to ping local cluster's tunnel IP through the externalCIDR.
	natTunnelIP, err := liqonetutils.GetTunnelIP(externalCIDR)
	if err != nil {
		return fmt.Errorf("unable to get tunnel IP per cluster %s: %w", clusterID, err)
	}
	natTunnelIPLocal, err := liqonetutils.GetTunnelIP(localExternalCIDR)
	if err != nil {
		return fmt.Errorf("unable to get tunnel IP per cluster %s: %w", clusterID, err)
	}

	err = liqoIPAM.AcquireSpecificIP(natTunnelIPLocal, localExternalCIDR)
	if err != nil {
		return fmt.Errorf("unable to acquire IP %s: %w", natTunnelIPLocal, err)
	}

	if err := liqoIPAM.SetSpecificNatMapping(natTunnelIPLocal, natTunnelIP, consts.WgTunnelIP, clusterID); err != nil {
		return fmt.Errorf("an error occurred while setting the NAT mapping for the nat tunnel ip %s: %w", natTunnelIP, err)
	}

	return nil
}

// terminateNatMappingsPerCluster is used to update endpointMappings after a cluster peering is terminated.
func (liqoIPAM *IPAM) terminateNatMappingsPerCluster(clusterID string) error {
	// Get NAT mappings
	// natMappings keys are the set of endpoint reflected on remote cluster.
	natMappings, err := liqoIPAM.natMappingInflater.GetNatMappings(clusterID)
	if err != nil && !errors.Is(err, &liqoneterrors.MissingInit{
		StructureName: clusterID,
	}) {
		// Unknown error
		return fmt.Errorf("cannot get NAT mappings for cluster %s: %w", clusterID, err)
	}
	if err != nil && errors.Is(err, &liqoneterrors.MissingInit{
		StructureName: clusterID,
	}) {
		/*
			This can happen if:
			a: terminateNatMappingsPerCluster has been called more than once after initialization.
			b. terminateNatMappingsPerCluster has been called once without previous initialization.
			In both circumstances, there are no actions to be performed here.
		*/
		return nil
	}

	// Get endpointMappings
	endpointMappings := liqoIPAM.ipamStorage.getEndpointMappings()

	// Get local ExternalCIDR
	localExternalCIDR := liqoIPAM.ipamStorage.getExternalCIDR()
	if localExternalCIDR == emptyCIDR {
		return fmt.Errorf("cannot get ExternalCIDR: %w", err)
	}

	// Remove cluster from the list of clusters the endpoint is reflected in.
	for ip := range natMappings {
		m := endpointMappings[ip]

		klog.Infof("removed mapping from %s to %s", ip, m.ClusterMappings[clusterID].ExternalCIDRNattedIP)
		delete(m.ClusterMappings, clusterID)

		if len(m.ClusterMappings) == 0 {
			// Free IP
			err = liqoIPAM.ipam.ReleaseIPFromPrefix(context.TODO(), localExternalCIDR, m.ExternalCIDROriginalIP)
			if err != nil && !errors.Is(err, goipam.ErrNotFound) {
				/*
					ReleaseIPFromPrefix can return ErrNotFound either if the prefix
					is not found and if the IP is not allocated.
					Since the prefix represents the ExternalCIDR, whose existence has
					been checked some lines above, ReleaseIPFromPrefix returns
					ErrNotFound if the IP has not been allocated or has already been freed.
				*/
				return fmt.Errorf("cannot free IP: %w", err)
			}
			if err == nil {
				klog.Infof("IP %s (mapped from %s) has been freed", m.ExternalCIDROriginalIP, ip)
			}

			delete(endpointMappings, ip)
		} else {
			endpointMappings[ip] = m
		}
	}

	// Update endpointMappings
	if err := liqoIPAM.ipamStorage.updateEndpointMappings(endpointMappings); err != nil {
		return fmt.Errorf("cannot update endpointMappings: %w", err)
	}

	// Free/Remove resources in Inflater
	if err := liqoIPAM.natMappingInflater.TerminateNatMappingsPerCluster(clusterID); err != nil {
		return err
	}
	return nil
}

// AddNetworkPool adds a network to the set of network pools.
func (liqoIPAM *IPAM) AddNetworkPool(network string) error {
	// Get resource
	ipamPools := liqoIPAM.ipamStorage.getPools()
	// Check overlapping with existing pools
	// Either this and the following checks are carried out also within NewPrefix.
	// Perform them here permits a more detailed error description.
	pool, overlaps, err := liqoIPAM.overlapsWithPool(network)
	if err != nil {
		return fmt.Errorf("cannot establish if new network pool overlaps with existing network pools: %w", err)
	}
	if overlaps {
		return fmt.Errorf("cannot add new network pool %s because it overlaps with existing network pool %s", network, pool)
	}
	// Check overlapping with cluster subnets
	cluster, overlaps, err := liqoIPAM.overlapsWithCluster(network)
	if err != nil {
		return fmt.Errorf("cannot establish if new network pool overlaps with a reserved subnet: %w", err)
	}
	if overlaps {
		return fmt.Errorf("cannot add network pool %s because it overlaps with network of cluster %s", network, cluster)
	}
	// Add network pool
	_, err = liqoIPAM.ipam.NewPrefix(context.TODO(), network)
	if err != nil {
		return fmt.Errorf("cannot add network pool %s: %w", network, err)
	}
	ipamPools = append(ipamPools, network)
	klog.Infof("Network pool %s added to IPAM", network)
	// Update configuration
	err = liqoIPAM.ipamStorage.updatePools(ipamPools)
	if err != nil {
		return fmt.Errorf("cannot update Ipam configuration: %w", err)
	}
	return nil
}

// RemoveNetworkPool removes a network from the set of network pools.
func (liqoIPAM *IPAM) RemoveNetworkPool(network string) error {
	// Get resource
	ipamPools := liqoIPAM.ipamStorage.getPools()
	// Get cluster subnets
	clusterSubnets := liqoIPAM.ipamStorage.getClusterSubnets()
	// Check existence
	if exists := slice.ContainsString(ipamPools, network); !exists {
		return fmt.Errorf("network %s is not a network pool", network)
	}
	// Cannot remove a default one
	if contains := slice.ContainsString(Pools, network); contains {
		return fmt.Errorf("cannot remove a default network pool")
	}
	// Check overlapping with cluster networks
	cluster, overlaps, err := liqoIPAM.overlapsWithCluster(network)
	if err != nil {
		return fmt.Errorf("cannot check if network pool %s overlaps with cluster networks: %w", network, err)
	}
	if overlaps {
		return fmt.Errorf("cannot remove network pool %s because it overlaps with network %s of cluster %s",
			network, clusterSubnets[cluster], cluster)
	}
	// Release it
	_, err = liqoIPAM.ipam.DeletePrefix(context.TODO(), network)
	if err != nil {
		return fmt.Errorf("cannot remove network pool %s: %w", network, err)
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
		return fmt.Errorf("cannot update Ipam configuration: %w", err)
	}
	klog.Infof("Network pool %s has just been removed", network)
	return nil
}

// AddLocalSubnetsPerCluster stores how the PodCIDR and the ExternalCIDR of local cluster
// has been remapped in a remote cluster. If no remapping happened, then the CIDR value should be equal to "None".
func (liqoIPAM *IPAM) AddLocalSubnetsPerCluster(podCIDR, externalCIDR, clusterID string) error {
	var subnetsExist, natMappingsPerClusterConfigured bool
	var subnets netv1alpha1.Subnets
	if clusterID == "" {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.ClusterIDLabelName,
			Reason:    liqoneterrors.StringNotEmpty,
		}
	}

	// Get cluster subnets
	clusterSubnets := liqoIPAM.ipamStorage.getClusterSubnets()

	// Get NatMappingsConfigured map
	natMappingsConfigured := liqoIPAM.ipamStorage.getNatMappingsConfigured()

	// Check existence of subnets struct and NatMappings have already been configured
	subnets, subnetsExist = clusterSubnets[clusterID]
	_, natMappingsPerClusterConfigured = natMappingsConfigured[clusterID]

	if !subnetsExist {
		return fmt.Errorf("remote subnets for cluster %s do not exist yet. Call first GetSubnetsPerCluster",
			clusterID)
	}
	if subnets.LocalNATPodCIDR != "" && subnets.LocalNATExternalCIDR != "" && natMappingsPerClusterConfigured {
		return nil
	}

	// Set networks
	if subnetsExist {
		subnets.LocalNATPodCIDR = podCIDR
		subnets.LocalNATExternalCIDR = externalCIDR
	} else {
		subnets = netv1alpha1.Subnets{
			LocalNATPodCIDR:      podCIDR,
			RemotePodCIDR:        "",
			LocalNATExternalCIDR: externalCIDR,
			RemoteExternalCIDR:   "",
		}
	}
	clusterSubnets[clusterID] = subnets
	klog.Infof("Local NAT PodCIDR of cluster %s set to %s", clusterID, podCIDR)
	klog.Infof("Local NAT ExternalCIDR of cluster %s set to %s", clusterID, externalCIDR)

	// Push it in clusterSubnets
	if err := liqoIPAM.ipamStorage.updateClusterSubnets(clusterSubnets); err != nil {
		return fmt.Errorf("cannot update cluster subnets: %w", err)
	}

	// Init NAT mappings
	if err := liqoIPAM.initNatMappingsPerCluster(clusterID, subnets); err != nil {
		return fmt.Errorf("unable to initialize NAT mappings per cluster %s: %w", clusterID, err)
	}

	// Update natMappingsConfigured
	natMappingsConfigured[clusterID] = netv1alpha1.ConfiguredCluster{}
	if err := liqoIPAM.ipamStorage.updateNatMappingsConfigured(natMappingsConfigured); err != nil {
		return fmt.Errorf("unable to update NatMappingsConfigured: %w", err)
	}
	return nil
}

// RemoveLocalSubnetsPerCluster deletes networks related to a cluster.
func (liqoIPAM *IPAM) RemoveLocalSubnetsPerCluster(clusterID string) error {
	var exists bool
	var subnets netv1alpha1.Subnets

	// Get cluster subnets
	clusterSubnets := liqoIPAM.ipamStorage.getClusterSubnets()
	// Check existence
	subnets, exists = clusterSubnets[clusterID]
	if !exists || (subnets.LocalNATPodCIDR == "" && subnets.LocalNATExternalCIDR == "") {
		return nil
	}

	// Unset networks
	subnets.LocalNATPodCIDR = ""
	subnets.LocalNATExternalCIDR = ""
	clusterSubnets[clusterID] = subnets

	klog.Infof("Local NAT networks of cluster %s deleted", clusterID)
	if err := liqoIPAM.eventuallyDeleteClusterSubnet(clusterID, clusterSubnets); err != nil {
		return err
	}
	return nil
}

// GetExternalCIDR chooses and returns the local cluster's ExternalCIDR.
func (liqoIPAM *IPAM) GetExternalCIDR(mask uint8) (string, error) {
	var externalCIDR string
	var err error

	// Get cluster ExternalCIDR
	externalCIDR = liqoIPAM.ipamStorage.getExternalCIDR()
	if externalCIDR != "" {
		return externalCIDR, nil
	}
	if externalCIDR, err = liqoIPAM.getNetworkFromPool(mask); err != nil {
		return "", fmt.Errorf("cannot allocate an ExternalCIDR: %w", err)
	}
	if err := liqoIPAM.ipamStorage.updateExternalCIDR(externalCIDR); err != nil {
		_ = liqoIPAM.FreeReservedSubnet(externalCIDR)
		return "", fmt.Errorf("cannot update ExternalCIDR: %w", err)
	}
	return externalCIDR, nil
}

// Function that receives an IP and a network and returns true if
// the IP address does belong to the network.
func ipBelongsToNetwork(ip, network string) (bool, error) {
	// Parse network
	p, err := netaddr.ParseIPPrefix(network)
	if err != nil {
		return false, fmt.Errorf("cannot parse network: %w", err)
	}
	return p.Contains(netaddr.MustParseIP(ip)), nil
}

func (liqoIPAM *IPAM) belongsToPodCIDRInternal(ip string) (bool, error) {
	if netIP := net.ParseIP(ip); netIP == nil {
		return false, &liqoneterrors.WrongParameter{
			Reason:    liqoneterrors.ValidIP,
			Parameter: "Endpoint IP",
		}
	}

	podCIDR := liqoIPAM.ipamStorage.getPodCIDR()
	if podCIDR == "" {
		return false, fmt.Errorf("the pod CIDR is not set")
	}
	klog.V(5).Infof("BelongsToPodCIDR(%s): pod CIDR is %s", ip, podCIDR)

	return ipBelongsToNetwork(ip, podCIDR)
}

// BelongsToPodCIDR tells if the given IP belongs to the remote pod CIDR for the given cluster.
func (liqoIPAM *IPAM) BelongsToPodCIDR(ctx context.Context, belongsRequest *BelongsRequest) (*BelongsResponse, error) {
	belongs, err := liqoIPAM.belongsToPodCIDRInternal(belongsRequest.GetIp())
	if err != nil {
		return &BelongsResponse{}, fmt.Errorf("cannot tell if IP %s is in pod CIDR: %w", belongsRequest.GetIp(), err)
	}
	return &BelongsResponse{Belongs: belongs}, nil
}

/*
	mapIPToExternalCIDR acquires an IP belonging to the local ExternalCIDR for the specific IP and

if necessary maps it using the remoteExternalCIDR (this means remote cluster has remapped local ExternalCIDR)
Further invocations passing the same IP won't acquire a new IP, but will use the one already acquired.
*/
func (liqoIPAM *IPAM) mapIPToExternalCIDR(clusterID, remoteExternalCIDR, ip string) (string, error) {
	var externalCIDR string
	// Get endpointMappings
	endpointMappings := liqoIPAM.ipamStorage.getEndpointMappings()

	// Get local ExternalCIDR
	localExternalCIDR := liqoIPAM.ipamStorage.getExternalCIDR()

	if remoteExternalCIDR == consts.DefaultCIDRValue {
		externalCIDR = localExternalCIDR
	} else {
		externalCIDR = remoteExternalCIDR
	}

	// Check entry existence
	if _, exists := endpointMappings[ip]; !exists {
		// Create new entry
		ipamIP, err := liqoIPAM.ipam.AcquireIP(context.TODO(), localExternalCIDR)
		if err != nil {
			return "", fmt.Errorf("cannot allocate a new IP for endpoint %s: %w", ip, err)
		}
		endpointMappings[ip] = netv1alpha1.EndpointMapping{
			ExternalCIDROriginalIP: ipamIP.IP.String(),
			ClusterMappings:        make(map[string]netv1alpha1.ClusterMapping),
		}
		klog.Infof("%s has been acquired for endpoint %s", endpointMappings[ip].ExternalCIDROriginalIP, ip)
	}

	if _, exists := endpointMappings[ip].ClusterMappings[clusterID]; !exists {
		// Map IP if remote cluster has remapped local ExternalCIDR
		externalCIDRNattedIP, err := liqonetutils.MapIPToNetwork(externalCIDR, endpointMappings[ip].ExternalCIDROriginalIP)
		if err != nil {
			return "", fmt.Errorf("cannot map IP %s to network %s: %w", endpointMappings[ip].ExternalCIDROriginalIP, externalCIDR, err)
		}

		// setup clusterMappings
		endpointMappings[ip].ClusterMappings[clusterID] = netv1alpha1.ClusterMapping{ExternalCIDRNattedIP: externalCIDRNattedIP}
		klog.Infof("Endpoint %s has been remapped as %s", ip, externalCIDRNattedIP)

		// Update endpointMappings
		if err := liqoIPAM.ipamStorage.updateEndpointMappings(endpointMappings); err != nil {
			return "", fmt.Errorf("cannot update endpointMappings: %w", err)
		}

		// Add NAT mapping
		if err := liqoIPAM.natMappingInflater.AddMapping(ip, externalCIDRNattedIP, clusterID); err != nil {
			return "", fmt.Errorf("cannot add NAT mapping: %w", err)
		}
	}

	return endpointMappings[ip].ClusterMappings[clusterID].ExternalCIDRNattedIP, nil
}

/*
	mapEndpointIPInternal is the internal implementation of MapEndpointIP gRPC.

If the received IP belongs to local PodCIDR, then it maps the address in the traditional way,
i.e. using the network used in the remote cluster for local PodCIDR.
If the received IP does not belong to local PodCIDR, then it maps the address using the ExternalCIDR.
*/
func (liqoIPAM *IPAM) mapEndpointIPInternal(clusterID, ip string) (string, error) {
	var subnets netv1alpha1.Subnets
	var exists bool

	err := validateEndpointMappingInputs(clusterID, ip)
	if err != nil {
		return "", err
	}

	liqoIPAM.mutex.Lock()
	defer liqoIPAM.mutex.Unlock()

	// Get cluster subnets
	clusterSubnets := liqoIPAM.ipamStorage.getClusterSubnets()
	subnets, exists = clusterSubnets[clusterID]
	if !exists {
		return "", fmt.Errorf("cluster %s has not a network configuration", clusterID)
	}

	// Get PodCIDR
	podCIDR := liqoIPAM.ipamStorage.getPodCIDR()
	if podCIDR == emptyCIDR {
		return "", fmt.Errorf("cannot get cluster PodCIDR: %w", err)
	}

	belongs, err := ipBelongsToNetwork(ip, podCIDR)
	if err != nil {
		return "", fmt.Errorf("cannot establish if IP %s belongs to PodCIDR: %w", ip, err)
	}
	if belongs {
		klog.V(5).Infof("MapEndpointIP(%s, %s): ip is in pod CIDR %s, mapping to LocalNATPodCIDR %s",
			ip, clusterID, podCIDR, subnets.LocalNATPodCIDR)

		/* IP belongs to local PodCIDR, this means the Pod is a local Pod and
		the new IP should belong to the network used in the remote cluster
		for local Pods: this can be either the cluster PodCIDR or a different network */
		newIP, err := liqonetutils.MapIPToNetwork(subnets.LocalNATPodCIDR, ip)
		if err != nil {
			return "", fmt.Errorf("cannot map endpoint IP %s to PodCIDR of remote cluster %s: %w", ip, clusterID, err)
		}
		return newIP, nil
	}
	// IP does not belong to cluster PodCIDR: Pod is a reflected Pod
	klog.V(5).Infof("MapEndpointIP(%s, %s): ip is not in pod CIDR %s, mapping to LocalNATExternalCIDR %s",
		ip, clusterID, podCIDR, subnets.LocalNATExternalCIDR)

	// Map IP to ExternalCIDR
	newIP, err := liqoIPAM.mapIPToExternalCIDR(clusterID, subnets.LocalNATExternalCIDR, ip)
	if err != nil {
		return "", fmt.Errorf("cannot map endpoint IP %s to ExternalCIDR of cluster %s: %w", ip, clusterID, err)
	}

	return newIP, nil
}

// MapEndpointIP receives a service endpoint IP and a cluster identifier and,
// if the endpoint IP does not belong to cluster PodCIDR, maps
// the endpoint IP to a new IP taken from the remote ExternalCIDR of the remote cluster.
func (liqoIPAM *IPAM) MapEndpointIP(ctx context.Context, mapRequest *MapRequest) (*MapResponse, error) {
	mappedIP, err := liqoIPAM.mapEndpointIPInternal(mapRequest.GetClusterID(), mapRequest.GetIp())
	if err != nil {
		return &MapResponse{}, fmt.Errorf("cannot map endpoint IP to ExternalCIDR of cluster %s, %w",
			mapRequest.GetClusterID(), err)
	}
	return &MapResponse{Ip: mappedIP}, nil
}

func validateEndpointMappingInputs(clusterID, ip string) error {
	const emptyClusterID = ""
	// Parse IP
	if netIP := net.ParseIP(ip); netIP == nil {
		return &liqoneterrors.WrongParameter{
			Reason:    liqoneterrors.ValidIP,
			Parameter: "Endpoint IP",
		}
	}

	if clusterID == emptyClusterID {
		return &liqoneterrors.WrongParameter{
			Reason:    liqoneterrors.StringNotEmpty,
			Parameter: consts.ClusterIDLabelName,
		}
	}
	return nil
}

// GetHomePodIP receives a Pod IP valid in the remote cluster and returns the corresponding home Pod IP
// (i.e. with validity in home cluster).
func (liqoIPAM *IPAM) GetHomePodIP(ctx context.Context, request *GetHomePodIPRequest) (*GetHomePodIPResponse, error) {
	homeIP, err := liqoIPAM.getHomePodIPInternal(request.GetClusterID(), request.GetIp())
	if err != nil {
		return &GetHomePodIPResponse{}, fmt.Errorf("cannot get home Pod IP starting from IP %s: %w",
			request.GetIp(), err)
	}
	return &GetHomePodIPResponse{HomeIP: homeIP}, nil
}

// Internal implementation of exported func GetHomePodIP.
func (liqoIPAM *IPAM) getHomePodIPInternal(clusterID, ip string) (string, error) {
	if clusterID == "" {
		return "", &liqoneterrors.WrongParameter{
			Parameter: consts.ClusterIDLabelName,
			Reason:    liqoneterrors.StringNotEmpty,
		}
	}
	if parsedIP := net.ParseIP(ip); parsedIP == nil {
		return "", &liqoneterrors.WrongParameter{
			Reason:    liqoneterrors.ValidIP,
			Parameter: ip,
		}
	}

	liqoIPAM.mutex.Lock()
	defer liqoIPAM.mutex.Unlock()

	// Get cluster subnets
	clusterSubnets := liqoIPAM.ipamStorage.getClusterSubnets()
	subnets, exists := clusterSubnets[clusterID]

	// Check if RemotePodCIDR is set
	if !exists {
		return "", fmt.Errorf("cluster %s subnets are not set", clusterID)
	}

	if subnets.RemotePodCIDR == "" {
		return "", &liqoneterrors.WrongParameter{
			Reason: liqoneterrors.StringNotEmpty,
		}
	}

	klog.V(5).Infof("GetHomePodIP(%s, %s): mapping to RemotePodCIDR %s",
		ip, clusterID, subnets.RemotePodCIDR)
	return liqonetutils.MapIPToNetwork(subnets.RemotePodCIDR, ip)
}

// unmapEndpointIPInternal is the internal implementation of UnmapEndpointIP.
// If the endpointIP is not reflected anymore in any remote cluster, then it frees the corresponding ExternalCIDR IP.
func (liqoIPAM *IPAM) unmapEndpointIPInternal(clusterID, endpointIP string) error {
	var exists bool

	err := validateEndpointMappingInputs(clusterID, endpointIP)
	if err != nil {
		return err
	}

	liqoIPAM.mutex.Lock()
	defer liqoIPAM.mutex.Unlock()

	// Get endpointMappings
	endpointMappings := liqoIPAM.ipamStorage.getEndpointMappings()

	// Get local ExternalCIDR
	localExternalCIDR := liqoIPAM.ipamStorage.getExternalCIDR()
	if localExternalCIDR == emptyCIDR {
		return fmt.Errorf("cannot get ExternalCIDR: %w", err)
	}

	endpointMapping, exists := endpointMappings[endpointIP]
	if !exists {
		// a. 	the entry does not exists because the endpointIP is an IP
		//		belonging to the local PodCIDR, therefore there is no need of do nothing.
		// b. 	the entry does not exists because it was already deleted, same as above.
		return nil
	}

	klog.Infof("endpoint IP %s: removed %s for cluster %s", endpointIP, endpointMapping.ClusterMappings[clusterID].ExternalCIDRNattedIP, clusterID)
	delete(endpointMapping.ClusterMappings, clusterID)

	if len(endpointMapping.ClusterMappings) == 0 {
		// Free IP
		err = liqoIPAM.ipam.ReleaseIPFromPrefix(context.TODO(), localExternalCIDR, endpointMapping.ExternalCIDROriginalIP)
		if err != nil && !errors.Is(err, goipam.ErrNotFound) {
			/*
				ReleaseIPFromPrefix can return ErrNotFound either if the prefix
					is not found and if the IP is not allocated.
					Since the prefix represents the ExternalCIDR, whose existence has
					been checked some lines above, ReleaseIPFromPrefix returns
					ErrNotFound if the IP has not been allocated or has already been freed.
			*/
			return fmt.Errorf("cannot free IP: %w", err)
		}
		if err == nil {
			klog.Infof("IP %s (mapped from %s) has been freed", endpointMapping.ExternalCIDROriginalIP, endpointIP)
		}

		delete(endpointMappings, endpointIP)
	} else {
		endpointMappings[endpointIP] = endpointMapping
	}

	// Push update
	if err := liqoIPAM.ipamStorage.updateEndpointMappings(endpointMappings); err != nil {
		return fmt.Errorf("cannot update endpointIPs: %w", err)
	}

	// Remove NAT mapping
	if err := liqoIPAM.natMappingInflater.RemoveMapping(endpointIP, clusterID); err != nil {
		return err
	}
	return nil
}

// UnmapEndpointIP set the endpoint as unused for a specific cluster.
func (liqoIPAM *IPAM) UnmapEndpointIP(ctx context.Context, unmapRequest *UnmapRequest) (*UnmapResponse, error) {
	err := liqoIPAM.unmapEndpointIPInternal(unmapRequest.GetClusterID(), unmapRequest.GetIp())
	if err != nil {
		return &UnmapResponse{}, fmt.Errorf("cannot unmap the IP of endpoint %s: %w", unmapRequest.GetIp(), err)
	}
	return &UnmapResponse{}, nil
}

// SetPodCIDR sets the PodCIDR.
func (liqoIPAM *IPAM) SetPodCIDR(podCIDR string) error {
	// Get PodCIDR
	oldPodCIDR := liqoIPAM.ipamStorage.getPodCIDR()
	if oldPodCIDR != "" && oldPodCIDR != podCIDR {
		return fmt.Errorf("trying to change PodCIDR")
	}
	if oldPodCIDR != "" && oldPodCIDR == podCIDR {
		return nil
	}
	// Acquire PodCIDR
	if err := liqoIPAM.AcquireReservedSubnet(podCIDR); err != nil {
		return fmt.Errorf("cannot acquire PodCIDR: %w", err)
	}
	// Update PodCIDR
	if err := liqoIPAM.ipamStorage.updatePodCIDR(podCIDR); err != nil {
		return fmt.Errorf("cannot set PodCIDR: %w", err)
	}
	return nil
}

// SetServiceCIDR sets the ServiceCIDR.
func (liqoIPAM *IPAM) SetServiceCIDR(serviceCIDR string) error {
	// Get ServiceCIDR
	oldServiceCIDR := liqoIPAM.ipamStorage.getServiceCIDR()
	if oldServiceCIDR != "" && oldServiceCIDR != serviceCIDR {
		return fmt.Errorf("trying to change ServiceCIDR")
	}
	if oldServiceCIDR != "" && oldServiceCIDR == serviceCIDR {
		return nil
	}
	// Acquire Service CIDR
	if err := liqoIPAM.AcquireReservedSubnet(serviceCIDR); err != nil {
		return fmt.Errorf("cannot acquire ServiceCIDR: %w", err)
	}
	// Update Service CIDR
	if err := liqoIPAM.ipamStorage.updateServiceCIDR(serviceCIDR); err != nil {
		return fmt.Errorf("cannot set ServiceCIDR: %w", err)
	}
	return nil
}

// SetReservedSubnets acquires and/or frees the reserved networks.
func (liqoIPAM *IPAM) SetReservedSubnets(subnets []string) error {
	reserved := liqoIPAM.ipamStorage.getReservedSubnets()

	// Free all the reserved networks not needed anymore.
	for _, r := range reserved {
		if !slice.ContainsString(subnets, r) {
			klog.Infof("freeing old reserved subnet %s", r)
			if err := liqoIPAM.FreeReservedSubnet(r); err != nil {
				return fmt.Errorf("an error occurred while freeing reserved subnet {%s}: %w", r, err)
			}
			if err := liqoIPAM.ipamStorage.updateReservedSubnets(r, updateOpRemove); err != nil {
				return err
			}
		}
	}
	// Get the reserved subnets after we have freed the old ones.
	reserved = liqoIPAM.ipamStorage.getReservedSubnets()

	// Enforce the reserved subnets. Being the reservation a two-step process,
	// it could happen that a subnet is added to the reserved list but not
	// reserved due to an error. So we make sure that all the subnets in the
	// reserved list have been acquired.
	// We are sure that if a reserved network has been added to the reserved list
	// the prefix for that network is free or has been already acquired on behalf
	// of the current reserved network.
	for _, rSubnet := range reserved {
		if err := liqoIPAM.MarkAsAcquiredReservedSubnet(rSubnet); err != nil {
			return fmt.Errorf("an error occurred while enforcing reserved subnet {%s}: %w", rSubnet, err)
		}
	}

	// Reserve the newly added subnets.
	for _, s := range subnets {
		if slice.ContainsString(reserved, s) {
			continue
		}
		klog.Infof("acquiring reserved subnet %s", s)
		// Check if the subnet does not overlap with the existing reserved subnets.
		if err := liqoIPAM.reservedSubnetOverlaps(s); err != nil {
			return err
		}

		if err := liqoIPAM.ipamStorage.updateReservedSubnets(s, updateOpAdd); err != nil {
			return err
		}
		if err := liqoIPAM.MarkAsAcquiredReservedSubnet(s); err != nil {
			return fmt.Errorf("an error occurred while reserving subnet {%s}: %w", s, err)
		}
	}
	return nil
}

func (liqoIPAM *IPAM) reservedSubnetOverlaps(subnet string) error {
	// Check if subnet overlaps with local pod CIDR.
	podCidr := liqoIPAM.ipamStorage.getPodCIDR()
	overlaps, err := liqoIPAM.overlapsWithNetwork(subnet, podCidr)
	if err != nil {
		return err
	}
	if overlaps {
		return fmt.Errorf("network %s cannot be reserved because it overlaps with the local podCIDR %s",
			subnet, podCidr)
	}

	// Check if subnet overlaps with local service CIDR.
	serviceCidr := liqoIPAM.ipamStorage.getServiceCIDR()
	overlaps, err = liqoIPAM.overlapsWithNetwork(subnet, serviceCidr)
	if err != nil {
		return err
	}
	if overlaps {
		return fmt.Errorf("network %s cannot be reserved because it overlaps with the local serviceCIDR %s",
			subnet, serviceCidr)
	}

	// Check if subnet overlaps with local external CIDR.
	externalCidr := liqoIPAM.ipamStorage.getExternalCIDR()
	overlaps, err = liqoIPAM.overlapsWithNetwork(subnet, externalCidr)
	if err != nil {
		return err
	}
	if overlaps {
		return fmt.Errorf("network %s cannot be reserved because it overlaps with the local external CIDR %s",
			subnet, externalCidr)
	}

	// Check if the subnet does not overlap with the existing reserved subnets.
	overlappingNet, overlaps, err := liqoIPAM.overlapsWithReserved(subnet)
	if err != nil {
		return err
	}
	if overlaps {
		return fmt.Errorf("network %s cannot be reserved because it overlaps with the reserved network %s",
			subnet, overlappingNet)
	}

	// Check if the subnet does not overlap wit the existing cluster subnets.
	overlappingNet, overlaps, err = liqoIPAM.overlapsWithCluster(subnet)
	if err != nil {
		return err
	}
	if overlaps {
		return fmt.Errorf("network %s cannot be reserved because it overlaps with the reserved network %s",
			subnet, overlappingNet)
	}

	return nil
}

// AcquireSpecificIP acquires the first IP in the given subnet and return it.
// This function returns nil if the IP is already acquired.
func (liqoIPAM *IPAM) AcquireSpecificIP(ip, subnet string) error {
	r, err := liqoIPAM.ipam.AcquireSpecificIP(context.Background(), subnet, ip)
	if err != nil && r != nil {
		return err
	}
	return nil
}

// SetSpecificNatMapping sets a specific NAT mapping.
func (liqoIPAM *IPAM) SetSpecificNatMapping(newIPLocal, newIP, oldIP, clusterID string) error {
	endpointMappings := liqoIPAM.ipamStorage.getEndpointMappings()

	// Check entry existence
	if _, exists := endpointMappings[oldIP]; !exists {
		endpointMappings[oldIP] = netv1alpha1.EndpointMapping{
			ExternalCIDROriginalIP: newIPLocal,
			ClusterMappings:        make(map[string]netv1alpha1.ClusterMapping),
		}
	}

	// Update clusterMappings
	endpointMappings[oldIP].ClusterMappings[clusterID] = netv1alpha1.ClusterMapping{ExternalCIDRNattedIP: newIP}

	// Update endpointMappings
	if err := liqoIPAM.ipamStorage.updateEndpointMappings(endpointMappings); err != nil {
		return err
	}

	// Add NAT mapping
	klog.Infof("setting DNAT mapping for IP %s to %s", newIP, oldIP)
	if err := liqoIPAM.natMappingInflater.AddMapping(oldIP, newIP, clusterID); err != nil {
		return err
	}
	return nil
}

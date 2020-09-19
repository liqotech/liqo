package liqonet

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestIpManager_GetNewSubnetPerCluster(t *testing.T) {
	//init ipam
	ipam := IpManager{
		UsedSubnets:        make(map[string]*net.IPNet),
		FreeSubnets:        make(map[string]*net.IPNet),
		ConflictingSubnets: make(map[string]*net.IPNet),
		SubnetPerCluster:   make(map[string]*net.IPNet),
	}
	err := ipam.Init()
	assert.Nil(t, err, "should be nil")
	//test without conflicting
	//expect the same net to be returned and error to be nil
	clusterID := "test1"
	_, clusterSubnet, err := net.ParseCIDR("10.1.0.0/16")
	assert.Nil(t, err, "error should be nil")
	newSubnet, err := ipam.GetNewSubnetPerCluster(clusterSubnet, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, clusterSubnet.String(), newSubnet.String(), "should be equal")
	_, exists := ipam.UsedSubnets[newSubnet.String()]
	assert.True(t, exists)
	reserved, exists := ipam.SubnetPerCluster[clusterID]
	assert.True(t, exists)
	assert.Equal(t, newSubnet, reserved, "should be equal")
	overlap := VerifyNoOverlap(ipam.FreeSubnets, newSubnet)
	assert.False(t, overlap, "should be true")
	overlap = VerifyNoOverlap(ipam.FreeSubnets, clusterSubnet)
	assert.False(t, overlap, "should be true")

	//test2 with conflicting subnets
	//expecting a new net to be returned and error to be nil
	clusterID = "test2"
	_, clusterSubnet, err = net.ParseCIDR("10.1.0.0/16")
	assert.Nil(t, err, "error should be nil")
	newSubnet, err = ipam.GetNewSubnetPerCluster(clusterSubnet, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.NotEqual(t, clusterSubnet.String(), newSubnet.String(), "should be different")
	_, exists = ipam.UsedSubnets[newSubnet.String()]
	assert.True(t, exists)
	reserved, exists = ipam.SubnetPerCluster[clusterID]
	assert.True(t, exists)
	assert.Equal(t, newSubnet.String(), reserved.String(), "should be equal")
	overlap = VerifyNoOverlap(ipam.FreeSubnets, newSubnet)
	assert.False(t, overlap, "should be false")
	overlap = VerifyNoOverlap(ipam.FreeSubnets, clusterSubnet)
	assert.False(t, overlap, "should be false")

	//test3 requiring a new address for a cluster that we already have processed
	//expecting to receive the already allocated address and error to be nil
	alreadyAssignedSubnet, err := ipam.GetNewSubnetPerCluster(clusterSubnet, clusterID)
	assert.Nil(t, err, "error should be nil")
	//check that is equal to the one assigned before
	assert.Equal(t, newSubnet.String(), alreadyAssignedSubnet.String(), "should be equal")
	assert.NotEqual(t, clusterSubnet.String(), alreadyAssignedSubnet.String(), "should be different")
	_, exists = ipam.UsedSubnets[alreadyAssignedSubnet.String()]
	assert.True(t, exists)
	reserved, exists = ipam.SubnetPerCluster[clusterID]
	assert.True(t, exists)
	assert.Equal(t, alreadyAssignedSubnet.String(), reserved.String(), "should be equal")
	overlap = VerifyNoOverlap(ipam.FreeSubnets, alreadyAssignedSubnet)
	assert.False(t, overlap, "should be false")
	overlap = VerifyNoOverlap(ipam.FreeSubnets, clusterSubnet)
	assert.False(t, overlap, "should be false")

	//test4 no more subnets available in the free pool
	//the cluster subnet does not need to be NATed
	//expect the same subnet to be returned and error to be nil
	clusterID = "test4"
	_, clusterSubnet, err = net.ParseCIDR("10.3.0.0/16")
	assert.Nil(t, err, "should be nil")
	ipam.FreeSubnets = map[string]*net.IPNet{}
	newSubnet, err = ipam.GetNewSubnetPerCluster(clusterSubnet, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, clusterSubnet.String(), newSubnet.String(), "should be equal")
	_, exists = ipam.UsedSubnets[newSubnet.String()]
	assert.True(t, exists)
	reserved, exists = ipam.SubnetPerCluster[clusterID]
	assert.True(t, exists)
	assert.Equal(t, newSubnet, reserved, "should be equal")

	//test5 no more subnets available in the free pool
	//the cluster subnet needs to be NATed
	//expect subnet to be nil and an error to be returned
	clusterID = "test5"
	_, clusterSubnet, err = net.ParseCIDR("10.3.0.0/16")
	assert.Nil(t, err, "should be nil")
	ipam.FreeSubnets = map[string]*net.IPNet{}
	_, err = ipam.GetNewSubnetPerCluster(clusterSubnet, clusterID)
	assert.NotNil(t, err, "should be not nil")
}

func TestIpManager_RemoveReservedSubnet(t *testing.T) {
	//init ipam
	ipam := IpManager{
		UsedSubnets:        make(map[string]*net.IPNet),
		FreeSubnets:        make(map[string]*net.IPNet),
		ConflictingSubnets: make(map[string]*net.IPNet),
		SubnetPerCluster:   make(map[string]*net.IPNet),
	}
	err := ipam.Init()
	assert.Nil(t, err, "should be nil")

	//test1 we reserve a subnet for a peering cluster
	//expecting the reserved subnet to be made again available
	clusterID := "test1"
	_, clusterSubnet, err := net.ParseCIDR("10.1.0.0/16")
	assert.Nil(t, err, "error should be nil")
	newSubnet, err := ipam.GetNewSubnetPerCluster(clusterSubnet, clusterID)
	assert.Nil(t, err, "error should be nil")
	ipam.RemoveReservedSubnet(clusterID)
	_, exists := ipam.UsedSubnets[newSubnet.String()]
	assert.False(t, exists)
	_, exists = ipam.SubnetPerCluster[clusterID]
	assert.False(t, exists)

	//test2 we try to free a reserved subnet for a cluster that we did not processed
	clusterID = "test2"
	ipam.RemoveReservedSubnet(clusterID)
	_, exists = ipam.UsedSubnets[newSubnet.String()]
	assert.False(t, exists)
	_, exists = ipam.SubnetPerCluster[clusterID]
	assert.False(t, exists)
}

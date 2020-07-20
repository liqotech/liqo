package liqonet

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	controller "github.com/liqoTech/liqo/internal/liqonet"
	liqonetOperator "github.com/liqoTech/liqo/pkg/liqonet"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"net"
	"sync"
	"testing"
)

func getTunnelEndpointCreator() *controller.TunnelEndpointCreator {
	return &controller.TunnelEndpointCreator{
		Client:          nil,
		Log:             nil,
		Scheme:          nil,
		ReservedSubnets: make(map[string]*net.IPNet),
		IPManager: liqonetOperator.IpManager{
			UsedSubnets:        make(map[string]*net.IPNet),
			FreeSubnets:        make(map[string]*net.IPNet),
			ConflictingSubnets: make(map[string]*net.IPNet),
			SubnetPerCluster:   nil,
			Log:                nil,
		},
		Mutex:        sync.Mutex{},
		IsConfigured: false,
		Configured:   nil,
		RetryTimeout: 0,
	}
}

func getClusterConfigurationCR(reservedSubnets []string) *policyv1.ClusterConfig {
	return &policyv1.ClusterConfig{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: policyv1.ClusterConfigSpec{
			AdvertisementConfig: policyv1.AdvertisementConfig{},
			DiscoveryConfig:     policyv1.DiscoveryConfig{},
			LiqonetConfig: policyv1.LiqonetConfig{
				ReservedSubnets:  reservedSubnets,
				GatewayPrivateIP: "",
				VxlanNetConfig:   liqonetOperator.VxlanNetConfig{},
			},
		},
		Status: policyv1.ClusterConfigStatus{},
	}
}

func setupConfig(reservedSubnets, clusterSubnets []string) (*controller.TunnelEndpointCreator, error) {
	clusterSubnetsMap, err := convertSliceToMap(clusterSubnets)
	if err != nil {
		return nil, err
	}
	reservedSubnetsMap, err := convertSliceToMap(reservedSubnets)
	if err != nil {
		return nil, err
	}
	tep := getTunnelEndpointCreator()
	err = tep.InitConfiguration(reservedSubnetsMap, clusterSubnetsMap)
	return tep, err
}

func convertSliceToMap(subnets []string) (map[string]*net.IPNet, error) {
	var mapSubnets = map[string]*net.IPNet{}
	for _, subnet := range subnets {
		_, sn, err := net.ParseCIDR(subnet)
		if err != nil {
			klog.Errorf("an error occurred while parsing configuration: %s", err)
			return nil, err
		} else {
			klog.Infof("subnet %s correctly added to the reserved subnets", sn.String())
			mapSubnets[sn.String()] = sn
		}
	}
	return mapSubnets, nil
}

func TestGetConfiguration(t *testing.T) {
	goodReservedSubnets := []string{"10.24.0.0/16", "10.96.0.0/12", "10.250.250.0/30"}
	badReservedSubnets := []string{"10.24.0/16", "10.96.0.0/12", "10.250.250.0/30"}

	tests := []struct {
		good            bool
		reservedSubnets []string
	}{
		{true, goodReservedSubnets},
		{false, badReservedSubnets},
	}
	tep := getTunnelEndpointCreator()

	for _, test := range tests {
		if test.good {
			clusterConfig := getClusterConfigurationCR(test.reservedSubnets)
			configFetched, err := tep.GetConfiguration(clusterConfig)
			assert.Nil(t, err, "error should be nil")
			assert.Equal(t, len(test.reservedSubnets), len(configFetched), "the number of reserved subnets should be the same in input and output")
			for _, subnet := range test.reservedSubnets {
				_, ok := configFetched[subnet]
				assert.Equal(t, ok, true, "subnet %s is present", subnet)
			}
		} else {
			clusterConfig := getClusterConfigurationCR(test.reservedSubnets)
			_, err := tep.GetConfiguration(clusterConfig)
			assert.Error(t, err, "error should be not nil")
		}
	}
}

func TestInitConfiguration(t *testing.T) {
	tests := []struct {
		clusterSubnets  []string
		reservedSubnets []string
		testing         string
	}{
		{[]string{"192.168.1.0/24", "192.168.4.0/28", "10.0.0.0/16"}, []string{"192.168.3.0/24", "172.16.6.0/22", "172.16.2.0/24", "10.96.0.0/12"}, "Overlapping"},
		{[]string{"10.24.0.0/16", "10.96.0.0/12", "10.250.250.0/30"}, []string{"10.0.0.0/16", "10.95.0.0/16", "10.250.250.0/24", "172.16.2.0/24"}, "Conflicting"},
		{[]string{"192.168.1.0/24", "192.168.4.0/28"}, []string{"192.168.3.0/24", "172.16.6.0/22", "172.16.2.0/24"}, "NoOverlapping"},
	}
	for _, test := range tests {
		clusterSubnetsMap, err := convertSliceToMap(test.clusterSubnets)
		assert.Nil(t, err, "should be nil, otherwise check the subnets provided as a test")
		reservedSubnetsMap, err := convertSliceToMap(test.reservedSubnets)
		assert.Nil(t, err, "should be nil, otherwise check the subnets provided as a test")
		switch test.testing {
		case "Conflicting":
			//case 1: clusterSubnets and reservedSubnets address spaces have conflicts
			//the function should return an error
			tep := getTunnelEndpointCreator()
			err = tep.InitConfiguration(reservedSubnetsMap, clusterSubnetsMap)
			assert.NotNil(t, err, "it should be not nil, because there are conflicts between the already used subnets and reserved ones")

		case "NoOverlapping":
			//case 2: clusterSubnets and reservedSubnets address spaces does not have conflicts between them and at the same time are not in the 10.0.0.0/8 CIDR block
			//the function should return nil
			//conflictingSubnets should be empty
			//all the subnets provided should be in the usedSubnets map
			tep := getTunnelEndpointCreator()
			err = tep.InitConfiguration(reservedSubnetsMap, clusterSubnetsMap)
			assert.Nil(t, err, "should be nil")
			assert.Equal(t, 0, len(tep.IPManager.ConflictingSubnets), "the length of the conflicting subnets should be 0")
			for sn := range clusterSubnetsMap {
				_, ok := tep.IPManager.UsedSubnets[sn]
				assert.Equal(t, true, ok, "subnet %s should be present in UsedSubnets", sn)
			}
			for sn := range reservedSubnetsMap {
				_, ok := tep.IPManager.UsedSubnets[sn]
				assert.Equal(t, true, ok, "subnet %s should be present in UsedSubnets", sn)
			}

		case "Overlapping":
			//case 3: clusterSubnetsand and reserved subnets address spaces does not conflict between them but some of them belongs to the 10.0.0.0/8 CIDR block
			//the function should return nil
			//conflictingSubnets should have some subnets based on the input, in this case the 10.0.0.0/16 generates a conflicts with only the 10.0.0.0/16 subnet,
			//the 10.96.0.0/12 generates 16 conflicts with sixteen 10.x.x.x/16 address spaces belonging to the 10.0.0.0/8: total 16 + 1 = 17
			//all the subnets provided should be in the used subnets map
			tep := getTunnelEndpointCreator()
			err = tep.InitConfiguration(reservedSubnetsMap, clusterSubnetsMap)
			assert.Nil(t, err, "should be nil")
			assert.Equal(t, 17, len(tep.IPManager.ConflictingSubnets), "the length of the conflicting subnets should be 0")
			for sn := range clusterSubnetsMap {
				_, ok := tep.IPManager.UsedSubnets[sn]
				assert.Equal(t, true, ok, "subnet %s should be present in UsedSubnets", sn)
			}
			for sn := range reservedSubnetsMap {
				_, ok := tep.IPManager.UsedSubnets[sn]
				assert.Equal(t, true, ok, "subnet %s should be present in UsedSubnets", sn)
			}
		}
	}
}

func TestUpdateConfiguration(t *testing.T) {
	tests := []struct {
		clusterSubnets         []string
		reservedSubnets        []string
		updatedReservedSubnets []string
		testing                string
	}{
		{[]string{"192.168.1.0/24", "192.168.4.0/28", "10.0.0.0/16"}, []string{"192.168.3.0/24", "172.16.6.0/22", "172.16.2.0/24", "10.96.0.0/12"}, []string{"192.168.3.0/24", "172.16.6.0/22", "172.16.2.0/24"}, "RemovingReservedSubnet"},
		{[]string{"192.168.1.0/24", "192.168.4.0/28", "10.10.0.0/16"}, []string{"192.168.3.0/24", "172.16.6.0/22", "172.16.2.0/24", "10.96.0.0/12"}, []string{"192.168.3.0/24", "172.16.6.0/22", "172.16.2.0/24", "10.96.0.0/12", "10.10.0.0/16", "10.250.0.0/30"}, "AddingReservedSubnetWithConflict"},
		{[]string{"192.168.1.0/24", "192.168.4.0/28"}, []string{"192.168.3.0/24", "172.16.6.0/22", "172.16.2.0/24", "10.0.0.0/12"}, []string{"192.168.3.0/24", "172.16.6.0/22", "172.16.2.0/24", "10.0.0.0/12", "10.250.0.0/16"}, "AddingReservedSubnetWithNoConflict"},
	}
	for _, test := range tests {
		tep, err := setupConfig(test.reservedSubnets, test.clusterSubnets)
		assert.Nil(t, err, "should be nil, otherwise check the subnets given in input to test the function")
		switch test.testing {
		case "RemovingReservedSubnet":
			/*we remove a previously reserved subnet: 10.96.0.0/12
			**the error should be nil
			**the conflicting ips with the removed subnet are now allocatable so the length of ConflictingSubnets should be 1
			**the subnet shouldn't be anymore in the UsedSubnets and in ReservedSubnets
			 */
			updatedReservedSubnet, err := convertSliceToMap(test.updatedReservedSubnets)
			assert.Nil(t, err, "should be nil, otherwise check the updatedReservedSubnet values")
			err = tep.UpdateConfiguration(updatedReservedSubnet)
			assert.Nil(t, err, "should be nil")
			assert.Equal(t, 1, len(tep.IPManager.ConflictingSubnets), "conflictinqSubnet should be empty")
			_, ok := tep.IPManager.UsedSubnets["10.96.0.0/12"]
			assert.Equal(t, false, ok, "should be false")
			_, ok = tep.ReservedSubnets["10.96.0.0/12"]
			assert.Equal(t, false, ok, "should be false")

		case "AddingReservedSubnetWithConflict":
			/*we add two new subnet to reservedSubnets which only one have conflicts with the subnets used by the clusters
			**the function returns nil but warns the user when processing the conflicting subnet
			**the conflicting subnet shouldn't be added to the usedSubnets or to the reservedSubnets
			**the othe subnet should be added to the usedSubnets and to reservedSubnets
			 */
			updatedReservedSubnet, err := convertSliceToMap(test.updatedReservedSubnets)
			assert.Nil(t, err, "should be nil, otherwise check the updatedReservedSubnet values")
			err = tep.UpdateConfiguration(updatedReservedSubnet)
			assert.Nil(t, err, "should be not nil")
			_, ok := tep.IPManager.UsedSubnets["10.100.0.0/24"]
			assert.Equal(t, false, ok, "should be false")
			_, ok = tep.ReservedSubnets["10.100.0.0/24"]
			assert.Equal(t, false, ok, "should be false")
			_, ok = tep.IPManager.UsedSubnets["10.250.0.0/30"]
			assert.Equal(t, true, ok, "should be true")
			_, ok = tep.ReservedSubnets["10.250.0.0/30"]
			assert.Equal(t, true, ok, "should be true")
			overlapping := liqonetOperator.VerifyNoOverlap(tep.IPManager.FreeSubnets, updatedReservedSubnet["10.250.0.0/30"])
			assert.Equal(t, false, overlapping)

		case "AddingReservedSubnetWithNoConflict":
			/*we add a new subnet to reservedSubnets which does not have conflicts with the subnets used by the clusters
			**the function returns nil
			**the new subnet should be added to the usedSubnets and to the reservedSubnets
			**no conflicts should be present with the subnets in the freeSubnets map
			 */
			updatedReservedSubnet, err := convertSliceToMap(test.updatedReservedSubnets)
			assert.Nil(t, err, "should be nil, otherwise check the updatedReservedSubnet values")
			err = tep.UpdateConfiguration(updatedReservedSubnet)
			assert.Nil(t, err, "should be not nil")
			_, ok := tep.IPManager.UsedSubnets["10.250.0.0/16"]
			assert.Equal(t, true, ok, "should be true")
			_, ok = tep.ReservedSubnets["10.250.0.0/16"]
			assert.Equal(t, true, ok, "should be true")
			overlapping := liqonetOperator.VerifyNoOverlap(tep.IPManager.FreeSubnets, updatedReservedSubnet["10.250.0.0/16"])
			assert.Equal(t, false, overlapping)
		}
	}
}

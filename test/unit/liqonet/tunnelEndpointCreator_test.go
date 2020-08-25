package liqonet

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	v1 "github.com/liqoTech/liqo/api/liqonet/v1"
	controller "github.com/liqoTech/liqo/internal/liqonet"
	"github.com/liqoTech/liqo/pkg/liqonet"
	liqonetOperator "github.com/liqoTech/liqo/pkg/liqonet"
	"github.com/stretchr/testify/assert"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"net"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"testing"
	"time"
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
				ReservedSubnets: reservedSubnets,
				PodCIDR:         "",
				VxlanNetConfig:  liqonetOperator.VxlanNetConfig{},
			},
		},
		Status: policyv1.ClusterConfigStatus{},
	}
}

func setupTunnelEndpointCreatorOperator() error {
	var err error
	tunEndpointCreator = &controller.TunnelEndpointCreator{
		Client:          k8sManager.GetClient(),
		Log:             ctrl.Log.WithName("controllers").WithName("TunnelEndpointCreator"),
		Scheme:          k8sManager.GetScheme(),
		ReservedSubnets: make(map[string]*net.IPNet),
		Configured:      make(chan bool, 1),
		IPManager: liqonet.IpManager{
			UsedSubnets:        make(map[string]*net.IPNet),
			FreeSubnets:        make(map[string]*net.IPNet),
			SubnetPerCluster:   make(map[string]*net.IPNet),
			ConflictingSubnets: make(map[string]*net.IPNet),
			Log:                ctrl.Log.WithName("IPAM"),
		},
		RetryTimeout: 30 * time.Second,
	}
	config := k8sManager.GetConfig()
	newConfig := &rest.Config{
		Host: config.Host,
		// gotta go fast during tests -- we don't really care about overwhelming our test API server
		QPS:   1000.0,
		Burst: 2000.0,
	}
	tunEndpointCreator.WatchConfiguration(newConfig, &policyv1.GroupVersion)
	err = tunEndpointCreator.SetupWithManager(k8sManager)
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	return nil
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

func TestGetTunEndPerADV(t *testing.T) {
	//during the set up of the environment a custom resource of type advertisement.protocol.liqo.io
	//have been created. Here we test that given an advertisement we can retrieve the associated
	//custom resource of type tunnelEndpoint.liqonet.liqo.io
	var err error
	adv := getAdv()
	err = tunEndpointCreator.Get(ctx, client.ObjectKey{
		Namespace: adv.Namespace,
		Name:      adv.Name,
	}, adv)
	assert.Nil(t, err, "should be nil")
	//here we sleep in order to permit the controller to create the tunnelEndpoint custom resource
	time.Sleep(1 * time.Second)
	_, err = tunEndpointCreator.GetTunEndPerADV(adv)

	assert.Nil(t, err, "should be nil")
}

func TestCreateTunEndpoint(t *testing.T) {
	//testing that given a custom resource of type advertisement.protocol.liqo.io
	//a custom resource of type tunnelendpoint.liqonet.liqo.io is created and all the
	//associated fields are correct
	var tep v1.TunnelEndpoint
	var err error
	adv := getAdv()
	for {
		tep, err = tunEndpointCreator.GetTunEndPerADV(adv)
		if !k8sApiErrors.IsNotFound(err) {
			break
		}
	}
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, adv.Spec.Network.PodCIDR, tep.Spec.PodCIDR, "pod CIDRs should be equal")
	assert.Equal(t, adv.Spec.Network.GatewayIP, tep.Spec.TunnelPublicIP, "gatewayIPs should be equal")
	assert.Equal(t, adv.Spec.Network.GatewayPrivateIP, tep.Spec.TunnelPrivateIP, "gatewayPrivateIPs should be equal")
}

func TestUpdateTunEndpoint(t *testing.T) {
	//test1: given an advertisement.protocol.liqo.io custom resource which carries a network configuration
	//for a remote cluster that does not have conflicts with the local network configuration
	//then the .Status.RemoteRemappedPodCIDR should be set to "None" value
	adv := getAdv()
	time.Sleep(1 * time.Second)
	tep, err := tunEndpointCreator.GetTunEndPerADV(adv)
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, adv.Spec.Network.PodCIDR, tep.Spec.PodCIDR, "pod CIDRs should be equal")
	assert.Equal(t, adv.Spec.Network.GatewayIP, tep.Spec.TunnelPublicIP, "gatewayIPs should be equal")
	assert.Equal(t, adv.Spec.Network.GatewayPrivateIP, tep.Spec.TunnelPrivateIP, "gatewayPrivateIPs should be equal")
	assert.Equal(t, "New", tep.Status.Phase, "the phase field in status should be New")
	assert.Equal(t, "None", tep.Status.RemoteRemappedPodCIDR, "the remote podCIDR should be empty")

	//test2: same as the first test but in this case the podCIDR of the remote cluster has to be NATed
	newPodCIDR := "10.0.0.0/16"
	newAdv := getAdv()
	newAdv.Spec.Network.PodCIDR = newPodCIDR
	newAdv.Name = "conflict-testing"
	newAdv.Spec.ClusterId = "conflicting"
	err = tunEndpointCreator.Create(ctx, newAdv)
	assert.Nil(t, err, "should be nil")
	time.Sleep(2 * time.Second)
	tep, err = tunEndpointCreator.GetTunEndPerADV(newAdv)
	assert.Nil(t, err, "should be nil")
	//here we check that the remote pod CIDR has been remapped
	assert.NotEqual(t, "None", tep.Status.RemoteRemappedPodCIDR, "the remoteremappedPodCIDR should be set to a value different None")
	assert.NotEqual(t, "", tep.Status.RemoteRemappedPodCIDR, "the remoteremappedPodCIDR should be set to a value different than empty string")
	assert.Equal(t, "New", tep.Status.Phase, "the phase field in status should be New")

	//test3: we update the status of an existing advertisement.protocol.liqo.io
	//setting the Status.LocalRemappedPodCIDR field to a correct value
	//we expect that this value is set also in the status of tunnelendpoint.liqonet.liqo.io custom resource
	//associated to the previously updated advertisement.
	//and the Status.Phase field is set to "Processed"
	err = tunEndpointCreator.Get(ctx, client.ObjectKey{
		Namespace: newAdv.Namespace,
		Name:      newAdv.Name,
	}, newAdv)
	assert.Nil(t, err, "should be nil")
	newAdv.Status.LocalRemappedPodCIDR = "192.168.1.0/24"
	err = tunEndpointCreator.Status().Update(ctx, newAdv)
	assert.Nil(t, err, "should be nil")
	time.Sleep(2 * time.Second)
	tep, err = tunEndpointCreator.GetTunEndPerADV(newAdv)
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, "Processed", tep.Status.Phase, "phase should be set to Processed")
	assert.Equal(t, "192.168.1.0/24", tep.Status.LocalRemappedPodCIDR, "should be equal")

}

func TestDeleteTunEndpoint(t *testing.T) {
	//testing that after a advertisement.protocol.liqo.io custom resource is
	//deleted than the associated tunnelendpoint.liqonet.liqo.io custom resource is
	//deleted aswell
	adv := getAdv()
	err := tunEndpointCreator.Get(ctx, client.ObjectKey{
		Namespace: adv.Namespace,
		Name:      adv.Name,
	}, adv)
	assert.Nil(t, err, "should be nil")
	_, err = tunEndpointCreator.GetTunEndPerADV(adv)
	assert.Nil(t, err, "should be nil")
	err = tunEndpointCreator.Delete(ctx, adv)
	assert.Nil(t, err, "should be nil")
	time.Sleep(500 * time.Millisecond)
	_, err = tunEndpointCreator.GetTunEndPerADV(adv)
	assert.Equal(t, k8sApiErrors.IsNotFound(err), true, "the error should be notFound")
	adv = getAdv()
	err = tunEndpointCreator.Create(ctx, adv)
	assert.Nil(t, err, "should be nil")
}

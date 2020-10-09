package liqonet

import (
	"context"
	"fmt"
	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator"
	controller "github.com/liqotech/liqo/internal/liqonet"
	"github.com/liqotech/liqo/pkg/liqonet"
	liqonetOperator "github.com/liqotech/liqo/pkg/liqonet"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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
		GatewayIP:       "192.168.1.1",
		ReservedSubnets: make(map[string]*net.IPNet),
		IPManager: liqonetOperator.IpManager{
			UsedSubnets:        make(map[string]*net.IPNet),
			FreeSubnets:        make(map[string]*net.IPNet),
			ConflictingSubnets: make(map[string]*net.IPNet),
			SubnetPerCluster:   nil,
		},
		Mutex:        sync.Mutex{},
		IsConfigured: false,
		Configured:   nil,
		RetryTimeout: 0,
	}
}

func getClusterConfigurationCR(reservedSubnets []string) *configv1alpha1.ClusterConfig {
	return &configv1alpha1.ClusterConfig{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: configv1alpha1.ClusterConfigSpec{
			AdvertisementConfig: configv1alpha1.AdvertisementConfig{},
			DiscoveryConfig:     configv1alpha1.DiscoveryConfig{},
			LiqonetConfig: configv1alpha1.LiqonetConfig{
				ReservedSubnets: reservedSubnets,
				PodCIDR:         "10.1.0.0/16",
				ServiceCIDR:     "10.96.0.0/12",
				VxlanNetConfig:  liqonetOperator.VxlanNetConfig{},
			},
		},
		Status: configv1alpha1.ClusterConfigStatus{},
	}
}

func setupTunnelEndpointCreatorOperator() error {
	var err error
	dynClient := dynamic.NewForConfigOrDie(k8sManager.GetConfig())
	dynFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, controller.ResyncPeriod)
	tec = &controller.TunnelEndpointCreator{
		DynClient:                  dynClient,
		DynFactory:                 dynFactory,
		Client:                     k8sManager.GetClient(),
		Log:                        ctrl.Log.WithName("controllers").WithName("TunnelEndpointCreator"),
		Scheme:                     k8sManager.GetScheme(),
		ReservedSubnets:            make(map[string]*net.IPNet),
		Configured:                 make(chan bool, 1),
		ForeignClusterStartWatcher: make(chan bool, 1),
		ForeignClusterStopWatcher:  make(chan struct{}),
		IPManager: liqonet.IpManager{
			UsedSubnets:        make(map[string]*net.IPNet),
			FreeSubnets:        make(map[string]*net.IPNet),
			SubnetPerCluster:   make(map[string]*net.IPNet),
			ConflictingSubnets: make(map[string]*net.IPNet),
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
	tec.WatchConfiguration(newConfig, &configv1alpha1.GroupVersion)
	err = tec.SetupWithManager(k8sManager)
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	klog.Infof("starting watchers")
	foreingClusterHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    tec.ForeignClusterHandlerAdd,
		UpdateFunc: tec.ForeignClusterHandlerUpdate,
		DeleteFunc: tec.ForeignClusterHandlerDelete,
	}
	go tec.Watcher(tec.DynFactory, controller.ForeignClusterGVR, foreingClusterHandler, tec.ForeignClusterStartWatcher, tec.ForeignClusterStopWatcher)
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

func TestSetNetParameters(t *testing.T) {
	cc := getClusterConfig()
	assert.Equal(t, cc.Spec.LiqonetConfig.ServiceCIDR, tec.ServiceCIDR, "the two values should be equal")
	assert.Equal(t, cc.Spec.LiqonetConfig.PodCIDR, tec.PodCIDR, "the two values should be equal")
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

//test that the networkConfig is created and deleted based on the join status
func TestCreateNetConfigFromForeignClusterOutgoingJoined(t *testing.T) {
	fc := discoveryv1alpha1.ForeignCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ForeignCluster",
			APIVersion: discoveryv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foreigncluster-testing",
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID: "testing",
			},
			Namespace:     "",
			Join:          false,
			ApiUrl:        "",
			DiscoveryType: "LAN",
		},
		Status: discoveryv1alpha1.ForeignClusterStatus{
			Outgoing: discoveryv1alpha1.Outgoing{
				Joined: true,
			},
			Incoming: discoveryv1alpha1.Incoming{
				Joined: false,
			},
			Ttl: 0,
		},
	}
	tests := []struct {
		outgoingJoined bool
		incomingJoined bool
	}{
		{outgoingJoined: true, incomingJoined: false},  //expect the netconfig to be created and delete after the foreigncluster has been deleted
		{outgoingJoined: false, incomingJoined: true},  //expect the netconfig to be created and delete after the foreigncluster has been deleted
		{outgoingJoined: true, incomingJoined: true},   //expect the netconfig to be created and delete after the foreigncluster has been deleted
		{outgoingJoined: false, incomingJoined: false}, //expect the netconfig not to be created
	}
	//only the outgoing join is set to true
	//we expect the netconfig to be created
	//convert foreignCluster in unstructured object
	for _, test := range tests {
		fc.Status.Outgoing.Joined = test.outgoingJoined
		fc.Status.Incoming.Joined = test.incomingJoined
		if test.incomingJoined || test.outgoingJoined {
			pReqObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&fc)
			assert.Nil(t, err, "should be nil")
			//create foreign cluster
			_, err = tec.DynClient.Resource(controller.ForeignClusterGVR).Create(context.TODO(), &unstructured.Unstructured{Object: pReqObj}, metav1.CreateOptions{})
			assert.Nil(t, err, "should be nil")
			time.Sleep(2 * time.Second)
			netConfig, err := getNetworkConfigByLabel(fc)
			assert.Nil(t, err, "error should be nil")
			assert.Equal(t, fc.Spec.ClusterIdentity.ClusterID, netConfig.Spec.ClusterID, "should be equal")
			assert.Equal(t, tec.PodCIDR, netConfig.Spec.PodCIDR, "should be equal")
			assert.Equal(t, tec.GatewayIP, netConfig.Spec.TunnelPublicIP, "should be equal")
			labels := netConfig.GetLabels()
			assert.Equal(t, "true", labels[crdReplicator.LocalLabelSelector])
			assert.Equal(t, fc.Spec.ClusterIdentity.ClusterID, labels[crdReplicator.DestinationLabel])
			err = tec.Delete(context.TODO(), netConfig)
			assert.Nil(t, err)
			err = tec.DynClient.Resource(controller.ForeignClusterGVR).Delete(context.TODO(), fc.Name, metav1.DeleteOptions{})
			assert.Nil(t, err)
			time.Sleep(2 * time.Second)
			//check that the netconfig has been deleted
			netConfig, err = getNetworkConfigByLabel(fc)
			assert.Nil(t, netConfig)
			assert.True(t, apierrors.IsNotFound(err))
		} else {
			pReqObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&fc)
			assert.Nil(t, err, "should be nil")
			//create foreign cluster
			_, err = tec.DynClient.Resource(controller.ForeignClusterGVR).Create(context.TODO(), &unstructured.Unstructured{Object: pReqObj}, metav1.CreateOptions{})
			assert.Nil(t, err, "should be nil")
			time.Sleep(2 * time.Second)
			_, err = getNetworkConfigByLabel(fc)
			assert.True(t, apierrors.IsNotFound(err))
			err = tec.DynClient.Resource(controller.ForeignClusterGVR).Delete(context.TODO(), fc.Name, metav1.DeleteOptions{})
			assert.Nil(t, err)
			time.Sleep(2 * time.Second)
		}
	}

}

func getNetworkConfig() *netv1alpha1.NetworkConfig {
	return &netv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "netconfig-1",
		},
		Spec: netv1alpha1.NetworkConfigSpec{
			ClusterID:      "localclusterid",
			PodCIDR:        "10.2.0.0/16",
			TunnelPublicIP: "192.168.1.1",
		},
		Status: netv1alpha1.NetworkConfigStatus{},
	}
}

func getNetworkConfigByLabel(fc discoveryv1alpha1.ForeignCluster) (*netv1alpha1.NetworkConfig, error) {
	clusterID := fc.Spec.ClusterIdentity.ClusterID
	netConfigList := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdReplicator.DestinationLabel: clusterID}
	err := tec.List(context.Background(), netConfigList, labels)
	if err != nil {
		klog.Errorf("an error occurred while listing resources: %s", err)
		return nil, err
	}
	if len(netConfigList.Items) != 1 {
		if len(netConfigList.Items) == 0 {
			klog.Infof("no resource of type %s for remote cluster %s not found", netv1alpha1.GroupVersion.String(), clusterID)
			return nil, apierrors.NewNotFound(netv1alpha1.GroupResource, clusterID)
		} else {
			klog.Errorf("more than one instances of type %s exists for remote cluster %s", netv1alpha1.GroupVersion.String(), clusterID)
			return nil, fmt.Errorf("multiple instances of %s for remote cluster %s", netv1alpha1.GroupVersion.String(), clusterID)
		}
	}
	return &netConfigList.Items[0], nil
}

//we create a networkConfig instance as it comes from a peering cluster
//we expect the status to be set
func TestNetConfigProcessing(t *testing.T) {
	//test 1
	//the networkConfig has not a conflicting podCIDR
	labels := map[string]string{
		crdReplicator.LocalLabelSelector:     "false",
		crdReplicator.ReplicationStatuslabel: "true",
		crdReplicator.RemoteLabelSelector:    "remoteclusterid",
	}
	netConfig1 := getNetworkConfig()
	netConfig1.SetLabels(labels)
	err := tec.Create(context.Background(), netConfig1)
	assert.Nil(t, err)
	time.Sleep(3 * time.Second)
	err = tec.Get(context.Background(), types.NamespacedName{Name: netConfig1.Name}, netConfig1)
	assert.Nil(t, err)
	assert.Equal(t, "false", netConfig1.Status.NATEnabled)
	assert.Equal(t, "None", netConfig1.Status.PodCIDRNAT)

	//test2
	//the networkConfig has a conflicting podCIDR
	netConfig2 := getNetworkConfig()
	netConfig2.SetLabels(labels)
	netConfig2.Name = "netconfig-2"
	netConfig2.Spec.PodCIDR = "10.1.0.0/12"
	err = tec.Create(context.Background(), netConfig2)
	assert.Nil(t, err)
	time.Sleep(2 * time.Second)
	err = tec.Get(context.Background(), types.NamespacedName{Name: netConfig2.Name}, netConfig2)
	assert.Nil(t, err)
	assert.Equal(t, "true", netConfig2.Status.NATEnabled)
	assert.NotEqual(t, "None", netConfig2.Status.PodCIDRNAT)

	//test3
	//modifying the netconfig2 to be a local resource
	//we expect a tunnelEndpoint to be created
	localLabels := map[string]string{
		crdReplicator.LocalLabelSelector: "true",
		crdReplicator.DestinationLabel:   "remoteclusterid",
	}
	netConfig2.SetLabels(localLabels)
	netConfig2.Spec.ClusterID = "remoteclusterid"
	err = tec.Update(context.Background(), netConfig2)
	assert.Nil(t, err)
	time.Sleep(10 * time.Second)
	tep, found, err := tec.GetTunnelEndpoint("remoteclusterid")
	assert.True(t, found)
	assert.Nil(t, err)
	assert.Equal(t, netConfig1.Status.PodCIDRNAT, tep.Status.RemoteRemappedPodCIDR)
	assert.Equal(t, netConfig2.Status.PodCIDRNAT, tep.Status.LocalRemappedPodCIDR)
	assert.Equal(t, netConfig2.Spec.ClusterID, tep.Spec.ClusterID)
	assert.Equal(t, netConfig1.Spec.PodCIDR, tep.Spec.PodCIDR)
	assert.Equal(t, netConfig1.Spec.TunnelPublicIP, tep.Spec.TunnelPublicIP)
	assert.Equal(t, "Ready", tep.Status.Phase)

	//test4
	//we change some fields on the remote netConfig spec and some on the local netConfig status
	//expect that the tunnelEndpoint is updated
	err = tec.Get(context.Background(), types.NamespacedName{Name: netConfig1.Name}, netConfig1)
	assert.Nil(t, err)
	newPodCIDR := "10.200.0.0/16"
	netConfig1.Spec.PodCIDR = newPodCIDR
	err = tec.Update(context.Background(), netConfig1)
	assert.Nil(t, err)
	err = tec.Get(context.Background(), types.NamespacedName{Name: netConfig2.Name}, netConfig2)
	assert.Nil(t, err)
	newNATPodCIDR := "10.300.0.0/16"
	netConfig2.Status.PodCIDRNAT = newNATPodCIDR
	err = tec.Status().Update(context.Background(), netConfig2)
	assert.Nil(t, err)
	time.Sleep(3 * time.Second)
	tep, found, err = tec.GetTunnelEndpoint("remoteclusterid")
	assert.True(t, found)
	assert.Nil(t, err)
	assert.Equal(t, newNATPodCIDR, tep.Status.LocalRemappedPodCIDR)
	assert.Equal(t, newPodCIDR, tep.Spec.PodCIDR)

	//test5
	//we delete the local netConfig
	//expect that the tunneEndpoint associated is also deleted
	err = tec.Get(context.Background(), types.NamespacedName{Name: netConfig2.Name}, netConfig2)
	assert.Nil(t, err)
	err = tec.Delete(context.Background(), netConfig2)
	assert.Nil(t, err)
	time.Sleep(2 * time.Second)
	_, found, err = tec.GetTunnelEndpoint("remoteclusterid")
	assert.False(t, found)
	assert.Nil(t, err)
	time.Sleep(10 * time.Second)
}

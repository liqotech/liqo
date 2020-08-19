package controllers

import (
	v1 "github.com/liqoTech/liqo/api/liqonet/v1"
	"github.com/liqoTech/liqo/pkg/liqonet"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
)

func GetTunnelEndpointCR() *v1.TunnelEndpoint {
	return &v1.TunnelEndpoint{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1.TunnelEndpointSpec{
			ClusterID:       "cluster-test",
			PodCIDR:         "10.0.0.0/12",
			TunnelPublicIP:  "192.168.5.1",
			TunnelPrivateIP: "192.168.4.1",
		},
		Status: v1.TunnelEndpointStatus{
			Phase:                 "",
			LocalRemappedPodCIDR:  "None",
			RemoteRemappedPodCIDR: "None",
			NATEnabled:            false,
			RemoteTunnelPublicIP:  "192.168.10.1",
			RemoteTunnelPrivateIP: "192.168.9.1",
			LocalTunnelPublicIP:   "192.168.5.1",
			LocalTunnelPrivateIP:  "192.168.4.1",
			TunnelIFaceIndex:      0,
			TunnelIFaceName:       "",
		},
	}
}

func getRouteController() *RouteController {
	return &RouteController{
		Client:                             nil,
		Log:                                ctrl.Log.WithName("route-operator"),
		Scheme:                             nil,
		clientset:                          kubernetes.Clientset{},
		RouteOperator:                      false,
		NodeName:                           "test",
		ClientSet:                          nil,
		RemoteVTEPs:                        nil,
		IsGateway:                          false,
		VxlanNetwork:                       "",
		GatewayVxlanIP:                     "172.12.1.1",
		VxlanIfaceName:                     "vxlanTest",
		VxlanPort:                          0,
		ClusterPodCIDR:                     "10.1.0.0/16",
		IPTablesRuleSpecsReferencingChains: make(map[string]liqonet.IPtableRule),
		IPTablesChains:                     make(map[string]liqonet.IPTableChain),
		IPtablesRuleSpecsPerRemoteCluster:  make(map[string][]liqonet.IPtableRule),
		RoutesPerRemoteCluster:             make(map[string][]netlink.Route),
		RetryTimeout:                       0,
		IPtables: &liqonet.MockIPTables{
			Rules:  []liqonet.IPtableRule{},
			Chains: []liqonet.IPTableChain{},
		},
		NetLink: &liqonet.MockRouteManager{
			RouteList: []netlink.Route{},
		},
	}
}

func routePerDestination(routeList []netlink.Route, dest string) bool {
	for _, route := range routeList {
		if route.Dst.String() == dest {
			return true
		}
	}
	return false
}

func TestCreateAndInsertIPTablesChains(t *testing.T) {
	//testing that all the tables and chains are inserted correctly
	//the function should be idempotent
	r := getRouteController()
	//the function is run 3 times and we expect that the number of tables is 3 and of rules 4
	for i := 3; i >= 0; i-- {
		err := r.createAndInsertIPTablesChains()
		assert.Nil(t, err, "error should be nil")
		assert.Equal(t, 3, len(r.IPTablesChains), "there should be three new chains")
		assert.Equal(t, 4, len(r.IPTablesRuleSpecsReferencingChains), "there should be 4 new rules")
	}
}

func TestAddIPTablesRulespecForRemoteCluster(t *testing.T) {
	r := getRouteController()
	tep := GetTunnelEndpointCR()
	//test:1 NAT not enabled and node is not the gateway
	//in this case we expect only 3 rules to be inserted
	err := r.addIPTablesRulespecForRemoteCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 3, len(r.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 3 rules")

	//test:2 NAT enabled and node is not the gateway
	//in this case we expect 3 rules to be inserted
	r = getRouteController()
	tep.Status.RemoteRemappedPodCIDR = "10.96.0.0/16"
	tep.Status.LocalRemappedPodCIDR = "10.100.0.0/16"
	err = r.addIPTablesRulespecForRemoteCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 3, len(r.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 3 rules")

	//test:3 NAT not enabled and node is the gateway
	//in this case we expect 4 rules to be inserted
	r = getRouteController()
	r.IsGateway = true
	tep.Status.LocalRemappedPodCIDR = "None"
	err = r.addIPTablesRulespecForRemoteCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 4, len(r.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 4 rules")

	//test:4 NAT enabled and node is the gateway
	//in this case we expect 6 rules to be inserted
	r = getRouteController()
	r.IsGateway = true
	tep.Status.LocalRemappedPodCIDR = "10.100.0.0/16"
	err = r.addIPTablesRulespecForRemoteCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 6, len(r.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 6 rules")
}

func TestDeleteIPTablesRulespecForRemoteCluster(t *testing.T) {
	//Testing that giving a tunnelEnpoint.liqonet.liqo.io we can remove
	//all the rules inserted for the cluster described by the custom resource
	//firt we add the rules and then we remove it
	//expecting that the rulse are 0.
	r := getRouteController()
	tep := GetTunnelEndpointCR()
	r.IsGateway = true
	tep.Status.LocalRemappedPodCIDR = "10.100.0.0/16"
	err := r.addIPTablesRulespecForRemoteCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 6, len(r.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 6 rules")
	err = r.deleteIPTablesRulespecForRemoteCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 0, len(r.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 6 rules")
}

func TestDeleteAllIPTablesChains(t *testing.T) {
	//testing that all the iptables chains are deleted
	//first we add the rules for a new cluster described by a tunnelendpoint.liqone.liqo.io
	//which have NatEnabled and the node is the gateway node.
	//after that 6 rules should be present, after the delete function is called
	//0 rules should be present
	r := getRouteController()
	tep := GetTunnelEndpointCR()
	r.IsGateway = true
	tep.Status.LocalRemappedPodCIDR = "10.100.0.0/16"
	err := r.addIPTablesRulespecForRemoteCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 6, len(r.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 6 rules")
	r.DeleteAllIPTablesChains()
	assert.Equal(t, 0, len(r.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 0 rules")
	assert.Equal(t, 0, len(r.IPTablesChains), "number of chains should be 0")
	assert.Equal(t, 0, len(r.IPTablesRuleSpecsReferencingChains), "number of rules referencing the chains should be 0")

}

func TestInsertRoutesPerCluster(t *testing.T) {
	//test1: given a tunnelendpoint.liqonet.liqo.io we add the routes
	//in a node that is not the gateway node
	//the expected number of routes is two
	r := getRouteController()
	tep := GetTunnelEndpointCR()
	err := r.InsertRoutesPerCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 1, len(r.RoutesPerRemoteCluster[tep.Spec.ClusterID]), "number of routes should be 1")
	assert.True(t, routePerDestination(r.RoutesPerRemoteCluster[tep.Spec.ClusterID], tep.Spec.PodCIDR), "the route for the remote pod cidr should be present")

	//test2: same as above but the node is gateway node and the remote pod CIDR has been remapped
	//the expected number of routes is 2
	r = getRouteController()
	r.IsGateway = true
	tep = GetTunnelEndpointCR()
	tep.Status.RemoteRemappedPodCIDR = "10.100.0.0/16"
	err = r.InsertRoutesPerCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 1, len(r.RoutesPerRemoteCluster[tep.Spec.ClusterID]), "number of routes should be 1")
	assert.True(t, routePerDestination(r.RoutesPerRemoteCluster[tep.Spec.ClusterID], "10.100.0.0/16"), "the route for the remote remapped pod cidr should be present")
}

func TestDeleteRoutesPerCluster(t *testing.T) {
	//first we add routes for cluster and then we delete them and check if
	//the results are as expected
	//When we delete the routes for a given cluster we expect that all the routes are removed
	r := getRouteController()
	tep := GetTunnelEndpointCR()
	err := r.InsertRoutesPerCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 1, len(r.RoutesPerRemoteCluster[tep.Spec.ClusterID]), "number of routes should be 1")
	assert.True(t, routePerDestination(r.RoutesPerRemoteCluster[tep.Spec.ClusterID], tep.Spec.PodCIDR), "the route for the remote pod cidr should be present")
	//here we delete all the routes for the cluster
	err = r.deleteRoutesPerCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Zero(t, len(r.RoutesPerRemoteCluster[tep.Spec.ClusterID]), "routes for the cluster should be zero")
}

func TestDeleteAllRoutes(t *testing.T) {
	//testing that all the routes are removed for all the clusters
	r := getRouteController()
	tep := GetTunnelEndpointCR()
	err := r.InsertRoutesPerCluster(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 1, len(r.RoutesPerRemoteCluster[tep.Spec.ClusterID]), "number of routes should be 2")
	assert.True(t, routePerDestination(r.RoutesPerRemoteCluster[tep.Spec.ClusterID], tep.Spec.PodCIDR), "the route for the remote pod cidr should be present")
	//here we delete all the routes for the clusters
	r.deleteAllRoutes()
	assert.Zero(t, len(r.RoutesPerRemoteCluster), "routes for the cluster should be zero")
}

package liqonet

import (
	clusterConfig "github.com/liqoTech/liqo/api/cluster-config/v1"
	v1 "github.com/liqoTech/liqo/api/liqonet/v1"
	controller "github.com/liqoTech/liqo/internal/liqonet"
	"github.com/liqoTech/liqo/pkg/liqonet"
	utils "github.com/liqoTech/liqo/pkg/liqonet"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"net"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"
)

var (
	waitTime = 1 * time.Second
)

func GetTunnelEndpointCR() *v1.TunnelEndpoint {
	return &v1.TunnelEndpoint{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tep-1",
			Namespace: "",
		},
		Spec: v1.TunnelEndpointSpec{
			ClusterID:       "cluster-test",
			PodCIDR:         "10.0.0.0/16",
			TunnelPublicIP:  "192.168.5.1",
			TunnelPrivateIP: "192.168.4.1",
		},
		Status: v1.TunnelEndpointStatus{},
	}
}

func setupRouteOperator() error {
	var err error
	routeOperator = &controller.RouteController{
		Client:         k8sManager.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("routeOperator"),
		Scheme:         k8sManager.GetScheme(),
		RouteOperator:  false,
		NodeName:       "testing",
		ClientSet:      nil,
		RemoteVTEPs:    nil,
		IsGateway:      false,
		VxlanNetwork:   "",
		GatewayVxlanIP: "192.168.2.2",
		VxlanIfaceName: "vxlanTest",
		VxlanPort:      0,
		IPtables: &liqonet.MockIPTables{
			Rules:  []liqonet.IPtableRule{},
			Chains: []liqonet.IPTableChain{},
		},
		NetLink: &liqonet.MockRouteManager{
			RouteList: []netlink.Route{},
		},
		Configured:                         make(chan bool, 1),
		IPTablesRuleSpecsReferencingChains: make(map[string]liqonet.IPtableRule),
		IPTablesChains:                     make(map[string]liqonet.IPTableChain),
		IPtablesRuleSpecsPerRemoteCluster:  make(map[string][]liqonet.IPtableRule),
		RoutesPerRemoteCluster:             make(map[string][]netlink.Route),
		RetryTimeout:                       0,
	}
	config := k8sManager.GetConfig()
	newConfig := &rest.Config{
		Host: config.Host,
		// gotta go fast during tests -- we don't really care about overwhelming our test API server
		QPS:   1000.0,
		Burst: 2000.0,
	}
	routeOperator.WatchConfiguration(newConfig, &clusterConfig.GroupVersion)
	if !routeOperator.IsConfigured {
		<-routeOperator.Configured
		routeOperator.IsConfigured = true
		klog.Infof("route-operator configured with podCIDR %s", routeOperator.ClusterPodCIDR)
	}
	err = routeOperator.SetupWithManager(k8sManager)
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	return nil
}

//creates the tunnelendpoint.liqonet.liqo.io custom resource
func createTEP() (*v1.TunnelEndpoint, error) {
	tep := GetTunnelEndpointCR()
	err := routeOperator.Client.Create(ctx, tep)
	if err != nil {
		return nil, err
	}
	time.Sleep(waitTime)
	err = routeOperator.Client.Get(ctx, client.ObjectKey{
		Namespace: tep.Namespace,
		Name:      tep.Name,
	}, tep)
	if err != nil {
		return nil, err
	}
	return tep, nil
}

func updateStatusTEP(tep *v1.TunnelEndpoint) (*v1.TunnelEndpoint, error) {
	err := routeOperator.Client.Status().Update(ctx, tep)
	if err != nil {
		return nil, err
	}
	time.Sleep(waitTime)
	return tep, nil
}

//set the  tunnelendpoint.liqonet.liqo.io custom resource to ready in order to be processed by the operator
func setToReady(tep *v1.TunnelEndpoint) error {
	err := routeOperator.Client.Get(ctx, client.ObjectKey{
		Namespace: tep.Namespace,
		Name:      tep.Name,
	}, tep)
	if err != nil {
		return err
	}
	tep.ObjectMeta.SetLabels(utils.SetLabelHandler(utils.TunOpLabelKey, "ready", tep.ObjectMeta.GetLabels()))
	err = routeOperator.Client.Update(ctx, tep)
	if err != nil {
		return err
	}
	time.Sleep(waitTime)
	return nil
}

func getIPtablesRules(endpoint *v1.TunnelEndpoint) map[string][]liqonet.IPtableRule {
	var remotePodCIDR, localPodCIDR string
	var rules = make(map[string][]liqonet.IPtableRule)
	clusterID := endpoint.Spec.ClusterID
	if endpoint.Status.RemoteRemappedPodCIDR != "None" && endpoint.Status.RemoteRemappedPodCIDR != "" {
		remotePodCIDR = endpoint.Status.RemoteRemappedPodCIDR
	} else {
		remotePodCIDR = endpoint.Spec.PodCIDR
	}
	if endpoint.Status.LocalRemappedPodCIDR != "None" && endpoint.Status.LocalRemappedPodCIDR != "" {
		localPodCIDR = endpoint.Status.LocalRemappedPodCIDR
	} else {
		localPodCIDR = routeOperator.ClusterPodCIDR
	}
	var ruleSpecs []utils.IPtableRule
	ruleSpec := []string{"-s", routeOperator.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "ACCEPT"}

	ruleSpecs = append(ruleSpecs, utils.IPtableRule{
		Table:    controller.NatTable,
		Chain:    controller.LiqonetPostroutingChain,
		RuleSpec: ruleSpec,
	})
	rules[clusterID] = ruleSpecs
	//enable forwarding for all the traffic directed to the remote pods
	ruleSpec = []string{"-d", remotePodCIDR, "-j", "ACCEPT"}
	ruleSpecs = append(ruleSpecs, utils.IPtableRule{
		Table:    controller.FilterTable,
		Chain:    controller.LiqonetForwardingChain,
		RuleSpec: ruleSpec,
	})
	rules[clusterID] = ruleSpecs
	//this rules are needed in an environment where strictly policies are applied for the input chain
	ruleSpec = []string{"-s", routeOperator.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "ACCEPT"}
	ruleSpecs = append(ruleSpecs, utils.IPtableRule{
		Table:    controller.FilterTable,
		Chain:    controller.LiqonetInputChain,
		RuleSpec: ruleSpec,
	})
	rules[clusterID] = ruleSpecs
	if routeOperator.IsGateway {
		//we get the first IP address from the podCIDR of the local cluster
		natIP, _, err := net.ParseCIDR(localPodCIDR)
		if err != nil {
			klog.Infof("unable to get the IP from localPodCIDR %s used to NAT the traffic from localhosts to remote hosts", localPodCIDR)
		}
		//all the traffic coming from the hosts and directed to the remote pods is natted using the first IP address
		//taken from the podCIDR of the local cluster
		//all the traffic leaving the tunnel interface is source nated.
		ruleSpec = []string{"-o", endpoint.Status.TunnelIFaceName, "-j", "SNAT", "--to", natIP.String()}

		ruleSpecs = append(ruleSpecs, utils.IPtableRule{
			Table:    controller.NatTable,
			Chain:    controller.LiqonetPostroutingChain,
			RuleSpec: ruleSpec,
		})
		rules[clusterID] = ruleSpecs
		//if we have been remapped by the remote cluster then insert the iptables rule to masquerade the source ip
		if endpoint.Status.LocalRemappedPodCIDR != "None" {
			ruleSpec = []string{"-s", routeOperator.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "NETMAP", "--to", endpoint.Status.LocalRemappedPodCIDR}
			ruleSpecs = append(ruleSpecs, utils.IPtableRule{
				Table:    controller.NatTable,
				Chain:    controller.LiqonetPostroutingChain,
				RuleSpec: ruleSpec,
			})
			rules[clusterID] = ruleSpecs
			//translate all the traffic coming to the local cluster in to the right podcidr because it has been remapped by the remote cluster
			ruleSpec = []string{"-d", endpoint.Status.LocalRemappedPodCIDR, "-i", endpoint.Status.TunnelIFaceName, "-j", "NETMAP", "--to", routeOperator.ClusterPodCIDR}
			ruleSpecs = append(ruleSpecs, utils.IPtableRule{
				Table:    controller.NatTable,
				Chain:    controller.LiqonetPreroutingChain,
				RuleSpec: ruleSpec,
			})
			rules[clusterID] = ruleSpecs
		}
	}
	return rules
}

func routePerDestination(routeList []netlink.Route, dest string) bool {
	for _, route := range routeList {
		if route.Dst.String() == dest {
			return true
		}
	}
	return false
}

//test1: a tunnelendpoint.liqonet.liqo.io CR is created without nat enabled and operator running in a non gateway node
//we expect 3 iptables rules and 2 routes
func Test1RouteOperator(t *testing.T) {
	tep, err := createTEP()
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, tep, "the cr should not be nil")
	tep.Status.RemoteTunnelPublicIP = "192.168.200.1"
	tep.Status.RemoteTunnelPrivateIP = "192.168.190.1"
	tep.Status.RemoteRemappedPodCIDR = "None"
	tep.Status.LocalRemappedPodCIDR = "None"
	tep, err = updateStatusTEP(tep)
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, tep, "the cr should not be nil")
	err = setToReady(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 3, len(routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 3 rules")
	assert.Equal(t, 1, len(routeOperator.RoutesPerRemoteCluster[tep.Spec.ClusterID]), "number of routes should be 1")
	expectedRules := getIPtablesRules(tep)
	equal := reflect.DeepEqual(expectedRules[tep.Spec.ClusterID], routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID])
	assert.True(t, equal, "the rules should be the same")
	assert.True(t, routePerDestination(routeOperator.RoutesPerRemoteCluster[tep.Spec.ClusterID], tep.Spec.PodCIDR), "the route for the remote pod cidr should be present")
	//here we remove the custom resource
	err = routeOperator.Client.Delete(ctx, tep)
	assert.Nil(t, err, "error should be nil")
	time.Sleep(waitTime)
	assert.Equal(t, 0, len(routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 3 rules")
}

//test2: a tunnelendpoint.liqonet.liqo.io CR is created without nat enabled and operator running in a gateway node
//we expect 4 iptables rules and 2 routes
func Test2RouteOperator(t *testing.T) {
	routeOperator.IsGateway = true
	tep, err := createTEP()
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, tep, "the cr should not be nil")
	tep.Status.RemoteTunnelPublicIP = "192.168.200.1"
	tep.Status.RemoteTunnelPrivateIP = "192.168.190.1"
	tep.Status.RemoteRemappedPodCIDR = "10.96.0.0/16"
	tep.Status.LocalRemappedPodCIDR = "None"
	tep, err = updateStatusTEP(tep)
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, tep, "the cr should not be nil")
	err = setToReady(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 4, len(routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 4 rules")
	assert.Equal(t, 1, len(routeOperator.RoutesPerRemoteCluster[tep.Spec.ClusterID]), "number of routes should be 1")
	expectedRules := getIPtablesRules(tep)
	equal := reflect.DeepEqual(expectedRules[tep.Spec.ClusterID], routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID])
	assert.True(t, equal, "the rules should be the same")
	assert.True(t, routePerDestination(routeOperator.RoutesPerRemoteCluster[tep.Spec.ClusterID], tep.Status.RemoteRemappedPodCIDR), "the route for the remote remapped pod cidr should be present")
	//here we remove the custom resource
	err = routeOperator.Client.Delete(ctx, tep)
	assert.Nil(t, err, "error should be nil")
	time.Sleep(waitTime)
	assert.Equal(t, 0, len(routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 3 rules")
}

//test3: a tunnelendpoint.liqonet.liqo.io CR is created with NAT and operator running in a not gateway node
//we expect 6 iptables rules and 2 routes
func Test3RouteOperator(t *testing.T) {
	routeOperator.IsGateway = false
	tep, err := createTEP()
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, tep, "the cr should not be nil")
	tep.Status.RemoteTunnelPublicIP = "192.168.200.1"
	tep.Status.RemoteTunnelPrivateIP = "192.168.190.1"
	tep.Status.RemoteRemappedPodCIDR = "10.96.0.0/16"
	tep.Status.LocalRemappedPodCIDR = "10.100.0.0/16"
	tep, err = updateStatusTEP(tep)
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, tep, "the cr should not be nil")
	err = setToReady(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 3, len(routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 6 rules")
	assert.Equal(t, 1, len(routeOperator.RoutesPerRemoteCluster[tep.Spec.ClusterID]), "number of routes should be 1")
	expectedRules := getIPtablesRules(tep)
	equal := reflect.DeepEqual(expectedRules[tep.Spec.ClusterID], routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID])
	assert.True(t, equal, "the rules should be the same")
	assert.True(t, routePerDestination(routeOperator.RoutesPerRemoteCluster[tep.Spec.ClusterID], tep.Status.RemoteRemappedPodCIDR), "the route for the remote pod cidr should be present")
	//here we remove the custom resource
	err = routeOperator.Client.Delete(ctx, tep)
	assert.Nil(t, err, "error should be nil")
	time.Sleep(waitTime)
	assert.Equal(t, 0, len(routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 3 rules")
}

//test4: a tunnelendpoint.liqonet.liqo.io CR is created with NAT and operator running in a gateway node
//we expect 6 iptables rules and 2 routes
func Test4RouteOperator(t *testing.T) {
	routeOperator.IsGateway = true
	tep, err := createTEP()
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, tep, "the cr should not be nil")
	tep.Status.RemoteTunnelPublicIP = "192.168.200.1"
	tep.Status.RemoteTunnelPrivateIP = "192.168.190.1"
	tep.Status.RemoteRemappedPodCIDR = "10.96.0.0/16"
	tep.Status.LocalRemappedPodCIDR = "10.100.0.0/16"
	tep, err = updateStatusTEP(tep)
	assert.Nil(t, err, "error should be nil")
	assert.NotNil(t, tep, "the cr should not be nil")
	err = setToReady(tep)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, 6, len(routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 6 rules")
	assert.Equal(t, 1, len(routeOperator.RoutesPerRemoteCluster[tep.Spec.ClusterID]), "number of routes should be 1")
	expectedRules := getIPtablesRules(tep)
	equal := reflect.DeepEqual(expectedRules[tep.Spec.ClusterID], routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID])
	assert.True(t, equal, "the rules should be the same")
	assert.True(t, routePerDestination(routeOperator.RoutesPerRemoteCluster[tep.Spec.ClusterID], tep.Status.RemoteRemappedPodCIDR), "the route for the remote remapped pod cidr should be present")
	//here we remove the custom resource
	err = routeOperator.Client.Delete(ctx, tep)
	assert.Nil(t, err, "error should be nil")
	time.Sleep(waitTime)
	assert.Equal(t, 0, len(routeOperator.IPtablesRuleSpecsPerRemoteCluster[tep.Spec.ClusterID]), "there should be 3 rules")
}

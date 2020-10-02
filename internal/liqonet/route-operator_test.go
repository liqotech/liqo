package liqonetOperators

import (
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"reflect"
	"strings"
	"testing"
)

var (
	ip *liqonet.MockIPTables
)

func GetTunnelEndpointCR() *netv1alpha1.TunnelEndpoint {
	return &netv1alpha1.TunnelEndpoint{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: netv1alpha1.TunnelEndpointSpec{
			ClusterID:      "cluster-test",
			PodCIDR:        "10.100.0.0/16",
			TunnelPublicIP: "192.168.5.1",
		},
		Status: netv1alpha1.TunnelEndpointStatus{
			Phase:                 "",
			LocalRemappedPodCIDR:  "None",
			RemoteRemappedPodCIDR: "None",
			NATEnabled:            false,
			RemoteTunnelPublicIP:  "192.168.10.1",
			LocalTunnelPublicIP:   "192.168.5.1",
			TunnelIFaceIndex:      0,
			TunnelIFaceName:       "testtunnel",
		},
	}
}

func getRouteController() *RouteController {
	ip = &liqonet.MockIPTables{
		Rules:  []liqonet.IPtableRule{},
		Chains: []liqonet.IPTableChain{},
	}
	return &RouteController{
		Client:                             nil,
		Scheme:                             nil,
		clientset:                          kubernetes.Clientset{},
		NodeName:                           "test",
		ClientSet:                          nil,
		RemoteVTEPs:                        nil,
		IsGateway:                          false,
		VxlanNetwork:                       "",
		GatewayVxlanIP:                     "172.12.1.1",
		VxlanIfaceName:                     "vxlanTest",
		VxlanPort:                          0,
		ClusterPodCIDR:                     "10.200.0.0/16",
		IPTablesRuleSpecsReferencingChains: make(map[string]liqonet.IPtableRule),
		IPTablesChains:                     make(map[string]liqonet.IPTableChain),
		RetryTimeout:                       0,
		IPtables:                           ip,
		NetLink: &liqonet.MockRouteManager{
			RouteList: []netlink.Route{},
		},
	}
}

func TestCreateAndInsertIPTablesChains(t *testing.T) {
	//testing that all the tables and chains are inserted correctly
	//the function should be idempotent
	r := getRouteController()
	//the function is run 3 times and we expect that the number of tables is 3 and of rules 4
	for i := 3; i >= 0; i-- {
		err := r.CreateAndEnsureIPTablesChains()
		assert.Nil(t, err, "error should be nil")
		assert.Equal(t, 4, len(r.IPTablesChains), "there should be 4 new chains")
		assert.Equal(t, 4, len(r.IPTablesRuleSpecsReferencingChains), "there should be 4 new rules")
	}

}

func TestRouteController_InsertIptablesRulespecIfNotExists(t *testing.T) {
	r := getRouteController()
	rulespec1 := liqonet.IPtableRule{
		Table:    "TestTable",
		Chain:    "TestChain",
		RuleSpec: []string{"this", "rule", "has", "to", "be", "at", "first", "position"},
	}
	rulespec2 := liqonet.IPtableRule{
		Table:    "TestTable",
		Chain:    "TestChain",
		RuleSpec: []string{"this", "rule", "used", "to", "test", "InsertIptablesRulespecIfNotExists"},
	}
	//insert the rule
	_ = r.InsertIptablesRulespecIfNotExists(rulespec1.Table, rulespec1.Chain, rulespec1.RuleSpec)
	//we check that the rule is at first position
	rules, _ := r.IPtables.List(rulespec1.Table, rulespec1.Chain)
	assert.True(t, reflect.DeepEqual(rules[0], strings.Join(rulespec1.RuleSpec, " ")))
	//insert rule two at the first position
	_ = r.IPtables.Insert(rulespec1.Table, rulespec1.Chain, 1, rulespec2.RuleSpec...)
	//check that the new rule is at the first position
	rules, _ = r.IPtables.List(rulespec1.Table, rulespec1.Chain)
	assert.True(t, reflect.DeepEqual(rules[0], strings.Join(rulespec2.RuleSpec, " ")))
	//ensure that rulespec1 is at first position
	_ = r.InsertIptablesRulespecIfNotExists(rulespec1.Table, rulespec1.Chain, rulespec1.RuleSpec)
	//we check that the rule is at first position
	rules, _ = r.IPtables.List(rulespec1.Table, rulespec1.Chain)
	assert.True(t, reflect.DeepEqual(rules[0], strings.Join(rulespec1.RuleSpec, " ")))
	//append again the rulespec1
	_ = r.IPtables.Insert(rulespec1.Table, rulespec1.Chain, 3, rulespec1.RuleSpec...)
	//make sure that rulespec1 is present at least two times
	occurrencies := 0
	rules, _ = r.IPtables.List(rulespec1.Table, rulespec1.Chain)
	for _, rule := range rules {
		if rule == strings.Join(rulespec1.RuleSpec, " ") {
			occurrencies++
		}
	}
	assert.Greater(t, occurrencies, 1)
	//ensure that the rulespec1 is at first position and present only once
	_ = r.InsertIptablesRulespecIfNotExists(rulespec1.Table, rulespec1.Chain, rulespec1.RuleSpec)
	occurrencies = 0
	rules, _ = r.IPtables.List(rulespec1.Table, rulespec1.Chain)
	for _, rule := range rules {
		if rule == strings.Join(rulespec1.RuleSpec, " ") {
			occurrencies++
		}
	}
	assert.Equal(t, 1, occurrencies)
	assert.True(t, reflect.DeepEqual(rules[0], strings.Join(rulespec1.RuleSpec, " ")))
}

func TestRouteController_GetPodCIDRS(t *testing.T) {
	tep := GetTunnelEndpointCR()
	r := getRouteController()
	tests := []struct {
		localRemappedPodCIDR  string
		remoteRemappedPodCIDR string
		remotePodCIDR         string
		expectedLocalCIDR     string
		expectedRemoteCIDR    string
	}{
		{
			defaultPodCIDRValue,
			defaultPodCIDRValue,
			"10.200.0.0/16",
			defaultPodCIDRValue,
			"10.200.0.0/16",
		},
		{
			"10.1.0.0/16",
			"10.2.0.0/16",
			"10.3.0.0/16",
			"10.1.0.0/16",
			"10.2.0.0/16",
		},
	}
	for _, test := range tests {
		tep.Spec.PodCIDR = test.remotePodCIDR
		tep.Status.LocalRemappedPodCIDR = test.localRemappedPodCIDR
		tep.Status.RemoteRemappedPodCIDR = test.remoteRemappedPodCIDR
		localPodCIDR, remotePodCIDR := r.GetPodCIDRS(tep)
		assert.Equal(t, test.expectedLocalCIDR, localPodCIDR)
		assert.Equal(t, test.expectedRemoteCIDR, remotePodCIDR)
	}
}

func TestRouteController_GetPostroutingRules(t *testing.T) {
	r := getRouteController()
	tep := GetTunnelEndpointCR()

	tests := []struct {
		isGateway             bool
		localRemappedPodCIDR  string
		remoteRemappedPodCIDR string
		rules                 []string
	}{
		{
			true,
			"10.1.0.0/16",
			defaultPodCIDRValue,
			[]string{strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", tep.Spec.PodCIDR, "-j", "NETMAP", "--to", "10.1.0.0/16"}, " "), strings.Join([]string{"!", "-s", r.ClusterPodCIDR, "-d", tep.Spec.PodCIDR, "-j", "SNAT", "--to-source", "10.1.0.0"}, " ")},
		},
		{
			true,
			"10.1.0.0/16",
			"10.2.0.0/16",
			[]string{strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", "10.2.0.0/16", "-j", "NETMAP", "--to", "10.1.0.0/16"}, " "), strings.Join([]string{"!", "-s", r.ClusterPodCIDR, "-d", "10.2.0.0/16", "-j", "SNAT", "--to-source", "10.1.0.0"}, " ")},
		},
		{
			true,
			defaultPodCIDRValue,
			"10.2.0.0/16",
			[]string{strings.Join([]string{"!", "-s", r.ClusterPodCIDR, "-d", "10.2.0.0/16", "-j", "SNAT", "--to-source", "10.200.0.0"}, " "), strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", "10.2.0.0/16", "-j", "ACCEPT"}, " ")},
		},
		{
			true,
			defaultPodCIDRValue,
			defaultPodCIDRValue,
			[]string{strings.Join([]string{"!", "-s", r.ClusterPodCIDR, "-d", tep.Spec.PodCIDR, "-j", "SNAT", "--to-source", "10.200.0.0"}, " "), strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", tep.Spec.PodCIDR, "-j", "ACCEPT"}, " ")},
		},
		{
			false,
			defaultPodCIDRValue,
			defaultPodCIDRValue,
			[]string{strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", tep.Spec.PodCIDR, "-j", "ACCEPT"}, " ")},
		},
		{
			false,
			defaultPodCIDRValue,
			"10.2.0.0/16",
			[]string{strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", "10.2.0.0/16", "-j", "ACCEPT"}, " ")},
		},
	}
	for _, test := range tests {
		r.IsGateway = test.isGateway
		tep.Status.LocalRemappedPodCIDR = test.localRemappedPodCIDR
		tep.Status.RemoteRemappedPodCIDR = test.remoteRemappedPodCIDR
		newRules, err := r.GetPostroutingRules(tep)
		assert.Nil(t, err)
		assert.Equal(t, len(test.rules), len(newRules))
		assert.Equal(t, test.rules, newRules)
	}
}

func TestRouteController_InsertRulesIfNotPresent(t *testing.T) {
	r := getRouteController()
	clusterID := "routeOperatorUnitTests"
	tests := []struct {
		table string
		chain string
		rules []string
	}{
		{
			"testTable",
			"testChain",
			[]string{"-d 10.2.0.0/16 -j LIQO-PSTRT-CLS-9ed4d9bd", " ! -s 10.245.0.0/16 -d 10.2.0.0/16 -j SNAT --to-source 10.245.0.0", " 10.245.0.0/16 -d 10.2.0.0/16 -j ACCEPT"},
		},
		{
			"testTable",
			"testChain",
			[]string{"-d 10.2.0.0/16 -j LIQO-PSTRT-CLS-9ed4d9bd", " ! -s 10.245.0.0/16 -d 10.2.0.0/16 -j SNAT --to-source 10.245.0.0", " 10.245.0.0/16 -d 10.2.0.0/16 -j ACCEPT", "! -s 10.2.0.0/16 -d 10.245.0.0/16 -j SNAT --to-source 10.2.0.0", "-s 10.2.0.0/16 -d 10.245.0.0/16 -j ACCEPT"},
		},
	}
	for _, test := range tests {
		_ = r.InsertRulesIfNotPresent(clusterID, test.table, test.chain, test.rules)
		for _, testRule := range test.rules {
			present := false
			for _, rule := range ip.Rules {
				if testRule == strings.Join(rule.RuleSpec, " ") {
					present = true
				}
			}
			assert.True(t, present)
		}
	}
}

func TestRouteController_UpdateRulesPerChain(t *testing.T) {
	r := getRouteController()
	existingRules := struct {
		clusterID string
		table     string
		chain     string
		rules     []string
	}{
		"routeOperatorUnitTests",
		"testTable",
		"testChain",
		[]string{"-d 10.2.0.0/16 -j LIQO-PSTRT-CLS-9ed4d9bd", "! -s 10.245.0.0/16 -d 10.2.0.0/16 -j SNAT --to-source 10.245.0.0", "10.245.0.0/16 -d 10.2.0.0/16 -j ACCEPT", "! -s 10.2.0.0/16 -d 10.245.0.0/16 -j SNAT --to-source 10.2.0.0", "-s 10.2.0.0/16 -d 10.245.0.0/16 -j ACCEPT"},
	}
	newRules := struct {
		clusterID string
		table     string
		chain     string
		rules     []string
	}{
		"routeOperatorUnitTests",
		"testTable",
		"testChain",
		[]string{"-d 10.2.0.0/16 -j LIQO-PSTRT-CLS-9ed4d9bd", "! -s 10.245.0.0/16 -d 10.2.0.0/16 -j SNAT --to-source 10.245.0.0", "10.245.0.0/16 -d 10.2.0.0/16 -j ACCEPT", "10.245.0.0/16 -d 10.89.0.0/16 -j ACCEPT"},
	}
	//first we insert the existing rules
	_ = r.InsertRulesIfNotPresent(existingRules.clusterID, existingRules.table, existingRules.chain, existingRules.rules)
	//here we update the rules twice, the first one to check that the outdated rules are removed
	//second one to check that the function is idempotent when no rules are outdated
	for i := 0; i < 2; i++ {
		_ = r.UpdateRulesPerChain(newRules.clusterID, newRules.chain, newRules.table, existingRules.rules, newRules.rules)
		assert.Equal(t, len(newRules.rules), len(ip.Rules))
		for _, testRule := range newRules.rules {
			present := false
			for _, rule := range ip.Rules {
				if testRule == strings.Join(rule.RuleSpec, " ") {
					present = true
				}
			}
			assert.True(t, present)
		}
	}
}

func TestRouteController_ListRulesInChain(t *testing.T) {
	r := getRouteController()
	//here we emulute how the rules are present when listing theme with option -S i.e iptables -S chain -t table
	existingRules := struct {
		clusterID     string
		table         string
		chain         string
		rules         []string
		expectedRules []string
	}{
		"routeOperatorUnitTests",
		"nat",
		"LIQO-PSTRT-CLS-d7cd85f9",
		[]string{"-N LIQO-PSTRT-CLS-d7cd85f9", "-A LIQO-PSTRT-CLS-d7cd85f9 ! -s 10.2.0.0/16 -d 10.245.0.0/16 -j SNAT --to-source 10.2.0.0", "-A LIQO-PSTRT-CLS-d7cd85f9 -s 10.2.0.0/16 -d 10.245.0.0/16 -j ACCEPT"},
		[]string{"! -s 10.2.0.0/16 -d 10.245.0.0/16 -j SNAT --to-source 10.2.0.0", "-s 10.2.0.0/16 -d 10.245.0.0/16 -j ACCEPT"},
	}
	//first we insert the existing rules
	_ = r.InsertRulesIfNotPresent(existingRules.clusterID, existingRules.table, existingRules.chain, existingRules.rules)
	rules, _ := r.ListRulesInChain(existingRules.table, existingRules.chain)
	assert.Equal(t, 2, len(rules))
	assert.Equal(t, existingRules.expectedRules, rules)
}

func TestRouteController_GetChainRulespecs(t *testing.T) {
	r := getRouteController()
	tep := GetTunnelEndpointCR()
	tests := []struct {
		localRemappedPodCIDR   string
		expectedNumberofChains int
	}{
		{"10.1.0.0/16",
			4,
		},
		{
			defaultPodCIDRValue,
			3,
		},
	}
	for _, test := range tests {
		tep.Status.LocalRemappedPodCIDR = test.localRemappedPodCIDR
		chainRulespecs := r.GetChainRulespecs(tep)
		assert.Equal(t, test.expectedNumberofChains, len(chainRulespecs))
	}
}

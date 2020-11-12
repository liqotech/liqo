package liqonet

import (
	"fmt"
	"github.com/coreos/go-iptables/iptables"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"k8s.io/klog/v2"
	"net"
	"strings"
)

const (
	LiqonetPostroutingChain              = "LIQO-POSTROUTING"
	LiqonetPreroutingChain               = "LIQO-PREROUTING"
	LiqonetForwardingChain               = "LIQO-FORWARD"
	LiqonetInputChain                    = "LIQO-INPUT"
	LiqonetPostroutingClusterChainPrefix = "LIQO-PSTRT-CLS-"
	LiqonetPreroutingClusterChainPrefix  = "LIQO-PRRT-CLS-"
	LiqonetForwardingClusterChainPrefix  = "LIQO-FRWD-CLS-"
	LiqonetInputClusterChainPrefix       = "LIQO-INPT-CLS-"
	NatTable                             = "nat"
	FilterTable                          = "filter"
	defaultPodCIDRValue                  = "None"
)

type IPtableRule struct {
	Table    string
	Chain    string
	RuleSpec []string
}

type IPTableChain struct {
	Table string
	Name  string
}

type rulespec struct {
	chainName string
	rulespec  string
	table     string
	chain     string
}

type IPTables interface {
	Insert(table string, chain string, pos int, rulespec ...string) error
	Delete(table string, chain string, rulespec ...string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
	ListChains(table string) ([]string, error)
	NewChain(table string, chain string) error
	List(table, chain string) ([]string, error)
	AppendUnique(table string, chain string, rulespec ...string) error
	ClearChain(table, chain string) error
	DeleteChain(table, chain string) error
}

type IPTablesHandler struct {
	ipt IPTables
}

func NewIPTablesHandler() (IPTablesHandler, error) {
	ipt, err := iptables.New()
	if err != nil {
		return IPTablesHandler{}, err
	}
	return IPTablesHandler{
		ipt: ipt,
	}, err
}

//this function is called at startup of the operator
//here we:
//create LIQONET-FORWARD in the filter table and insert it in the "FORWARD" chain
//create LIQONET-POSTROUTING in the nat table and insert it in the "POSTROUTING" chain
//create LIQONET-INPUT in the filter table and insert it in the input chain
//insert the rulespec which allows in input all the udp traffic incoming for the vxlan in the LIQONET-INPUT chain
func (h IPTablesHandler) CreateAndEnsureIPTablesChains(defaultIfaceName string) error {
	var err error
	ipt := h.ipt
	//creating LIQONET-POSTROUTING chain
	if err = createIptablesChainIfNotExists(ipt, NatTable, LiqonetPostroutingChain); err != nil {
		return err
	}
	//installing rulespec which forwards all traffic to LIQONET-POSTROUTING chain
	forwardToLiqonetPostroutingRuleSpec := []string{"-j", LiqonetPostroutingChain}
	if err = insertIptablesRulespecIfNotExists(ipt, NatTable, "POSTROUTING", forwardToLiqonetPostroutingRuleSpec); err != nil {
		return err
	}
	//creating LIQONET-PREROUTING chain
	if err = createIptablesChainIfNotExists(ipt, NatTable, LiqonetPreroutingChain); err != nil {
		return err
	}
	//installing rulespec which forwards all traffic to LIQONET-PREROUTING chain
	forwardToLiqonetPreroutingRuleSpec := []string{"-j", LiqonetPreroutingChain}
	if err = insertIptablesRulespecIfNotExists(ipt, NatTable, "PREROUTING", forwardToLiqonetPreroutingRuleSpec); err != nil {
		return err
	}

	//creating LIQONET-FORWARD chain
	if err = createIptablesChainIfNotExists(ipt, FilterTable, LiqonetForwardingChain); err != nil {
		return err
	}
	//installing rulespec which forwards all traffic to LIQONET-FORWARD chain
	forwardToLiqonetForwardRuleSpec := []string{"-j", LiqonetForwardingChain}
	if err = insertIptablesRulespecIfNotExists(ipt, FilterTable, "FORWARD", forwardToLiqonetForwardRuleSpec); err != nil {
		return err
	}
	//creating LIQONET-INPUT chain
	if err = createIptablesChainIfNotExists(ipt, FilterTable, LiqonetInputChain); err != nil {
		return err
	}

	//installing rulespec which forwards all udp incoming traffic to LIQONET-INPUT chain
	forwardToLiqonetInputSpec := []string{"-p", "udp", "-m", "udp", "-j", LiqonetInputChain}
	if err = insertIptablesRulespecIfNotExists(ipt, FilterTable, "INPUT", forwardToLiqonetInputSpec); err != nil {
		return err
	}

	//installing rulespec which allows udp traffic with destination port the VXLAN port
	//we put it here because this rulespec is independent from the remote cluster.
	//we don't save this rulespec it will be removed when the chains are flushed at exit time
	//TODO: wg port number use a variable and set it right value
	outTrafficRuleSpec := []string{"-o", defaultIfaceName, "-j", "MASQUERADE"}
	exists, err := ipt.Exists(NatTable, "POSTROUTING", outTrafficRuleSpec...)
	if err != nil {
		return err
	}
	if !exists {
		if err = ipt.AppendUnique(NatTable, "POSTROUTING", outTrafficRuleSpec...); err != nil {
			return err
		}
		klog.Infof("installed rulespec '%s' in chain %s of table %s", strings.Join(outTrafficRuleSpec, " "), "FORWARDING", NatTable)
	}
	return nil
}

func (h IPTablesHandler) EnsureChainRulespecs(tep *netv1alpha1.TunnelEndpoint) error {
	chains := getChainRulespecs(tep)
	clusterID := tep.Spec.ClusterID
	for _, chain := range chains {
		//create chain for the peering cluster if it does not exist
		err := createIptablesChainIfNotExists(h.ipt, chain.table, chain.chainName)
		if err != nil {
			klog.Errorf("%s -> unable to create chain %s: %s", clusterID, chain.chainName, err)
			return err
		}
		existingRules, err := h.ipt.List(chain.table, chain.chain)
		if err != nil {
			klog.Errorf("%s -> unable to list rules in chain %s from table %s: %s", clusterID, chain.chain, chain.table, err)
			return err
		}
		for _, rule := range existingRules {
			if strings.Contains(rule, chain.chainName) {
				if !strings.Contains(rule, chain.rulespec) {
					if err := h.ipt.Delete(chain.table, chain.chain, strings.Split(rule, " ")[2:]...); err != nil {
						return err
					}
					klog.Infof("%s -> removing outdated rule '%s' from chain %s in table %s", clusterID, rule, chain.chain, chain.table)
				}
			}
		}
		err = insertRulesIfNotPresent(h.ipt, clusterID, chain.table, chain.chain, []string{chain.rulespec})
		if err != nil {
			return err
		}
	}
	return nil
}

func (h IPTablesHandler) EnsurePostroutingRules(isGateway bool, tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	postRoutingChain := strings.Join([]string{LiqonetPostroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	rules, err := getPostroutingRules(isGateway, tep)
	if err != nil {
		return err
	}
	//list rules in the chain
	existingRules, err := listRulesInChain(h.ipt, NatTable, postRoutingChain)
	if err != nil {
		klog.Errorf("%s -> unable to list rules for chain %s in table %s: %s", clusterID, postRoutingChain, NatTable, err)
		return err
	}
	return updateRulesPerChain(h.ipt, clusterID, postRoutingChain, NatTable, existingRules, rules)
}

func (h IPTablesHandler) EnsurePreroutingRules(tep *netv1alpha1.TunnelEndpoint) error {
	localPodCIDR := tep.Status.LocalPodCIDR
	//check if we need to NAT the incoming traffic from the peering cluster
	localRemappedPodCIDR, remotePodCIDR := GetPodCIDRS(tep)
	if localRemappedPodCIDR == defaultPodCIDRValue {
		return nil
	}
	clusterID := tep.Spec.ClusterID
	preRoutingChain := strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	//list rules in the chain
	existingRules, err := listRulesInChain(h.ipt, NatTable, preRoutingChain)
	if err != nil {
		klog.Errorf("%s -> unable to list rules for chain %s in table %s: %s", clusterID, preRoutingChain, NatTable, err)
		return err
	}
	rules := []string{
		strings.Join([]string{"-s", remotePodCIDR, "-d", localRemappedPodCIDR, "-j", "NETMAP", "--to", localPodCIDR}, " "),
	}
	return updateRulesPerChain(h.ipt, clusterID, preRoutingChain, NatTable, existingRules, rules)
}

func createIptablesChainIfNotExists(ipt IPTables, table string, newChain string) error {
	//get existing chains
	chains_list, err := ipt.ListChains(table)
	if err != nil {
		return fmt.Errorf("imposible to retrieve chains in table -> %s : %v", table, err)
	}
	//if the chain exists do nothing
	for _, chain := range chains_list {
		if chain == newChain {
			return nil
		}
	}
	//if we come here the chain does not exist so we insert it
	err = ipt.NewChain(table, newChain)
	if err != nil {
		return fmt.Errorf("unable to create %s chain in %s table: %v", newChain, table, err)
	}
	klog.Infof("created chain %s in table %s", newChain, table)
	return nil
}

func insertIptablesRulespecIfNotExists(ipt IPTables, table string, chain string, ruleSpec []string) error {
	//get the list of rulespecs for the specified chain
	rulesList, err := ipt.List(table, chain)
	if err != nil {
		return fmt.Errorf("unable to get the rules in %s chain in %s table : %v", chain, table, err)
	}
	//here we check if the rulespec exists and at the same time if it exists more then once
	numOccurrences := 0
	for _, rule := range rulesList {
		if strings.Contains(rule, strings.Join(ruleSpec, " ")) {
			numOccurrences++
		}
	}
	//if the occurrences if greater then one, remove the rulespec
	if numOccurrences > 1 {
		for i := 0; i < numOccurrences; i++ {
			if err = ipt.Delete(table, chain, ruleSpec...); err != nil {
				return fmt.Errorf("unable to delete iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
			}
		}
		if err = ipt.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
	} else if numOccurrences == 1 {
		//if the occurrence is one then check the position and if not at the first one we delete and reinsert it
		if strings.Contains(rulesList[0], strings.Join(ruleSpec, " ")) {
			return nil
		}
		if err = ipt.Delete(table, chain, ruleSpec...); err != nil {
			return fmt.Errorf("unable to delete iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
		if err = ipt.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
		return nil
	} else if numOccurrences == 0 {
		//if the occurrence is zero then insert the rule in first position
		if err = ipt.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
		klog.Infof("installed rulespec '%s' in chain %s of table %s", strings.Join(ruleSpec, " "), chain, table)
	}
	return nil
}

func updateRulesPerChain(ipt IPTables, clusterID, chain, table string, existingRules, newRules []string) error {
	//remove the outdated rules
	//if the chain has been newly created than the for loop will do nothing
	for _, existingRule := range existingRules {
		outdated := true
		for _, newRule := range newRules {
			if existingRule == newRule {
				outdated = false
			}
		}
		if outdated {
			if err := ipt.Delete(table, chain, strings.Split(existingRule, " ")...); err != nil {
				return err
			}
			klog.Infof("%s -> removing outdated rule '%s' from chain %s in table %s", clusterID, existingRule, chain, table)
		}
	}
	if err := insertRulesIfNotPresent(ipt, clusterID, table, chain, newRules); err != nil {
		return err
	}
	return nil
}

func listRulesInChain(ipt IPTables, table, chain string) ([]string, error) {
	existingRules, err := ipt.List(table, chain)
	if err != nil {
		return nil, err
	}
	rules := make([]string, 0)
	ruleToRemove := strings.Join([]string{"-N", chain}, " ")
	for _, rule := range existingRules {
		if rule != ruleToRemove {
			tmp := strings.Split(rule, " ")
			rules = append(rules, strings.Join(tmp[2:], " "))
		}
	}
	return rules, nil
}

func insertRulesIfNotPresent(ipt IPTables, clusterID, table, chain string, rules []string) error {
	for _, rule := range rules {
		exists, err := ipt.Exists(table, chain, strings.Split(rule, " ")...)
		if err != nil {
			klog.Errorf("%s -> unable to check if rule '%s' exists in chain %s in table %s", clusterID, rule, chain, table)
			return err
		}
		if !exists {
			if err := ipt.AppendUnique(table, chain, strings.Split(rule, " ")...); err != nil {
				return err
			}
			klog.Infof("%s -> inserting rule '%s' in chain %s in table %s", clusterID, rule, chain, table)
		}
	}
	return nil
}

func GetPodCIDRS(tep *netv1alpha1.TunnelEndpoint) (string, string) {
	var remotePodCIDR, localRemappedPodCIDR string
	if tep.Status.RemoteRemappedPodCIDR != defaultPodCIDRValue {
		remotePodCIDR = tep.Status.RemoteRemappedPodCIDR
	} else {
		remotePodCIDR = tep.Spec.PodCIDR
	}
	localRemappedPodCIDR = tep.Status.LocalRemappedPodCIDR
	return localRemappedPodCIDR, remotePodCIDR
}

func getPostroutingRules(isGateway bool, tep *netv1alpha1.TunnelEndpoint) ([]string, error) {
	clusterID := tep.Spec.ClusterID
	localPodCIDR := tep.Status.LocalPodCIDR
	localRemappedPodCIDR, remotePodCIDR := GetPodCIDRS(tep)
	if isGateway {
		if localRemappedPodCIDR != defaultPodCIDRValue {
			//we get the first IP address from the podCIDR of the local cluster
			//in this case it is the podCIDR to which the local podCIDR has bee remapped by the remote peering cluster
			natIP, _, err := net.ParseCIDR(localRemappedPodCIDR)
			if err != nil {
				klog.Errorf("%s -> unable to get the IP from localPodCidr %s used to NAT the traffic from localhosts to remote hosts", clusterID, localRemappedPodCIDR)
				return nil, err
			}
			return []string{
				strings.Join([]string{"-s", localPodCIDR, "-d", remotePodCIDR, "-j", "NETMAP", "--to", localRemappedPodCIDR}, " "),
				strings.Join([]string{"!", "-s", localPodCIDR, "-d", remotePodCIDR, "-j", "SNAT", "--to-source", natIP.String()}, " "),
			}, nil
		}
		//we get the first IP address from the podCIDR of the local cluster
		natIP, _, err := net.ParseCIDR(localPodCIDR)
		if err != nil {
			klog.Errorf("%s -> unable to get the IP from localPodCidr %s used to NAT the traffic from localhosts to remote hosts", clusterID, tep.Spec.PodCIDR)
			return nil, err
		}
		return []string{
			strings.Join([]string{"!", "-s", localPodCIDR, "-d", remotePodCIDR, "-j", "SNAT", "--to-source", natIP.String()}, " "),
		}, nil
	}
	return []string{
		strings.Join([]string{"-d", remotePodCIDR, "-j", "ACCEPT"}, " "),
	}, nil
}

func getChainRulespecs(tep *netv1alpha1.TunnelEndpoint) []rulespec {
	clusterID := tep.Spec.ClusterID
	localRemappedPodCIDR, remotePodCIDR := GetPodCIDRS(tep)
	postRoutingChain := strings.Join([]string{LiqonetPostroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	preRoutingChain := strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	forwardChain := strings.Join([]string{LiqonetForwardingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	inputChain := strings.Join([]string{LiqonetInputClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	ruleSpecs := []rulespec{
		{
			postRoutingChain,
			strings.Join([]string{"-d", remotePodCIDR, "-j", postRoutingChain}, " "),
			NatTable,
			LiqonetPostroutingChain,
		},
		{
			forwardChain,
			strings.Join([]string{"-d", remotePodCIDR, "-j", forwardChain}, " "),
			FilterTable,
			LiqonetForwardingChain,
		},
		{
			inputChain,
			strings.Join([]string{"-d", remotePodCIDR, "-j", inputChain}, " "),
			FilterTable,
			LiqonetInputChain,
		},
	}
	if localRemappedPodCIDR != defaultPodCIDRValue {
		ruleSpecs = append(ruleSpecs, rulespec{
			preRoutingChain,
			strings.Join([]string{"-d", localRemappedPodCIDR, "-j", preRoutingChain}, " "),
			NatTable,
			LiqonetPreroutingChain,
		})
		return ruleSpecs
	}
	return ruleSpecs
}

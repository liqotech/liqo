package liqonet

import (
	"fmt"
	"net"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/util/slice"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	// LiqonetPostroutingChain is the name of the postrouting chain inserted by liqo.
	LiqonetPostroutingChain = "LIQO-POSTROUTING"
	// LiqonetPreroutingChain is the naame of the prerouting chain inserted by liqo.
	LiqonetPreroutingChain = "LIQO-PREROUTING"
	// LiqonetForwardingChain is the name of the forwarding chain inserted by liqo.
	LiqonetForwardingChain = "LIQO-FORWARD"
	// LiqonetInputChain is the name of the input chain inserted by liqo.
	LiqonetInputChain = "LIQO-INPUT"
	// LiqonetPostroutingClusterChainPrefix the prefix used to name the postrouting chains for a specific cluster.
	LiqonetPostroutingClusterChainPrefix = "LIQO-PSTRT-CLS-"
	// LiqonetPreroutingClusterChainPrefix prefix used to name the prerouting chains for a specific cluster.
	LiqonetPreroutingClusterChainPrefix = "LIQO-PRRT-CLS-"
	// LiqonetForwardingClusterChainPrefix prefix used to name the forwarding chains for a specific cluster.
	LiqonetForwardingClusterChainPrefix = "LIQO-FRWD-CLS-"
	// LiqonetInputClusterChainPrefix prefix used to name the input chains for a specific cluster.
	LiqonetInputClusterChainPrefix = "LIQO-INPT-CLS-"
	// NatTable constant used for the "nat" table.
	NatTable = "nat"
	// FilterTable constant used for the "filter" table.
	FilterTable         = "filter"
	defaultPodCIDRValue = "None"
)

// IPTableRule struct that holds all the information of an iptables rule.
type IPTableRule struct {
	Table    string
	Chain    string
	RuleSpec []string
}

// IPTableClusterChain Struct that holds all the information of an iptables cluster chain.
type IPTableClusterChain struct {
	Table        string
	WrapperChain string
	Name         string
}

// Struct that holds all the information of an iptables rulespec.
type rulespec struct {
	chainName string
	rulespec  string
	table     string
	chain     string
}

// IPTablesHandler a handler that exposes all the functions needed to configure the iptables chains and rules.
type IPTablesHandler struct {
	ipt iptables.IPTables
}

// NewIPTablesHandler return the iptables handler used to configure the iptables rules.
func NewIPTablesHandler() (IPTablesHandler, error) {
	ipt, err := iptables.New()
	if err != nil {
		return IPTablesHandler{}, err
	}
	return IPTablesHandler{
		ipt: *ipt,
	}, err
}

// CreateAndEnsureIPTablesChains function is called at startup of the operator.
// here we:
// create LIQONET-FORWARD in the filter table and insert it in the "FORWARD" chain.
// create LIQONET-POSTROUTING in the nat table and insert it in the "POSTROUTING" chain.
// create LIQONET-INPUT in the filter table and insert it in the input chain.
// insert the rulespec which allows in input all the udp traffic incoming for the vxlan in the LIQONET-INPUT chain.
func (h IPTablesHandler) CreateAndEnsureIPTablesChains(defaultIfaceName string) error {
	var err error
	ipt := h.ipt

	if defaultIfaceName == "" {
		return &WrongParameter{
			Argument:  "",
			Reason:    StringNotEmpty,
			Parameter: defaultIfaceName,
		}
	}
	if len(defaultIfaceName) > unix.IFNAMSIZ {
		return &WrongParameter{
			Argument:  fmt.Sprintf("%d", unix.IFNAMSIZ),
			Reason:    MinorOrEqual,
			Parameter: defaultIfaceName,
		}
	}
	// creating LIQONET-POSTROUTING chain
	if err = h.createIptablesChainIfNotExists(NatTable, LiqonetPostroutingChain); err != nil {
		return err
	}
	// installing rulespec which forwards all traffic to LIQONET-POSTROUTING chain
	forwardToLiqonetPostroutingRuleSpec := []string{"-j", LiqonetPostroutingChain}
	if err = h.insertIptablesRulespecIfNotExists(NatTable, "POSTROUTING", forwardToLiqonetPostroutingRuleSpec); err != nil {
		return err
	}
	// creating LIQONET-PREROUTING chain
	if err = h.createIptablesChainIfNotExists(NatTable, LiqonetPreroutingChain); err != nil {
		return err
	}
	// installing rulespec which forwards all traffic to LIQONET-PREROUTING chain
	forwardToLiqonetPreroutingRuleSpec := []string{"-j", LiqonetPreroutingChain}
	if err = h.insertIptablesRulespecIfNotExists(NatTable, "PREROUTING", forwardToLiqonetPreroutingRuleSpec); err != nil {
		return err
	}

	// creating LIQONET-FORWARD chain
	if err = h.createIptablesChainIfNotExists(FilterTable, LiqonetForwardingChain); err != nil {
		return err
	}
	// installing rulespec which forwards all traffic to LIQONET-FORWARD chain
	forwardToLiqonetForwardRuleSpec := []string{"-j", LiqonetForwardingChain}
	if err = h.insertIptablesRulespecIfNotExists(FilterTable, "FORWARD", forwardToLiqonetForwardRuleSpec); err != nil {
		return err
	}
	// creating LIQONET-INPUT chain
	if err = h.createIptablesChainIfNotExists(FilterTable, LiqonetInputChain); err != nil {
		return err
	}

	// installing rulespec which forwards all udp incoming traffic to LIQONET-INPUT chain
	forwardToLiqonetInputSpec := []string{"-p", "udp", "-m", "udp", "-j", LiqonetInputChain}
	if err = h.insertIptablesRulespecIfNotExists(FilterTable, "INPUT", forwardToLiqonetInputSpec); err != nil {
		return err
	}

	// installing rulespec which allows udp traffic with destination port the VXLAN port
	// we put it here because this rulespec is independent from the remote cluster.
	// we don't save this rulespec it will be removed when the chains are flushed at exit time
	// TODO: wg port number use a variable and set it right value
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

// EnsureChainRulespecsPerTep reads TunnelEndpoint resource and
// makes sure that the general chains for the given cluster exist.
// If the chains do not exist, they are created.
func (h IPTablesHandler) EnsureChainRulespecsPerTep(tep *netv1alpha1.TunnelEndpoint) error {
	chains, err := h.getChainRulespecsFromTep(tep)
	if err != nil {
		return err
	}
	clusterID := tep.Spec.ClusterID
	return h.ensureChainRulespecs(clusterID, chains)
}

// EnsureChainRulespecsPerNm reads NatMapping resource and
// makes sure that the prerouting chain for the given cluster exist.
// If the chain does not exist, it is created.
func (h IPTablesHandler) EnsureChainRulespecsPerNm(nm *netv1alpha1.NatMapping) error {
	chains, err := h.getChainRulespecsFromNp(nm)
	if err != nil {
		return err
	}
	clusterID := nm.Spec.ClusterID
	return h.ensureChainRulespecs(clusterID, chains)
}

// EnsureChainsPerCluster is used to be sure input, output, postrouting and prerouting chain for a given
// cluster are present in the NAT table and Filter table.
func (h IPTablesHandler) EnsureChainsPerCluster(clusterID string) error {
	chains := h.getChainsPerCluster(clusterID)
	for _, chain := range chains {
		if err := h.createIptablesChainIfNotExists(chain.Table, chain.Name); err != nil {
			return err
		}
	}
	return nil
}

func (h IPTablesHandler) ensureChainRulespecs(clusterID string, chains []rulespec) error {
	for _, chain := range chains {
		existingRules, err := h.ipt.List(chain.table, chain.chain)
		if err != nil {
			klog.Errorf("unable to list rules in chain %s from table %s: %s", chain.chain, chain.table, err)
			return err
		}
		for _, rule := range existingRules {
			if strings.Contains(rule, chain.chainName) {
				if !strings.Contains(rule, chain.rulespec) {
					if err := h.ipt.Delete(chain.table, chain.chain, strings.Split(rule, " ")[2:]...); err != nil {
						return err
					}
					klog.Infof("Removed outdated rule '%s' from chain %s in table %s", rule, chain.chain, chain.table)
				}
			}
		}
		err = h.insertRulesIfNotPresent(clusterID, chain.table, chain.chain, []string{chain.rulespec})
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveIPTablesConfigurationPerCluster clears and deletes input, forward, prerouting and postrouting chains
// for a remote cluster. In order to remove them, function first clears related rules in LIQO-POSTROUTING,
// LIQO-PREROUTING, LIQO-FORWARD and LIQO-INPUT.
func (h IPTablesHandler) RemoveIPTablesConfigurationPerCluster(clusterID string) error {
	err := h.removeChainRulesPerCluster(clusterID)
	if err != nil {
		return fmt.Errorf("cannot remove chain rules for cluster %s: %w", clusterID, err)
	}
	err = h.removeChainsPerCluster(clusterID)
	if err != nil {
		return fmt.Errorf("cannot remove chains per cluster")
	}
	return nil
}

// Function that returns the set of chains related to a remote cluster.
func (h IPTablesHandler) getChainsPerCluster(clusterID string) map[string]IPTableClusterChain {
	return map[string]IPTableClusterChain{
		"FORWARD": {
			Table:        FilterTable,
			WrapperChain: LiqonetForwardingChain,
			Name:         strings.Join([]string{LiqonetForwardingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, ""),
		},
		"INPUT": {
			Table:        FilterTable,
			WrapperChain: LiqonetInputChain,
			Name:         strings.Join([]string{LiqonetInputClusterChainPrefix, strings.Split(clusterID, "-")[0]}, ""),
		},
		"POSTROUTING": {
			Table:        NatTable,
			WrapperChain: LiqonetPostroutingChain,
			Name:         strings.Join([]string{LiqonetPostroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, ""),
		},
		"PREROUTING": {
			Table:        NatTable,
			WrapperChain: LiqonetPreroutingChain,
			Name:         strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, ""),
		},
	}
}

// Function removes rules related to a remote cluster from chains LIQO-POSTROUTING, LIQO-PREROUTING,
// LIQO-FORWARD, LIQO-INPUT.
func (h IPTablesHandler) removeChainRulesPerCluster(clusterID string) error {
	clusterChains := h.getChainsPerCluster(clusterID)
	for _, clusterChain := range clusterChains {
		table := clusterChain.Table
		chain := clusterChain.WrapperChain
		// Using List instead of Exists makes the code more verbose, but there's
		// the guarante that every rule that contains the clusterID is removed.
		rules, err := h.listRulesInChain(table, chain)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if !strings.Contains(rule, clusterChain.Name) {
				continue
			}
			// If rule contains the chain name, remove it
			err := h.ipt.Delete(table, chain, strings.Split(rule, " ")...)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Function removes all the chains (and contained rules) related to a remote cluster.
func (h IPTablesHandler) removeChainsPerCluster(clusterID string) error {
	// Get existing chains
	existingRulesNAT, err := h.ipt.ListChains(NatTable)
	if err != nil {
		return err
	}
	// Get existing chains
	existingRulesFilter, err := h.ipt.ListChains(FilterTable)
	if err != nil {
		return err
	}
	// Get chains per cluster
	chains := h.getChainsPerCluster(clusterID)
	// For each cluster chain, check if it exists.
	// If it does exist, then remove it. Otherwise do nothing.
	for _, chain := range chains {
		if chain.Table == NatTable && slice.ContainsString(existingRulesNAT, chain.Name, nil) {
			if err := h.ipt.ClearChain(NatTable, chain.Name); err != nil {
				return err
			}
			if err := h.ipt.DeleteChain(NatTable, chain.Name); err != nil {
				return err
			}
			continue
		}
		if chain.Table == FilterTable && slice.ContainsString(existingRulesFilter, chain.Name, nil) {
			if err := h.ipt.ClearChain(FilterTable, chain.Name); err != nil {
				return err
			}
			if err := h.ipt.DeleteChain(FilterTable, chain.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsurePostroutingRules makes sure that the postrouting rules for a given cluster are in place and updated.
func (h IPTablesHandler) EnsurePostroutingRules(isGateway bool, tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	postRoutingChain := strings.Join([]string{LiqonetPostroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	rules, err := getPostroutingRules(isGateway, tep)
	if err != nil {
		return err
	}
	// list rules in the chain
	existingRules, err := h.listRulesInChain(NatTable, postRoutingChain)
	if err != nil {
		klog.Errorf("%s -> unable to list rules for chain %s in table %s: %s", clusterID, postRoutingChain, NatTable, err)
		return err
	}
	return h.updateRulesPerChain(clusterID, postRoutingChain, NatTable, existingRules, rules)
}

// EnsurePreroutingRulesPerTep makes sure that the prerouting rules extracted from TunnelEndpoint
// resource for a given cluster are in place and updated.
func (h IPTablesHandler) EnsurePreroutingRulesPerTep(tep *netv1alpha1.TunnelEndpoint) error {
	localPodCIDR := tep.Status.LocalPodCIDR
	// check if we need to NAT the incoming traffic from the peering cluster
	localRemappedPodCIDR, remotePodCIDR := GetPodCIDRS(tep)
	if localRemappedPodCIDR == liqoconst.DefaultCIDRValue {
		return nil
	}
	clusterID := tep.Spec.ClusterID
	preRoutingChain := strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	// list rules in the chain
	existingRules, err := h.listRulesInChain(NatTable, preRoutingChain)
	if err != nil {
		klog.Errorf("unable to list rules for chain %s in table %s: %s", preRoutingChain, NatTable, err)
		return err
	}
	rules := []string{
		strings.Join([]string{"-s", remotePodCIDR, "-d", localRemappedPodCIDR, "-j", "NETMAP", "--to", localPodCIDR}, " "),
	}
	return h.updateRulesPerChain(clusterID, preRoutingChain, NatTable, existingRules, rules)
}

// EnsurePreroutingRulesPerNm makes sure that the prerouting rules extracted from NatMapping
// resource for a given cluster are in place and updated.
func (h IPTablesHandler) EnsurePreroutingRulesPerNm(nm *netv1alpha1.NatMapping) error {
	// List rules in chain
	clusterID := nm.Spec.ClusterID
	preRoutingChain := strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	existingRules, err := h.listRulesInChain(NatTable, preRoutingChain)
	if err != nil {
		klog.Errorf("cannot list rules for chain %s in table %s: %s", preRoutingChain, NatTable, err.Error())
		return err
	}
	// Forge rules from mappings
	rules := make([]string, 0)
	for oldIP, newIP := range nm.Spec.Mappings {
		rules = append(rules, strings.Join([]string{"-s", nm.Spec.PodCIDR, "-d", newIP, "-j", "DNAT", "--to", oldIP}, " "))
	}
	return h.updateRulesPerChain(clusterID, preRoutingChain, NatTable, existingRules, rules)
}

func (h IPTablesHandler) createIptablesChainIfNotExists(table, newChain string) error {
	// get existing chains
	chainsList, err := h.ipt.ListChains(table)
	if err != nil {
		return fmt.Errorf("cannot retrieve chains in table -> %s : %w", table, err)
	}
	// if the chain exists do nothing
	for _, chain := range chainsList {
		if chain == newChain {
			return nil
		}
	}
	// if we come here the chain does not exist so we insert it
	err = h.ipt.NewChain(table, newChain)
	if err != nil {
		return fmt.Errorf("unable to create %s chain in %s table: %w", newChain, table, err)
	}
	klog.Infof("Created chain %s in table %s", newChain, table)
	return nil
}

func (h IPTablesHandler) insertIptablesRulespecIfNotExists(table, chain string, ruleSpec []string) error {
	// get the list of rulespecs for the specified chain
	rulesList, err := h.ipt.List(table, chain)
	if err != nil {
		return fmt.Errorf("unable to get the rules in %s chain in %s table : %w", chain, table, err)
	}
	// here we check if the rulespec exists and at the same time if it exists more then once
	numOccurrences := 0
	for _, rule := range rulesList {
		if strings.Contains(rule, strings.Join(ruleSpec, " ")) {
			numOccurrences++
		}
	}
	// if the occurrences if greater then one, remove the rulespec
	if numOccurrences > 1 {
		for i := 0; i < numOccurrences; i++ {
			if err = h.ipt.Delete(table, chain, ruleSpec...); err != nil {
				return fmt.Errorf("unable to delete iptable rule \"%s\": %w", strings.Join(ruleSpec, " "), err)
			}
		}
		if err = h.ipt.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %w", strings.Join(ruleSpec, " "), err)
		}
	}
	if numOccurrences == 1 {
		// if the occurrence is one then check the position and if not at the first one we delete and reinsert it
		if strings.Contains(rulesList[0], strings.Join(ruleSpec, " ")) {
			return nil
		}
		if err = h.ipt.Delete(table, chain, ruleSpec...); err != nil {
			return fmt.Errorf("unable to delete iptable rule \"%s\": %w", strings.Join(ruleSpec, " "), err)
		}
		if err = h.ipt.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %w", strings.Join(ruleSpec, " "), err)
		}
		return nil
	}
	if numOccurrences == 0 {
		// if the occurrence is zero then insert the rule in first position
		if err = h.ipt.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to insert iptable rule \"%s\": %w", strings.Join(ruleSpec, " "), err)
		}
		klog.Infof("installed rulespec '%s' in chain %s of table %s", strings.Join(ruleSpec, " "), chain, table)
	}
	return nil
}

func (h IPTablesHandler) updateRulesPerChain(clusterID, chain, table string, existingRules, newRules []string) error {
	// If the chain has been newly created than the for loop will do nothing
	for _, existingRule := range existingRules {
		// If an existing rule does exist in the set of new rules, then it's outdated and has to be removed.
		outdated := !slice.ContainsString(newRules, existingRule, nil)
		if !outdated {
			continue
		}
		if err := h.ipt.Delete(table, chain, strings.Split(existingRule, " ")...); err != nil {
			return err
		}
		klog.Infof("Removing outdated rule '%s' from chain %s in table %s", existingRule, chain, table)
	}
	if err := h.insertRulesIfNotPresent(clusterID, table, chain, newRules); err != nil {
		return err
	}
	return nil
}

// Function used to adjust the result returned by List of go-iptables.
func (h IPTablesHandler) listRulesInChain(table, chain string) ([]string, error) {
	existingRules, err := h.ipt.List(table, chain)
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

func (h IPTablesHandler) insertRulesIfNotPresent(clusterID, table, chain string, rules []string) error {
	for _, rule := range rules {
		exists, err := h.ipt.Exists(table, chain, strings.Split(rule, " ")...)
		if err != nil {
			klog.Errorf("unable to check if rule '%s' exists in chain %s in table %s: %w", rule, chain, table, err)
			return err
		}
		if !exists {
			if err := h.ipt.AppendUnique(table, chain, strings.Split(rule, " ")...); err != nil {
				return err
			}
			klog.Infof("Inserting rule '%s' in chain %s in table %s", rule, chain, table)
		}
	}
	return nil
}

// GetPodCIDRS for a given tep the function retrieves the values for localPodCIDR and remotePodCIDR.
// Their values depend if the NAT is required or not.
func GetPodCIDRS(tep *netv1alpha1.TunnelEndpoint) (localRemappedPodCIDR, remotePodCIDR string) {
	if tep.Status.RemoteNATPodCIDR != defaultPodCIDRValue {
		remotePodCIDR = tep.Status.RemoteNATPodCIDR
	} else {
		remotePodCIDR = tep.Spec.PodCIDR
	}
	localRemappedPodCIDR = tep.Status.LocalNATPodCIDR
	return localRemappedPodCIDR, remotePodCIDR
}

func getPostroutingRules(isGateway bool, tep *netv1alpha1.TunnelEndpoint) ([]string, error) {
	clusterID := tep.Spec.ClusterID
	localPodCIDR := tep.Status.LocalPodCIDR
	localRemappedPodCIDR, remotePodCIDR := GetPodCIDRS(tep)
	if isGateway {
		if localRemappedPodCIDR != defaultPodCIDRValue {
			// Get the first IP address from the podCIDR of the local cluster
			// in this case it is the podCIDR to which the local podCIDR has bee remapped by the remote peering cluster
			natIP, _, err := net.ParseCIDR(localRemappedPodCIDR)
			if err != nil {
				klog.Errorf("Unable to get the IP from localPodCidr %s for remote cluster %s used to NAT the traffic from localhosts to remote hosts",
					localRemappedPodCIDR, clusterID)
				return nil, err
			}
			return []string{
				strings.Join([]string{"-s", localPodCIDR, "-d", remotePodCIDR, "-j", "NETMAP", "--to", localRemappedPodCIDR}, " "),
				strings.Join([]string{"!", "-s", localPodCIDR, "-d", remotePodCIDR, "-j", "SNAT", "--to-source", natIP.String()}, " "),
			}, nil
		}
		// Get the first IP address from the podCIDR of the local cluster
		natIP, _, err := net.ParseCIDR(localPodCIDR)
		if err != nil {
			klog.Errorf("Unable to get the IP from localPodCidr %s for cluster %s used to NAT the traffic from localhosts to remote hosts",
				tep.Spec.PodCIDR, clusterID)
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

func (h IPTablesHandler) getChainRulespecsFromTep(tep *netv1alpha1.TunnelEndpoint) ([]rulespec, error) {
	clusterID := tep.Spec.ClusterID
	localRemappedPodCIDR, remotePodCIDR := GetPodCIDRS(tep)
	if err := IsValidCIDR(localRemappedPodCIDR); localRemappedPodCIDR != liqoconst.DefaultCIDRValue &&
		err != nil {
		return nil, &WrongParameter{
			Reason:    ValidCIDR,
			Parameter: localRemappedPodCIDR,
		}
	}
	if err := IsValidCIDR(remotePodCIDR); err != nil {
		return nil, &WrongParameter{
			Reason:    ValidCIDR,
			Parameter: remotePodCIDR,
		}
	}
	if clusterID == "" {
		return nil, &WrongParameter{
			Reason: StringNotEmpty,
		}
	}
	clusterChains := h.getChainsPerCluster(clusterID)
	ruleSpecs := []rulespec{
		{
			clusterChains["POSTROUTING"].Name,
			strings.Join([]string{"-d", remotePodCIDR, "-j", clusterChains["POSTROUTING"].Name}, " "),
			clusterChains["POSTROUTING"].Table,
			clusterChains["POSTROUTING"].WrapperChain,
		},
		{
			clusterChains["FORWARD"].Name,
			strings.Join([]string{"-d", remotePodCIDR, "-j", clusterChains["FORWARD"].Name}, " "),
			clusterChains["FORWARD"].Table,
			clusterChains["FORWARD"].WrapperChain,
		},
		{
			clusterChains["INPUT"].Name,
			strings.Join([]string{"-d", remotePodCIDR, "-j", clusterChains["INPUT"].Name}, " "),
			clusterChains["INPUT"].Table,
			clusterChains["INPUT"].WrapperChain,
		},
	}
	if localRemappedPodCIDR != defaultPodCIDRValue {
		ruleSpecs = append(ruleSpecs, rulespec{
			clusterChains["PREROUTING"].Name,
			strings.Join([]string{"-d", localRemappedPodCIDR, "-j", clusterChains["PREROUTING"].Name}, " "),
			clusterChains["PREROUTING"].Table,
			clusterChains["PREROUTING"].WrapperChain,
		})
		return ruleSpecs, nil
	}
	return ruleSpecs, nil
}

func (h IPTablesHandler) getChainRulespecsFromNp(nm *netv1alpha1.NatMapping) ([]rulespec, error) {
	clusterID := nm.Spec.ClusterID
	if clusterID == "" {
		return nil, &WrongParameter{
			Reason: StringNotEmpty,
		}
	}
	if err := IsValidCIDR(nm.Spec.PodCIDR); err != nil {
		return nil, &WrongParameter{
			Reason:    ValidCIDR,
			Parameter: nm.Spec.PodCIDR,
		}
	}
	if err := IsValidCIDR(nm.Spec.ExternalCIDR); err != nil {
		return nil, &WrongParameter{
			Reason:    ValidCIDR,
			Parameter: nm.Spec.ExternalCIDR,
		}
	}
	preRoutingChain := h.getChainsPerCluster(clusterID)["PREROUTING"]
	ruleSpecs := []rulespec{
		{
			preRoutingChain.Name,
			strings.Join([]string{"-s", nm.Spec.PodCIDR, "-d", nm.Spec.ExternalCIDR, "-j", preRoutingChain.Name}, " "),
			preRoutingChain.Table,
			preRoutingChain.WrapperChain,
		},
	}
	return ruleSpecs, nil
}

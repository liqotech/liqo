package iptables

import (
	"fmt"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/util/slice"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

// IPTableRule struct that holds all the information of an iptables rule.
type IPTableRule struct {
	Table    string
	Chain    string
	RuleSpec []string
}

// IPTHandler a handler that exposes all the functions needed to configure the iptables chains and rules.
type IPTHandler struct {
	ipt iptables.IPTables
}

// NewIPTHandler return the iptables handler used to configure the iptables rules.
func NewIPTHandler() (IPTHandler, error) {
	ipt, err := iptables.New()
	if err != nil {
		return IPTHandler{}, err
	}
	return IPTHandler{
		ipt: *ipt,
	}, err
}

// Init function is called at startup of the operator.
// here we:
// create LIQONET-FORWARD in the filter table and insert it in the "FORWARD" chain.
// create LIQONET-POSTROUTING in the nat table and insert it in the "POSTROUTING" chain.
// create LIQONET-INPUT in the filter table and insert it in the input chain.
// insert the rulespec which allows in input all the udp traffic incoming for the vxlan in the LIQONET-INPUT chain.
func (h IPTHandler) Init(defaultIfaceName string) error {
	if defaultIfaceName == "" {
		return &errors.WrongParameter{
			Argument:  "",
			Reason:    errors.StringNotEmpty,
			Parameter: "defaultIfaceName",
		}
	}
	if len(defaultIfaceName) > unix.IFNAMSIZ {
		return &errors.WrongParameter{
			Argument:  fmt.Sprintf("%d", unix.IFNAMSIZ),
			Reason:    errors.MinorOrEqual,
			Parameter: defaultIfaceName,
		}
	}

	// Get Liqo Chains
	liqoChains := getLiqoChains()

	// Create Liqo chains
	if err := h.createLiqoChains(liqoChains); err != nil {
		return fmt.Errorf("cannot create Liqo default chains: %w", err)
	}

	// Get Liqo rules
	liqoRules := getLiqoRules(defaultIfaceName)

	// Insert Liqo rules
	if err := h.ensureLiqoRules(liqoRules); err != nil {
		return err
	}
	return nil
}

// Function that creates default Liqo chains.
func (h IPTHandler) createLiqoChains(chains map[string]string) error {
	for _, chain := range chains {
		if err := h.createIptablesChainIfNotExists(getTableFromChain(chain), chain); err != nil {
			return err
		}
	}
	return nil
}

// Function that guarrantees Liqo rules are inserted.
func (h IPTHandler) ensureLiqoRules(rules map[string]string) error {
	for chain, rule := range rules {
		if err := h.insertLiqoRuleIfNotExists(chain, rule); err != nil {
			return err
		}
	}
	return nil
}

// Function that returns the set of Liqo rules. Value is a set of rules
// and key is the chain the set of rules should be inserted in.
func getLiqoRules(iface string) map[string]string {
	return map[string]string{
		consts.InputChain:       fmt.Sprintf("-j %s", consts.LiqonetInputChain),
		consts.ForwardChain:     fmt.Sprintf("-j %s", consts.LiqonetForwardingChain),
		consts.PreroutingChain:  fmt.Sprintf("-j %s", consts.LiqonetPreroutingChain),
		consts.PostroutingChain: fmt.Sprintf("-j %s", consts.LiqonetPostroutingChain),
	}
}

// Terminate func is the counterpart of Init. It removes Liqo
// configuration from iptables.
func (h IPTHandler) Terminate(defaultIfaceName string) error {
	if defaultIfaceName == "" {
		return &errors.WrongParameter{
			Argument:  "",
			Reason:    errors.StringNotEmpty,
			Parameter: "defaultIfaceName",
		}
	}
	if len(defaultIfaceName) > unix.IFNAMSIZ {
		return &errors.WrongParameter{
			Argument:  fmt.Sprintf("%d", unix.IFNAMSIZ),
			Reason:    errors.MinorOrEqual,
			Parameter: defaultIfaceName,
		}
	}

	// Remove Liqo rules
	if err := h.removeLiqoRules(defaultIfaceName); err != nil {
		return err
	}

	// Delete Liqo chains
	if err := h.deleteLiqoChains(); err != nil {
		return fmt.Errorf("cannot delete Liqo default chains: %w", err)
	}
	klog.Infof("IPTables Liqo configuration has been successfully removed.")
	return nil
}

// Function that deletes Liqo rules.
func (h IPTHandler) removeLiqoRules(defaultIfaceName string) error {
	// Get Liqo rules
	liqoRules := getLiqoRules(defaultIfaceName)
	for chain, rule := range liqoRules {
		if err := h.deleteRulesInChain(chain, []string{rule}); err != nil {
			return err
		}
	}
	return nil
}

// Function that deletes Liqo chains from a specific table.
func (h IPTHandler) deleteLiqoChainsFromTable(liqoChains map[string]string, table string) error {
	existingChains, err := h.ipt.ListChains(table)
	if err != nil {
		return nil
	}
	chainsToBeRemoved := make([]string, 0, 2)
	switch table {
	case consts.NatTable:
		// Add to the set of chains to be removed the Liqo chains
		chainsToBeRemoved = append(chainsToBeRemoved,
			liqoChains[consts.PreroutingChain],
			liqoChains[consts.PostroutingChain],
		)
		// Get cluster chains that may have not been removed in table
		chainsToBeRemoved = append(chainsToBeRemoved,
			getSliceContainingString(existingChains, consts.LiqonetPostroutingClusterChainPrefix)...,
		)
		chainsToBeRemoved = append(chainsToBeRemoved,
			getSliceContainingString(existingChains, consts.LiqonetPreroutingClusterChainPrefix)...,
		)
	case consts.FilterTable:
		// Add to the set of chains to be removed the Liqo chains
		chainsToBeRemoved = append(chainsToBeRemoved,
			liqoChains[consts.InputChain],
			liqoChains[consts.ForwardChain])
		// Get cluster chains that may have not been removed in table
		chainsToBeRemoved = append(chainsToBeRemoved,
			getSliceContainingString(existingChains, consts.LiqonetInputClusterChainPrefix)...,
		)
		chainsToBeRemoved = append(chainsToBeRemoved,
			getSliceContainingString(existingChains, consts.LiqonetForwardingClusterChainPrefix)...,
		)
	}
	// Delete chains in table
	if err := h.deleteChainsInTable(table, existingChains, chainsToBeRemoved); err != nil {
		return err
	}
	return nil
}

// Function that deletes Liqo chains.
func (h IPTHandler) deleteLiqoChains() error {
	// Get Liqo Chains
	liqoChains := getLiqoChains()

	// Delete chains in NAT table
	if err := h.deleteLiqoChainsFromTable(liqoChains, consts.NatTable); err != nil {
		return fmt.Errorf("unable to delete LIQO Chains in table %s: %w", consts.NatTable, err)
	}

	// Delete chains in Filter table
	if err := h.deleteLiqoChainsFromTable(liqoChains, consts.FilterTable); err != nil {
		return fmt.Errorf("unable to delete LIQO Chains in table %s: %w", consts.FilterTable, err)
	}
	return nil
}

func (h IPTHandler) deleteChainsInTable(table string, chains, chainsToBeRemoved []string) error {
	for _, chain := range chainsToBeRemoved {
		// Check existence of chain
		if slice.ContainsString(chains, chain, nil) {
			continue
		}
		// Chain does exist, then delete it.
		if err := h.ipt.ClearChain(table, chain); err != nil {
			return err
		}
		if err := h.ipt.DeleteChain(table, chain); err != nil {
			return err
		}
		klog.Infof("Deleted chain %s (table %s)", chain, getTableFromChain(chain))
	}
	return nil
}

// EnsureChainRulesPerCluster reads TunnelEndpoint resource and
// makes sure that chain rules for the given cluster exist.
func (h IPTHandler) EnsureChainRulesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	chainRules, err := getChainRulesPerCluster(tep)
	if err != nil {
		return err
	}
	for chain, newRules := range chainRules {
		// Invoking updateRulesPerChain would be an error
		// since it would replace all the rules in a Liqo chain
		// with rules related to this remote cluster.
		// Therefore, first we need to get existing rules
		// related to remote cluster and then
		// call updateSpecificRulesPerChain.
		rules, err := h.getExistingChainRules(tep.Spec.ClusterID, chain)
		if err != nil {
			return fmt.Errorf("cannot get existing chain rules per cluster %s: %w", tep.Spec.ClusterID, err)
		}
		if err := h.updateSpecificRulesPerChain(chain, rules, newRules); err != nil {
			return fmt.Errorf("cannot update rule for chain %s (table %s): %w", chain, getTableFromChain(chain), err)
		}
	}
	return nil
}

func (h IPTHandler) getExistingChainRules(clusterID, chain string) ([]string, error) {
	existingChainRules := make([]string, 0)
	// Get rules in chain
	rules, err := h.ListRulesInChain(chain)
	if err != nil {
		return nil, fmt.Errorf("unable to list rules in chain %s (table %s): %w",
			chain, getTableFromChain(chain), err)
	}

	// Get cluster chains related to received chain
	clusterChains := getChainsPerCluster(clusterID)

	// Check if an existing rule refers to a cluster chain
	for _, rule := range rules {
		for _, clusterChain := range clusterChains {
			if !strings.Contains(rule, clusterChain) {
				continue
			}
			// Rule refers to cluster chain
			existingChainRules = append(existingChainRules, rule)
			break
		}
	}
	return existingChainRules, nil
}

func getChainsPerCluster(clusterID string) map[string]string {
	chains := make(map[string]string)
	chains[consts.ForwardChain] = getClusterForwardChain(clusterID)
	chains[consts.InputChain] = getClusterInputChain(clusterID)
	chains[consts.PostroutingChain] = getClusterPostRoutingChain(clusterID)
	chains[consts.PreroutingChain] = getClusterPreRoutingChain(clusterID)
	return chains
}

// EnsureChainsPerCluster is used to be sure input, output, postrouting and prerouting chain for a given
// cluster are present in the NAT table and Filter table.
func (h IPTHandler) EnsureChainsPerCluster(clusterID string) error {
	if clusterID == "" {
		return &errors.WrongParameter{
			Parameter: consts.ClusterIDLabelName,
			Reason:    errors.StringNotEmpty,
		}
	}
	// Get chains per cluster
	chains := getChainsPerCluster(clusterID)
	for _, chain := range chains {
		if err := h.createIptablesChainIfNotExists(getTableFromChain(chain), chain); err != nil {
			return err
		}
	}
	return nil
}

// Function that receives a chain name and returns
// the table name the chain should belong to.
func getTableFromChain(chain string) string {
	// First manage the case the chain is a cluster chain
	if strings.Contains(chain, consts.LiqonetForwardingClusterChainPrefix) ||
		strings.Contains(chain, consts.LiqonetInputClusterChainPrefix) {
		return consts.FilterTable
	}
	if strings.Contains(chain, consts.LiqonetPostroutingClusterChainPrefix) ||
		strings.Contains(chain, consts.LiqonetPreroutingClusterChainPrefix) {
		return consts.NatTable
	}
	// Chain is a default iptables chain or a Liqo chain
	switch chain {
	case consts.ForwardChain, consts.InputChain, consts.LiqonetForwardingChain, consts.LiqonetInputChain:
		return consts.FilterTable
	case consts.PreroutingChain, consts.PostroutingChain, consts.LiqonetPreroutingChain, consts.LiqonetPostroutingChain:
		return consts.NatTable
	}
	return ""
}

// RemoveIPTablesConfigurationPerCluster clears and deletes input, forward, prerouting and postrouting chains
// for a remote cluster. In order to remove them, function first deletes related rules in LIQO-POSTROUTING,
// LIQO-PREROUTING, LIQO-FORWARD and LIQO-INPUT.
func (h IPTHandler) RemoveIPTablesConfigurationPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	// Delete chain rules
	err := h.deleteChainRulesPerCluster(tep)
	if err != nil {
		return fmt.Errorf("cannot remove chain rules per cluster %s: %w", tep.Spec.ClusterID, err)
	}
	// Delete chains
	err = h.removeChainsPerCluster(tep.Spec.ClusterID)
	if err != nil {
		return fmt.Errorf("cannot remove chains per cluster: %w", err)
	}
	klog.Infof("IPTables config per cluster %s has been deleted", tep.Spec.ClusterID)
	return nil
}

// Function removes rules related to a remote cluster from chains LIQO-POSTROUTING, LIQO-PREROUTING,
// LIQO-FORWARD, LIQO-INPUT.
func (h IPTHandler) deleteChainRulesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	clusterChainRules, err := getChainRulesPerCluster(tep)
	if err != nil {
		return fmt.Errorf("cannot retrieve chain rules per cluster %s: %w", tep.Spec.ClusterID, err)
	}
	for chain, rules := range clusterChainRules {
		err = h.deleteRulesInChain(chain, rules)
		if err != nil {
			return fmt.Errorf("cannot delete cluster %s rules in chain %s: %w", tep.Spec.ClusterID, chain, err)
		}
	}
	return nil
}

func (h IPTHandler) deleteRulesInChain(chain string, rules []string) error {
	table := getTableFromChain(chain)
	existingRules, err := h.ListRulesInChain(chain)
	if err != nil {
		return fmt.Errorf("unable to list rules in chain %s (table %s): %w", chain, table, err)
	}
	for _, rule := range rules {
		if !slice.ContainsString(existingRules, rule, nil) {
			continue
		}
		// Rule exists, then delete it
		if err := h.ipt.Delete(table, chain, strings.Split(rule, " ")...); err != nil {
			return err
		}
		klog.Infof("Deleted rule %s in chain %s (table %s)", rule, chain, table)
	}
	return nil
}

// Function removes all the chains (and contained rules) related to a remote cluster.
func (h IPTHandler) removeChainsPerCluster(clusterID string) error {
	// Get existing NAT chains
	existingChainsNAT, err := h.ipt.ListChains(consts.NatTable)
	if err != nil {
		return err
	}
	// Get existing Filter chains
	existingChainsFilter, err := h.ipt.ListChains(consts.FilterTable)
	if err != nil {
		return err
	}
	// Get chains per cluster
	chains := getChainsPerCluster(clusterID)
	// For each cluster chain, check if it exists.
	// If it does exist, then remove it. Otherwise do nothing.
	for wrapperChain, chain := range chains {
		if getTableFromChain(wrapperChain) == consts.NatTable && slice.ContainsString(existingChainsNAT, chain, nil) {
			// Check if chain exists
			if !slice.ContainsString(existingChainsNAT, chain, nil) {
				continue
			}
			if err := h.ipt.ClearChain(consts.NatTable, chain); err != nil {
				return err
			}
			if err := h.ipt.DeleteChain(consts.NatTable, chain); err != nil {
				return err
			}
			klog.Infof("Deleted chain %s in table %s", chain, getTableFromChain(chain))
			continue
		}
		if getTableFromChain(wrapperChain) == consts.FilterTable && slice.ContainsString(existingChainsFilter, chain, nil) {
			if !slice.ContainsString(existingChainsFilter, chain, nil) {
				continue
			}
			if err := h.ipt.ClearChain(consts.FilterTable, chain); err != nil {
				return err
			}
			if err := h.ipt.DeleteChain(consts.FilterTable, chain); err != nil {
				return err
			}
			klog.Infof("Deleted chain %s in table %s", chain, getTableFromChain(chain))
		}
	}
	return nil
}

// Function that returns a slice of strings that contains a given string.
func getSliceContainingString(stringSlice []string, s string) (stringsThatContainsS []string) {
	stringsThatContainsS = make([]string, 0)
	for _, str := range stringSlice {
		if strings.Contains(str, s) {
			stringsThatContainsS = append(stringsThatContainsS, str)
		}
	}
	return
}

// ListRulesInChain is used to adjust the result returned by List of go-iptables.
func (h IPTHandler) ListRulesInChain(chain string) ([]string, error) {
	existingRules, err := h.ipt.List(getTableFromChain(chain), chain)
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

func (h IPTHandler) createIptablesChainIfNotExists(table, newChain string) error {
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

func (h IPTHandler) insertLiqoRuleIfNotExists(chain, rule string) error {
	table := getTableFromChain(chain)
	// Get the list of rules for the specified chain
	existingRules, err := h.ipt.List(table, chain)
	if err != nil {
		return fmt.Errorf("unable to get the rules in %s chain in %s table : %w", chain, table, err)
	}
	// Check if the rule exists and at the same time if it exists more then once
	numOccurrences := 0
	for _, existingRule := range existingRules {
		if strings.Contains(existingRule, rule) {
			numOccurrences++
		}
	}
	// If the occurrences if greater then one, remove the rule
	if numOccurrences > 1 {
		for i := 0; i < numOccurrences; i++ {
			if err = h.ipt.Delete(table, chain, strings.Split(rule, " ")...); err != nil {
				return fmt.Errorf("unable to delete iptable rule \"%s\": %w", rule, err)
			}
		}
		if err = h.ipt.Insert(table, chain, 1, strings.Split(rule, " ")...); err != nil {
			return fmt.Errorf("unable to insert iptable rule \"%s\": %w", rule, err)
		}
	}
	if numOccurrences == 1 {
		// If the occurrence is one then check the position and if not at the first one we delete and reinsert it
		if strings.Contains(existingRules[0], rule) {
			return nil
		}
		if err = h.ipt.Delete(table, chain, strings.Split(rule, " ")...); err != nil {
			return fmt.Errorf("unable to delete iptable rule \"%s\": %w", rule, err)
		}
		if err = h.ipt.Insert(table, chain, 1, strings.Split(rule, " ")...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %w", rule, err)
		}
		return nil
	}
	if numOccurrences == 0 {
		// If the occurrence is zero then insert the rule in first position
		if err = h.ipt.Insert(table, chain, 1, strings.Split(rule, " ")...); err != nil {
			return fmt.Errorf("unable to insert iptable rule \"%s\": %w", rule, err)
		}
		klog.Infof("Inserted rule '%s' in chain %s of table %s", rule, chain, table)
	}
	return nil
}

// Function to update specific rules in a given chain.
func (h IPTHandler) updateSpecificRulesPerChain(chain string, existingRules, newRules []string) error {
	table := getTableFromChain(chain)
	for _, existingRule := range existingRules {
		// Remove existing rules that are not in the set of new rules,
		// they are outdated.
		outdated := true
		for _, newRule := range newRules {
			if existingRule == newRule {
				outdated = false
			}
		}
		if outdated {
			if err := h.ipt.Delete(table, chain, strings.Split(existingRule, " ")...); err != nil {
				return fmt.Errorf("unable to delete outdated rule %s from chain %s (table %s): %w",
					existingRule, chain, table, err)
			}
			klog.Infof("Deleted outdated rule %s from chain %s (table %s)", existingRule, chain, table)
		}
	}
	err := h.insertRulesIfNotPresent(table, chain, newRules)
	if err != nil {
		return fmt.Errorf("cannot add new rules in chain %s (table %s): %w", chain, table, err)
	}
	return nil
}

func (h IPTHandler) insertRulesIfNotPresent(table, chain string, rules []string) error {
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

// Function that returns the set of rules used in Liqo chains (e.g. LIQO-PREROUTING)
// related to a remote cluster. Return value is a map of slices in which value
// is the a set of rules and key is the chain the set of rules should belong to.
func getChainRulesPerCluster(tep *netv1alpha1.TunnelEndpoint) (map[string][]string, error) {
	clusterID := tep.Spec.ClusterID
	localRemappedPodCIDR, remotePodCIDR := utils.GetPodCIDRS(tep)
	if err := utils.IsValidCIDR(localRemappedPodCIDR); localRemappedPodCIDR != consts.DefaultCIDRValue &&
		err != nil {
		return nil, &errors.WrongParameter{
			Reason:    errors.ValidCIDR,
			Parameter: localRemappedPodCIDR,
		}
	}
	if err := utils.IsValidCIDR(remotePodCIDR); err != nil {
		return nil, &errors.WrongParameter{
			Reason:    errors.ValidCIDR,
			Parameter: remotePodCIDR,
		}
	}
	if clusterID == "" {
		return nil, &errors.WrongParameter{
			Reason: errors.StringNotEmpty,
		}
	}

	// Init chain rules
	chainRules := make(map[string][]string)
	chainRules[consts.LiqonetPostroutingChain] = make([]string, 0)
	chainRules[consts.LiqonetPreroutingChain] = make([]string, 0)
	chainRules[consts.LiqonetForwardingChain] = make([]string, 0)
	chainRules[consts.LiqonetInputChain] = make([]string, 0)

	// For these rules, source in not necessary since
	// the remotePodCIDR is unique in home cluster
	chainRules[consts.LiqonetPostroutingChain] = append(chainRules[consts.LiqonetPostroutingChain],
		fmt.Sprintf("-d %s -j %s", remotePodCIDR, getClusterPostRoutingChain(clusterID)))
	chainRules[consts.LiqonetInputChain] = append(chainRules[consts.LiqonetInputChain],
		fmt.Sprintf("-d %s -j %s", remotePodCIDR, getClusterInputChain(clusterID)))
	chainRules[consts.LiqonetForwardingChain] = append(chainRules[consts.LiqonetForwardingChain],
		fmt.Sprintf("-d %s -j %s", remotePodCIDR, getClusterForwardChain(clusterID)))
	if localRemappedPodCIDR != consts.DefaultCIDRValue {
		// For the following rule, source is necessary
		// because more remote clusters could have
		// remapped home PodCIDR in the same way, then only use dst is not enough.
		chainRules[consts.LiqonetPreroutingChain] = append(chainRules[consts.LiqonetPreroutingChain],
			fmt.Sprintf("-s %s -d %s -j %s", remotePodCIDR, localRemappedPodCIDR, getClusterPreRoutingChain(clusterID)))
	}
	return chainRules, nil
}

func getClusterPreRoutingChain(clusterID string) string {
	return fmt.Sprintf("%s%s", consts.LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterPostRoutingChain(clusterID string) string {
	return fmt.Sprintf("%s%s", consts.LiqonetPostroutingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterForwardChain(clusterID string) string {
	return fmt.Sprintf("%s%s", consts.LiqonetForwardingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterInputChain(clusterID string) string {
	return fmt.Sprintf("%s%s", consts.LiqonetInputClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

// Function that returns the set of Liqo default chains.
// Value is the Liqo chain, key is the related default chain.
// Example: key: PREROUTING, value: LIQO-PREROUTING.
func getLiqoChains() map[string]string {
	return map[string]string{
		consts.PreroutingChain:  consts.LiqonetPreroutingChain,
		consts.PostroutingChain: consts.LiqonetPostroutingChain,
		consts.ForwardChain:     consts.LiqonetForwardingChain,
		consts.InputChain:       consts.LiqonetInputChain,
	}
}

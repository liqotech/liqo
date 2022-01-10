// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iptables

import (
	"fmt"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

const (
	// liqonetPostroutingChain is the name of the postrouting chain inserted by liqo.
	liqonetPostroutingChain = "LIQO-POSTROUTING"
	// liqonetPreroutingChain is the naame of the prerouting chain inserted by liqo.
	liqonetPreroutingChain = "LIQO-PREROUTING"
	// liqonetForwardingChain is the name of the forwarding chain inserted by liqo.
	liqonetForwardingChain = "LIQO-FORWARD"
	// liqonetInputChain is the name of the input chain inserted by liqo.
	liqonetInputChain = "LIQO-INPUT"
	// liqonetPostroutingClusterChainPrefix the prefix used to name the postrouting chains for a specific cluster.
	liqonetPostroutingClusterChainPrefix = "LIQO-PSTRT-CLS-"
	// liqonetPreroutingClusterChainPrefix prefix used to name the prerouting chains for a specific cluster.
	liqonetPreroutingClusterChainPrefix = "LIQO-PRRT-CLS-"
	// liqonetForwardingClusterChainPrefix prefix used to name the forwarding chains for a specific cluster.
	liqonetForwardingClusterChainPrefix = "LIQO-FRWD-CLS-"
	// liqonetInputClusterChainPrefix prefix used to name the input chains for a specific cluster.
	liqonetInputClusterChainPrefix = "LIQO-INPT-CLS-"
	// liqonetPreRoutingMappingClusterChainPrefix prefix used to name the prerouting mapping chain for a specific cluster.
	liqonetPreRoutingMappingClusterChainPrefix = "LIQO-PRRT-MAP-CLS-"
	// natTable constant used for the "nat" table.
	natTable = "nat"
	// filterTable constant used for the "filter" table.
	filterTable = "filter"
	// preroutingChain constant.
	preroutingChain = "PREROUTING"
	// postroutingChain constant.
	postroutingChain = "POSTROUTING"
	// inputChain constant.
	inputChain = "INPUT"
	// forwardChain constant.
	forwardChain = "FORWARD"
	// MASQUERADE action constant.
	MASQUERADE = "MASQUERADE"
	// SNAT action constant.
	SNAT = "SNAT"
	// DNAT action constant.
	DNAT = "DNAT"
	// NETMAP action constant.
	NETMAP = "NETMAP"
	// ACCEPT action constant.
	ACCEPT = "ACCEPT"
)

// IPTableRule is a slice of string. This is the format used by module go-iptables.
type IPTableRule []string

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
// create LIQONET-INPUT in the filter table and insert it in the input chain.
// create LIQONET-POSTROUTING in the nat table and insert it in the "POSTROUTING" chain.
// create LIQONET-PREROUTING in the nat table and insert it in the "PREROUTING" chain.
func (h IPTHandler) Init() error {
	// Get Liqo Chains
	liqoChains := getLiqoChains()

	// Create Liqo chains
	if err := h.createLiqoChains(liqoChains); err != nil {
		return fmt.Errorf("cannot create Liqo default chains: %w", err)
	}

	// Get Liqo rules
	liqoRules := getLiqoRules()

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
func (h IPTHandler) ensureLiqoRules(rules map[string]IPTableRule) error {
	for chain, rule := range rules {
		if err := h.insertLiqoRuleIfNotExists(chain, rule); err != nil {
			return err
		}
	}
	return nil
}

// Function that returns the set of Liqo rules. Value is a rule
// and key is the chain the rule should be inserted in.
func getLiqoRules() map[string]IPTableRule {
	return map[string]IPTableRule{
		inputChain:       {"-j", liqonetInputChain},
		forwardChain:     {"-j", liqonetForwardingChain},
		preroutingChain:  {"-j", liqonetPreroutingChain},
		postroutingChain: {"-j", liqonetPostroutingChain},
	}
}

// Terminate func is the counterpart of Init. It removes Liqo
// configuration from iptables.
func (h IPTHandler) Terminate() error {
	// Remove Liqo rules
	if err := h.removeLiqoRules(); err != nil {
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
func (h IPTHandler) removeLiqoRules() error {
	// Get Liqo rules
	liqoRules := getLiqoRules()
	for chain, rule := range liqoRules {
		if err := h.deleteRulesInChain(chain, []IPTableRule{rule}); err != nil {
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
	case natTable:
		// Add to the set of chains to be removed the Liqo chains
		chainsToBeRemoved = append(chainsToBeRemoved,
			liqoChains[preroutingChain],
			liqoChains[postroutingChain],
		)
		// Get cluster chains that may have not been removed in table
		chainsToBeRemoved = append(chainsToBeRemoved,
			getSliceContainingString(existingChains, liqonetPostroutingClusterChainPrefix)...,
		)
		chainsToBeRemoved = append(chainsToBeRemoved,
			getSliceContainingString(existingChains, liqonetPreroutingClusterChainPrefix)...,
		)
	case filterTable:
		// Add to the set of chains to be removed the Liqo chains
		chainsToBeRemoved = append(chainsToBeRemoved,
			liqoChains[inputChain],
			liqoChains[forwardChain])
		// Get cluster chains that may have not been removed in table
		chainsToBeRemoved = append(chainsToBeRemoved,
			getSliceContainingString(existingChains, liqonetInputClusterChainPrefix)...,
		)
		chainsToBeRemoved = append(chainsToBeRemoved,
			getSliceContainingString(existingChains, liqonetForwardingClusterChainPrefix)...,
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
	if err := h.deleteLiqoChainsFromTable(liqoChains, natTable); err != nil {
		return fmt.Errorf("unable to delete LIQO Chains in table %s: %w", natTable, err)
	}

	// Delete chains in Filter table
	if err := h.deleteLiqoChainsFromTable(liqoChains, filterTable); err != nil {
		return fmt.Errorf("unable to delete LIQO Chains in table %s: %w", filterTable, err)
	}
	return nil
}

func (h IPTHandler) deleteChainsInTable(table string, chains, chainsToBeRemoved []string) error {
	for _, chain := range chainsToBeRemoved {
		// Check existence of chain
		if slice.ContainsString(chains, chain) {
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

func getChainsPerCluster(clusterID string) []string {
	chains := []string{
		getClusterForwardChain(clusterID),
		getClusterInputChain(clusterID),
		getClusterPostRoutingChain(clusterID),
		getClusterPreRoutingChain(clusterID),
		getClusterPreRoutingMappingChain(clusterID),
	}
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
	if strings.Contains(chain, liqonetForwardingClusterChainPrefix) ||
		strings.Contains(chain, liqonetInputClusterChainPrefix) {
		return filterTable
	}
	if strings.Contains(chain, liqonetPostroutingClusterChainPrefix) ||
		strings.Contains(chain, liqonetPreRoutingMappingClusterChainPrefix) ||
		strings.Contains(chain, liqonetPreroutingClusterChainPrefix) {
		return natTable
	}
	// Chain is a default iptables chain or a Liqo chain
	switch chain {
	case forwardChain, inputChain, liqonetForwardingChain, liqonetInputChain:
		return filterTable
	case preroutingChain, postroutingChain, liqonetPreroutingChain, liqonetPostroutingChain:
		return natTable
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

func (h IPTHandler) deleteRulesInChain(chain string, rules []IPTableRule) error {
	table := getTableFromChain(chain)
	existingRules, err := h.ListRulesInChain(chain)
	if err != nil {
		return fmt.Errorf("unable to list rules in chain %s (table %s): %w", chain, table, err)
	}
	for _, rule := range rules {
		if !slice.ContainsString(existingRules, strings.Join(rule, " ")) {
			continue
		}
		// Rule exists, then delete it
		if err := h.ipt.Delete(table, chain, rule...); err != nil {
			return err
		}
		klog.Infof("Deleted rule %s in chain %s (table %s)", rule, chain, table)
	}
	return nil
}

// Function removes all the chains (and contained rules) related to a remote cluster.
func (h IPTHandler) removeChainsPerCluster(clusterID string) error {
	// Get existing NAT chains
	existingChainsNAT, err := h.ipt.ListChains(natTable)
	if err != nil {
		return err
	}
	// Get existing Filter chains
	existingChainsFilter, err := h.ipt.ListChains(filterTable)
	if err != nil {
		return err
	}
	// Get chains per cluster
	chains := getChainsPerCluster(clusterID)
	// For each cluster chain, check if it exists.
	// If it does exist, then remove it. Otherwise do nothing.
	for _, chain := range chains {
		if getTableFromChain(chain) == natTable && slice.ContainsString(existingChainsNAT, chain) {
			// Check if chain exists
			if !slice.ContainsString(existingChainsNAT, chain) {
				continue
			}
			if err := h.ipt.ClearChain(natTable, chain); err != nil {
				return err
			}
			if err := h.ipt.DeleteChain(natTable, chain); err != nil {
				return err
			}
			klog.Infof("Deleted chain %s in table %s", chain, getTableFromChain(chain))
			continue
		}
		if getTableFromChain(chain) == filterTable && slice.ContainsString(existingChainsFilter, chain) {
			if !slice.ContainsString(existingChainsFilter, chain) {
				continue
			}
			if err := h.ipt.ClearChain(filterTable, chain); err != nil {
				return err
			}
			if err := h.ipt.DeleteChain(filterTable, chain); err != nil {
				return err
			}
			klog.Infof("Deleted chain %s in table %s", chain, getTableFromChain(chain))
		}
	}
	return nil
}

// EnsurePostroutingRules makes sure that the postrouting rules for a given cluster are in place and updated.
func (h IPTHandler) EnsurePostroutingRules(tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	rules, err := getPostroutingRules(tep)
	if err != nil {
		return err
	}
	return h.updateRulesPerChain(getClusterPostRoutingChain(clusterID), rules)
}

// EnsurePreroutingRulesPerTunnelEndpoint makes sure that the prerouting rules extracted from a
// TunnelEndpoint resource are place and updated.
func (h IPTHandler) EnsurePreroutingRulesPerTunnelEndpoint(tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	rules, err := getPreRoutingRulesPerTunnelEndpoint(tep)
	if err != nil {
		return err
	}
	return h.updateRulesPerChain(getClusterPreRoutingChain(clusterID), rules)
}

// EnsurePreroutingRulesPerNatMapping makes sure that the prerouting rules extracted from a
// NatMapping resource are place and updated.
func (h IPTHandler) EnsurePreroutingRulesPerNatMapping(nm *netv1alpha1.NatMapping) error {
	clusterID := nm.Spec.ClusterID
	rules, err := getPreRoutingRulesPerNatMapping(nm)
	if err != nil {
		return err
	}
	return h.updateRulesPerChain(getClusterPreRoutingMappingChain(clusterID), rules)
}

func getPreRoutingRulesPerTunnelEndpoint(tep *netv1alpha1.TunnelEndpoint) ([]IPTableRule, error) {
	// Check tep fields
	if err := utils.CheckTep(tep); err != nil {
		return nil, fmt.Errorf("invalid TunnelEndpoint resource: %w", err)
	}
	localPodCIDR := tep.Spec.LocalPodCIDR
	localRemappedPodCIDR, remotePodCIDR := utils.GetPodCIDRS(tep)

	rules := make([]IPTableRule, 0)
	if localRemappedPodCIDR == consts.DefaultCIDRValue {
		// Remote cluster has not remapped home PodCIDR,
		// this means there is no need to NAT
		return rules, nil
	}
	// Remote cluster has remapped home PodCIDR
	rules = append(rules,
		IPTableRule{"-s", remotePodCIDR, "-d", localRemappedPodCIDR, "-j", NETMAP, "--to", localPodCIDR},
	)
	return rules, nil
}

func getPreRoutingRulesPerNatMapping(nm *netv1alpha1.NatMapping) ([]IPTableRule, error) {
	// Check tep fields
	if nm.Spec.ClusterID == "" {
		return nil, &errors.WrongParameter{
			Parameter: consts.ClusterIDLabelName,
			Reason:    errors.StringNotEmpty,
		}
	}

	rules := make([]IPTableRule, 0, len(nm.Spec.ClusterMappings))

	for oldIP, newIP := range nm.Spec.ClusterMappings {
		rules = append(rules,
			IPTableRule{"-d", newIP, "-j", DNAT, "--to-destination", oldIP},
		)
	}
	return rules, nil
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
			rule = strings.ReplaceAll(rule, "/32", "")
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

func (h IPTHandler) insertLiqoRuleIfNotExists(chain string, rule IPTableRule) error {
	table := getTableFromChain(chain)
	// Get the list of rules for the specified chain
	existingRules, err := h.ipt.List(table, chain)
	if err != nil {
		return fmt.Errorf("unable to get the rules in %s chain in %s table : %w", chain, table, err)
	}
	// Check if the rule exists and at the same time if it exists more then once
	numOccurrences := 0
	for _, existingRule := range existingRules {
		if strings.Contains(existingRule, strings.Join(rule, " ")) {
			numOccurrences++
		}
	}
	// If the occurrences if greater then one, remove the rule
	if numOccurrences > 1 {
		for i := 0; i < numOccurrences; i++ {
			if err = h.ipt.Delete(table, chain, rule...); err != nil {
				return fmt.Errorf("unable to delete iptable rule \"%s\": %w", rule, err)
			}
		}
		if err = h.ipt.Insert(table, chain, 1, rule...); err != nil {
			return fmt.Errorf("unable to insert iptable rule \"%s\": %w", rule, err)
		}
	}
	if numOccurrences == 1 {
		// If the occurrence is one then check the position and if not at the first one we delete and reinsert it
		if strings.Contains(existingRules[0], strings.Join(rule, " ")) {
			return nil
		}
		if err = h.ipt.Delete(table, chain, rule...); err != nil {
			return fmt.Errorf("unable to delete iptable rule \"%s\": %w", rule, err)
		}
		if err = h.ipt.Insert(table, chain, 1, rule...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %w", rule, err)
		}
		return nil
	}
	if numOccurrences == 0 {
		// If the occurrence is zero then insert the rule in first position
		if err = h.ipt.Insert(table, chain, 1, rule...); err != nil {
			return fmt.Errorf("unable to insert iptable rule \"%s\": %w", rule, err)
		}
		klog.Infof("Inserted rule '%s' in chain %s of table %s", rule, chain, table)
	}
	return nil
}

// Function to update specific rules in a given chain.
func (h IPTHandler) updateSpecificRulesPerChain(chain string, existingRules []string, newRules []IPTableRule) error {
	table := getTableFromChain(chain)
	for _, existingRule := range existingRules {
		// Remove existing rules that are not in the set of new rules,
		// they are outdated.
		outdated := true
		for _, newRule := range newRules {
			if strings.Contains(existingRule, strings.Join(newRule, " ")) {
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

// Function to updates rules in a given chain.
func (h IPTHandler) updateRulesPerChain(chain string, newRules []IPTableRule) error {
	existingRules, err := h.ListRulesInChain(chain)
	if err != nil {
		return fmt.Errorf("cannot list rules in chain %s (table %s): %w", chain, getTableFromChain(chain), err)
	}
	return h.updateSpecificRulesPerChain(chain, existingRules, newRules)
}

func (h IPTHandler) insertRulesIfNotPresent(table, chain string, rules []IPTableRule) error {
	for _, rule := range rules {
		exists, err := h.ipt.Exists(table, chain, rule...)
		if err != nil {
			klog.Errorf("unable to check if rule '%s' exists in chain %s in table %s: %w", rule, chain, table, err)
			return err
		}
		if !exists {
			if err := h.ipt.AppendUnique(table, chain, rule...); err != nil {
				return err
			}
			klog.Infof("Inserting rule '%s' in chain %s (table %s)", rule, chain, table)
		}
	}
	return nil
}

func getPostroutingRules(tep *netv1alpha1.TunnelEndpoint) ([]IPTableRule, error) {
	if err := utils.CheckTep(tep); err != nil {
		return nil, fmt.Errorf("invalid TunnelEndpoint resource: %w", err)
	}
	clusterID := tep.Spec.ClusterID
	localPodCIDR := tep.Spec.LocalPodCIDR
	localRemappedPodCIDR, remotePodCIDR := utils.GetPodCIDRS(tep)
	_, remoteExternalCIDR := utils.GetExternalCIDRS(tep)
	if localRemappedPodCIDR != consts.DefaultCIDRValue {
		// Get the first IP address from the podCIDR of the local cluster
		// in this case it is the podCIDR to which the local podCIDR has bee remapped by the remote peering cluster
		natIP, err := utils.GetFirstIP(localRemappedPodCIDR)
		if err != nil {
			klog.Errorf("Unable to get the IP from localPodCidr %s for remote cluster %s used to NAT the traffic from localhosts to remote hosts",
				localRemappedPodCIDR, clusterID)
			return nil, err
		}
		return []IPTableRule{
			{"-s", localPodCIDR, "-d", remotePodCIDR, "-j", NETMAP, "--to", localRemappedPodCIDR},
			{"-s", localPodCIDR, "-d", remoteExternalCIDR, "-j", NETMAP, "--to", localRemappedPodCIDR},
			{"!", "-s", localPodCIDR, "-d", remotePodCIDR, "-j", SNAT, "--to-source", natIP},
			{"!", "-s", localPodCIDR, "-d", remoteExternalCIDR, "-j", SNAT, "--to-source", natIP},
		}, nil
	}
	// Get the first IP address from the podCIDR of the local cluster
	natIP, err := utils.GetFirstIP(localPodCIDR)
	if err != nil {
		klog.Errorf("Unable to get the IP from localPodCidr %s for cluster %s used to NAT the traffic from localhosts to remote hosts",
			tep.Spec.RemotePodCIDR, clusterID)
		return nil, err
	}
	return []IPTableRule{
		{"!", "-s", localPodCIDR, "-d", remotePodCIDR, "-j", SNAT, "--to-source", natIP},
		{"!", "-s", localPodCIDR, "-d", remoteExternalCIDR, "-j", SNAT, "--to-source", natIP},
	}, nil
}

// Function that returns the set of rules used in Liqo chains (e.g. LIQO-PREROUTING)
// related to a remote cluster. Return value is a map of slices in which value
// is the a set of rules and key is the chain the set of rules should belong to.
func getChainRulesPerCluster(tep *netv1alpha1.TunnelEndpoint) (map[string][]IPTableRule, error) {
	if err := utils.CheckTep(tep); err != nil {
		return nil, fmt.Errorf("invalid TunnelEndpoint resource: %w", err)
	}
	clusterID := tep.Spec.ClusterID
	localRemappedPodCIDR, remotePodCIDR := utils.GetPodCIDRS(tep)
	localRemappedExternalCIDR, remoteExternalCIDR := utils.GetExternalCIDRS(tep)

	// Init chain rules
	chainRules := make(map[string][]IPTableRule)
	chainRules[liqonetPostroutingChain] = make([]IPTableRule, 0)
	chainRules[liqonetPreroutingChain] = make([]IPTableRule, 0)
	chainRules[liqonetForwardingChain] = make([]IPTableRule, 0)
	chainRules[liqonetInputChain] = make([]IPTableRule, 0)

	// For these rules, source in not necessary since
	// the remotePodCIDR is unique in home cluster
	chainRules[liqonetPostroutingChain] = append(chainRules[liqonetPostroutingChain],
		IPTableRule{"-d", remotePodCIDR, "-j", getClusterPostRoutingChain(clusterID)},
		IPTableRule{"-d", remoteExternalCIDR, "-j", getClusterPostRoutingChain(clusterID)})
	chainRules[liqonetInputChain] = append(chainRules[liqonetInputChain],
		IPTableRule{"-d", remotePodCIDR, "-j", getClusterInputChain(clusterID)})
	chainRules[liqonetForwardingChain] = append(chainRules[liqonetForwardingChain],
		IPTableRule{"-d", remotePodCIDR, "-j", getClusterForwardChain(clusterID)})
	chainRules[liqonetPreroutingChain] = append(chainRules[liqonetPreroutingChain],
		IPTableRule{"-s", remotePodCIDR, "-d", localRemappedExternalCIDR, "-j", getClusterPreRoutingMappingChain(clusterID)})
	if localRemappedPodCIDR != consts.DefaultCIDRValue {
		// For the following rule, source is necessary
		// because more remote clusters could have
		// remapped home PodCIDR in the same way, then only use dst is not enough.
		chainRules[liqonetPreroutingChain] = append(chainRules[liqonetPreroutingChain],
			IPTableRule{"-s", remotePodCIDR, "-d", localRemappedPodCIDR, "-j", getClusterPreRoutingChain(clusterID)})
	}
	return chainRules, nil
}

func getClusterPreRoutingChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterPostRoutingChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetPostroutingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterForwardChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetForwardingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterInputChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetInputClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterPreRoutingMappingChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetPreRoutingMappingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

// Function that returns the set of Liqo default chains.
// Value is the Liqo chain, key is the related default chain.
// Example: key: PREROUTING, value: LIQO-PREROUTING.
func getLiqoChains() map[string]string {
	return map[string]string{
		preroutingChain:  liqonetPreroutingChain,
		postroutingChain: liqonetPostroutingChain,
		forwardChain:     liqonetForwardingChain,
		inputChain:       liqonetInputChain,
	}
}

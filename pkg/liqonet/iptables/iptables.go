// Copyright 2019-2023 The Liqo Authors
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
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/coreos/go-iptables/iptables"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	k8sstrings "k8s.io/utils/strings"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/errors"
	liqoipset "github.com/liqotech/liqo/pkg/liqonet/ipset"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

const (
	// liqonetPostroutingChain is the name of the postrouting chain inserted by liqo.
	liqonetPostroutingChain = "LIQO-POSTROUTING"
	// liqonetPreroutingChain is the naame of the prerouting chain inserted by liqo.
	liqonetPreroutingChain = "LIQO-PREROUTING"
	// liqonetForwardingChain is the name of the forwarding chain inserted by liqo.
	liqonetForwardingChain = "LIQO-FORWARD"
	// liqonetPostroutingClusterChainPrefix the prefix used to name the postrouting chains for a specific cluster.
	liqonetPostroutingClusterChainPrefix = "LIQO-PSTRT-CLS-"
	// liqonetPreroutingClusterChainPrefix prefix used to name the prerouting chains for a specific cluster.
	liqonetPreroutingClusterChainPrefix = "LIQO-PRRT-CLS-"
	// liqonetForwardingClusterChainPrefix prefix used to name the forwarding chains for traffic
	// allowed by IntraClusterTrafficSegregation security mode for a specific cluster.
	liqonetForwardingClusterChainPrefix = "LIQO-FRWD-CLS-"
	// liqonetInputClusterChainPrefix prefix used to name the input chains for a specific cluster.
	liqonetForwardingExtClusterChainPrefix = "LIQO-FRWD-EXT-CLS-"
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
	// DROP action constant.
	DROP = "DROP"
	// iptables module for accessing the connection tracking information for a packet.
	conntrackModule = "conntrack"
	// iptables module for matching IP sets defined by ipsets.
	setModule = "set"
	// ESTABLISHED state: the packet is associated with a connection which has seen packets in both directions.
	ESTABLISHED = "ESTABLISHED"
	// RELATED state: the packet is associated with a connection which is related to another already ESTABLISHED connection.
	RELATED = "RELATED"
	// liqonetOffloadedPodsIPSetPrefix is the prefix used to name the IPSet containing adresses of pods offloaded from a specific cluster.
	liqonetOffloadedPodsIPSetPrefix = "SET-CLS"
	// IPSetNameMaxLength is the maximum number of characters accepted for the name of an IPSet.
	IPSetNameMaxLength = 31
	// keepExistingRules: constant for keeping existing rules during the update of an iptables chain.
	keepExistingRules = true
	// notKeepExistingRules: constant for not keeping existing rules during the update of an iptables chain.
	notKeepExistingRules = false
)

// PodInfo contains informations useful to create rules allowing
// traffic towards offloaded pods.
type PodInfo struct {
	PodIP           string
	RemoteClusterID string
	Deleting        bool
}

// EndpointInfo contains informations useful to create rules allowing
// traffic towards service endpoints.
type EndpointInfo struct {
	Address       string
	SrcClusterIDs []string
	Deleting      bool
}

// RuleInsertionStrategyType represents different insertion strategies for inserting an iptables rule in a table.
type RuleInsertionStrategyType string

const (
	// Prepend indicates to insert the rule as first.
	Prepend RuleInsertionStrategyType = "Prepended"
	// Append indicates to insert the rule as last.
	Append RuleInsertionStrategyType = "Appended"
)

// IPTableRule is a slice of string. This is the format used by module go-iptables.
type IPTableRule []string

// String returns the string representation of the rule.
func (itr IPTableRule) String() string {
	return strings.Join(itr, " ")
}

// ParseRule parses a string rule in the format used by go-iptables.
func ParseRule(rule string) (IPTableRule, error) {
	// replace \' contained in the rule returned by go-iptables "List" method to match generated comments.
	rule = strings.ReplaceAll(rule, `\'`, "'")
	r := csv.NewReader(strings.NewReader(rule))
	r.Comma = ' '
	fields, err := r.Read()
	return IPTableRule(fields), err
}

// IPTHandler a handler that exposes all the functions needed to configure the iptables chains and rules.
type IPTHandler struct {
	Ipt iptables.IPTables
}

// NewIPTHandler return the iptables handler used to configure the iptables rules.
func NewIPTHandler() (IPTHandler, error) {
	selectedmode := os.Getenv("IPTABLES_MODE")
	var ipt *iptables.IPTables
	var err error
	if iptables.ModeType(selectedmode) == iptables.ModeTypeNFTables || iptables.ModeType(selectedmode) == iptables.ModeTypeLegacy {
		ipt, err = iptables.New(iptables.Mode(iptables.ModeType(selectedmode)))
	} else {
		ipt, err = iptables.New()
	}
	if err != nil {
		return IPTHandler{}, err
	}
	v1, v2, v3, mode := ipt.GetIptablesVersion()
	klog.Infof("Iptables version: %d.%d.%d, mode: %s", v1, v2, v3, mode)
	return IPTHandler{
		Ipt: *ipt,
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
	existingChains, err := h.Ipt.ListChains(table)
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
			liqoChains[forwardChain])
		// Get cluster chains that may have not been removed in table
		chainsToBeRemoved = append(chainsToBeRemoved,
			getSliceContainingString(existingChains, liqonetForwardingExtClusterChainPrefix)...,
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
		if err := h.Ipt.ClearChain(table, chain); err != nil {
			return err
		}
		if err := h.Ipt.DeleteChain(table, chain); err != nil {
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
		rules, err := h.getExistingChainRules(tep.Spec.ClusterIdentity.ClusterID, chain)
		if err != nil {
			return fmt.Errorf("cannot get existing chain rules per cluster %s: %w", tep.Spec.ClusterIdentity, err)
		}
		if err := h.updateSpecificRulesPerChain(chain, rules, newRules, notKeepExistingRules, Append); err != nil {
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
		getClusterForwardExtChain(clusterID),
		getClusterForwardChain(clusterID),
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
			return fmt.Errorf("unable to create chain %s: %w", chain, err)
		}
	}
	return nil
}

// Function that receives a chain name and returns
// the table name the chain should belong to.
func getTableFromChain(chain string) string {
	// First manage the case the chain is a cluster chain
	if strings.Contains(chain, liqonetForwardingExtClusterChainPrefix) ||
		strings.Contains(chain, liqonetForwardingClusterChainPrefix) {
		return filterTable
	}
	if strings.Contains(chain, liqonetPostroutingClusterChainPrefix) ||
		strings.Contains(chain, liqonetPreRoutingMappingClusterChainPrefix) ||
		strings.Contains(chain, liqonetPreroutingClusterChainPrefix) {
		return natTable
	}
	// Chain is a default iptables chain or a Liqo chain
	switch chain {
	case forwardChain, inputChain, liqonetForwardingChain:
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
		return fmt.Errorf("cannot remove chain rules per cluster %s: %w", tep.Spec.ClusterIdentity, err)
	}
	// Delete chains
	err = h.removeChainsPerCluster(tep.Spec.ClusterIdentity.ClusterID)
	if err != nil {
		return fmt.Errorf("cannot remove chains per cluster: %w", err)
	}
	klog.Infof("IPTables config per cluster %s has been deleted", tep.Spec.ClusterIdentity)
	return nil
}

// Function removes rules related to a remote cluster from chains LIQO-POSTROUTING, LIQO-PREROUTING,
// LIQO-FORWARD, LIQO-INPUT.
func (h IPTHandler) deleteChainRulesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	clusterChainRules, err := getChainRulesPerCluster(tep)
	if err != nil {
		return fmt.Errorf("cannot retrieve chain rules per cluster %s: %w", tep.Spec.ClusterIdentity, err)
	}
	for chain, rules := range clusterChainRules {
		err = h.deleteRulesInChain(chain, rules)
		if err != nil {
			return fmt.Errorf("cannot delete cluster %s rules in chain %s: %w", tep.Spec.ClusterIdentity, chain, err)
		}
	}
	return nil
}

func (h IPTHandler) deleteRulesInChain(chain string, rules []IPTableRule) error {
	table := getTableFromChain(chain)
	existingRules, err := h.ListRulesInChain(chain)
	// the next lines parse listed rules in the format used by go-iptables.
	for i := range existingRules {
		existingRule, err := ParseRule(existingRules[i])
		if err != nil {
			return fmt.Errorf("cannot parse rule %q: %w", existingRules[i], err)
		}
		existingRules[i] = existingRule.String()
	}
	if err != nil {
		return fmt.Errorf("unable to list rules in chain %s (table %s): %w", chain, table, err)
	}
	for _, rule := range rules {
		if !slice.ContainsString(existingRules, rule.String()) {
			continue
		}
		// Rule exists, then delete it
		if err := h.Ipt.Delete(table, chain, rule...); err != nil {
			return err
		}
		klog.Infof("Deleted rule %s in chain %s (table %s)", rule, chain, table)
	}
	return nil
}

// Function removes all the chains (and contained rules) related to a remote cluster.
func (h IPTHandler) removeChainsPerCluster(clusterID string) error {
	// Get existing NAT chains
	existingChainsNAT, err := h.Ipt.ListChains(natTable)
	if err != nil {
		return err
	}
	// Get existing Filter chains
	existingChainsFilter, err := h.Ipt.ListChains(filterTable)
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
			if err := h.Ipt.ClearAndDeleteChain(natTable, chain); err != nil {
				return err
			}
			klog.Infof("Deleted chain %s in table %s", chain, getTableFromChain(chain))
			continue
		}
		if getTableFromChain(chain) == filterTable && slice.ContainsString(existingChainsFilter, chain) {
			if !slice.ContainsString(existingChainsFilter, chain) {
				continue
			}
			if err := h.Ipt.ClearChain(filterTable, chain); err != nil {
				return err
			}
			if err := h.Ipt.DeleteChain(filterTable, chain); err != nil {
				return err
			}
			klog.Infof("Deleted chain %s in table %s", chain, getTableFromChain(chain))
		}
	}
	return nil
}

// EnsureForwardExtRules makes sure that the forwarding rules for a given cluster are in place and updated.
func (h IPTHandler) EnsureForwardExtRules(tep *netv1alpha1.TunnelEndpoint) error {
	rules, err := getClusterForwardExtRules(tep)
	if err != nil {
		return err
	}
	return h.updateRulesPerChain(getClusterForwardExtChain(tep.Spec.ClusterIdentity.ClusterID), rules, notKeepExistingRules, Append)
}

// EnsureClusterForwardRules ensures the starting DROP rule for IntraClusterTrafficSegregation security mode.
func (h IPTHandler) EnsureClusterForwardRules(tep *netv1alpha1.TunnelEndpoint) error {
	rules, err := getClusterForwardRules(tep)
	if err != nil {
		return err
	}
	return h.updateRulesPerChain(getClusterForwardChain(tep.Spec.ClusterIdentity.ClusterID), rules, keepExistingRules, Append)
}

func mergeRulesPerCluster(rulesPerCLuster1, rulesPerCLuster2 map[string][]IPTableRule) map[string][]IPTableRule {
	res := map[string][]IPTableRule{}

	for clusterID, rules := range rulesPerCLuster1 {
		res[clusterID] = append(res[clusterID], rules...)
	}
	for clusterID, rules := range rulesPerCLuster2 {
		res[clusterID] = append(res[clusterID], rules...)
	}

	return res
}

// EnsureRulesForClustersForwarding ensures the forward rules for traffic allowed by IntraClusterTrafficSegregation security mode.
func (h IPTHandler) EnsureRulesForClustersForwarding(podsInfo, endpointslicesInfo *sync.Map, ipSetHandler *liqoipset.IPSHandler) error {
	err := ipsetsGarbageCollector(ipSetHandler)
	if err != nil {
		return err
	}

	offloadedPodsRulesPerCluster, err := buildRulesPerClusterForOffloadedPods(podsInfo, ipSetHandler)
	if err != nil {
		return err
	}

	reflectedEndpointslicesRulesPerCluster, err := buildRulesPerClusterForEndpointslicesReflected(endpointslicesInfo, ipSetHandler)
	if err != nil {
		return err
	}

	rulesPerCluster := mergeRulesPerCluster(offloadedPodsRulesPerCluster, reflectedEndpointslicesRulesPerCluster)
	// Add DROP rule as first for each cluster
	dropRule := IPTableRule{"-j", DROP}
	for clusterID := range rulesPerCluster {
		rulesPerCluster[clusterID] = append([]IPTableRule{dropRule}, rulesPerCluster[clusterID]...)
	}

	for clusterID, rules := range rulesPerCluster {
		// Insert each subsequent rule at top of chain as the first rule (DROP rules will be last)
		if err := h.updateRulesPerChain(getClusterForwardChain(clusterID), rules, notKeepExistingRules, Prepend); err != nil {
			return err
		}
	}

	return nil
}

// buildRulesPerClusterForOffloadedPods builds rules allowing traffic from remote clusters towards their pods offloaded on this cluster.
func buildRulesPerClusterForOffloadedPods(podsInfo *sync.Map, ipSetHandler *liqoipset.IPSHandler) (map[string][]IPTableRule, error) {
	// Map of Pod IPs per cluster
	ipsPerCluster := map[string][]string{}

	// Populate Pod IPs per cluster
	podsInfo.Range(func(key, value any) bool {
		podInfo := value.(PodInfo)
		klog.Infof("buildIPSetPerClusterForOffloadedPods: %s", podInfo)
		if _, ok := ipsPerCluster[podInfo.RemoteClusterID]; !ok {
			// Add remote cluster ID key (regardless of pod being deleted or not)
			ipsPerCluster[podInfo.RemoteClusterID] = []string{}
		}
		if !podInfo.Deleting {
			ipsPerCluster[podInfo.RemoteClusterID] = append(ipsPerCluster[podInfo.RemoteClusterID], podInfo.PodIP)
		}
		return true
	})

	// Map of IPTables rules and IP sets per cluster
	rulesPerCluster := map[string][]IPTableRule{}

	// Populate IPTables rules and IP set per cluster
	for clusterID, ips := range ipsPerCluster {
		rulesPerCluster[clusterID] = []IPTableRule{}
		// Create IP set
		setName := getClusterIPSetForOffloadedPods(clusterID)
		ipset, err := ipSetHandler.CreateSet(setName, "")
		if err != nil {
			klog.Infof("Error while creating IP set %q: %w", setName, err)
			return nil, err
		}

		// Clear IP set (just in case it already existed)
		if err := ipSetHandler.FlushSet(ipset.Name); err != nil {
			klog.Infof("Error while deleting all entries from IP set %q: %w", setName, err)
			return nil, err
		}

		if len(ips) > 0 {
			for _, podIP := range ips {
				// Add pod's IP entry to IP set
				if err := ipSetHandler.AddEntry(podIP, ipset); err != nil {
					klog.Infof("Error while adding entry %q to IP set %q: %w", podIP, ipset.Name, err)
					return nil, err
				}
			}

			// Add match-set rule
			rulesPerCluster[clusterID] = append(
				rulesPerCluster[clusterID],
				IPTableRule{
					"-m", "comment", "--comment",
					// WARNING: Never use double-quotes inside the comment, otherwise IpTableRule parser will fail
					fmt.Sprintf("Allows traffic from '%s' only to pods offloaded by that remote cluster", clusterID),
					"-m", setModule,
					"--match-set", ipset.Name, "dst",
					"-j", ACCEPT})
		}
	}

	return rulesPerCluster, nil
}

// buildRulesPerClusterForEndpointslicesReflected builds rules allowing traffic towards endpoints of local services reflected on other clusters.
func buildRulesPerClusterForEndpointslicesReflected(
	endpointslicesInfo *sync.Map,
	ipSetHandler *liqoipset.IPSHandler,
) (map[string][]IPTableRule, error) {
	// Map of Pod IPs per cluster
	endpointSetsPerCluster := map[string]map[string][]string{}

	// Populate Pod IPs per cluster
	endpointslicesInfo.Range(func(key, value any) bool {
		namespacedName := key.(types.NamespacedName)
		endpointsInfo := value.(map[string]EndpointInfo)
		for _, endpointInfo := range endpointsInfo {
			for _, clusterID := range endpointInfo.SrcClusterIDs {
				if _, ok := endpointSetsPerCluster[clusterID]; !ok {
					endpointSetsPerCluster[clusterID] = map[string][]string{}
				}
				if _, ok := endpointSetsPerCluster[clusterID][namespacedName.String()]; !ok {
					endpointSetsPerCluster[clusterID][namespacedName.String()] = []string{}
				}
				if !endpointInfo.Deleting {
					endpointSetsPerCluster[clusterID][namespacedName.String()] = append(
						endpointSetsPerCluster[clusterID][namespacedName.String()], endpointInfo.Address)
				}
			}
		}
		return true
	})

	// Map of IPTables rules and IP sets per cluster
	rulesPerCluster := map[string][]IPTableRule{}

	// Populate IP set per endpointslice and cluster, and  IPTables rules per cluster
	for clusterID, endpointsSets := range endpointSetsPerCluster {
		rulesPerCluster[clusterID] = []IPTableRule{}
		for namespacedName, endpointSet := range endpointsSets {
			namespacedNameChunks := strings.Split(namespacedName, "/")
			if len(namespacedNameChunks) != 2 {
				return nil, fmt.Errorf("invalid value %v", namespacedNameChunks)
			}
			setName := fmt.Sprintf("%s-%s", strings.ToUpper(namespacedNameChunks[1]), strings.Split(clusterID, "-")[0])
			croppedSetName := k8sstrings.ShortenString(setName, IPSetNameMaxLength)

			// Create IP set
			ipset, err := ipSetHandler.CreateSet(croppedSetName, setName)
			if err != nil {
				klog.Infof("Error while creating IP set %q: %w", setName, err)
				return nil, err
			}

			// Clear IP set (just in case it already existed)
			if err := ipSetHandler.FlushSet(ipset.Name); err != nil {
				klog.Infof("Error while deleting all entries from IP set %q: %w", setName, err)
				return nil, err
			}

			if len(endpointSet) > 0 {
				for _, ip := range endpointSet {
					// Add endpoint's IP entry to IP set
					if err := ipSetHandler.AddEntry(ip, ipset); err != nil {
						klog.Infof("Error while adding entry %q to IP set %q: %w", ip, ipset.Name, err)
						return nil, err
					}
				}

				// Add match-set rule
				rulesPerCluster[clusterID] = append(
					rulesPerCluster[clusterID],
					IPTableRule{
						"-m", setModule,
						"--match-set", ipset.Name, "dst",
						"-j", ACCEPT})
			}
		}
	}

	return rulesPerCluster, nil
}

// ipsetsGarbageCollector delete empty IPsets for a given IPSetHandler.
func ipsetsGarbageCollector(ipSetHandler *liqoipset.IPSHandler) error {
	IPSets, err := ipSetHandler.ListSets()
	if err != nil {
		return err
	}
	for _, setName := range IPSets {
		if setName != "" {
			entries, err := ipSetHandler.ListEntries(setName)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				err = ipSetHandler.DestroySet(setName)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// EnsurePostroutingRules makes sure that the postrouting rules for a given cluster are in place and updated.
func (h IPTHandler) EnsurePostroutingRules(tep *netv1alpha1.TunnelEndpoint) error {
	rules, err := getPostroutingRules(tep)
	if err != nil {
		return err
	}
	return h.updateRulesPerChain(getClusterPostRoutingChain(tep.Spec.ClusterIdentity.ClusterID), rules, notKeepExistingRules, Append)
}

// EnsurePreroutingRulesPerTunnelEndpoint makes sure that the prerouting rules extracted from a
// TunnelEndpoint resource are place and updated.
func (h IPTHandler) EnsurePreroutingRulesPerTunnelEndpoint(tep *netv1alpha1.TunnelEndpoint) error {
	rules, err := getPreRoutingRulesPerTunnelEndpoint(tep)
	if err != nil {
		return err
	}
	return h.updateRulesPerChain(getClusterPreRoutingChain(tep.Spec.ClusterIdentity.ClusterID), rules, notKeepExistingRules, Append)
}

// EnsurePreroutingRulesPerNatMapping makes sure that the prerouting rules extracted from a
// NatMapping resource are place and updated.
func (h IPTHandler) EnsurePreroutingRulesPerNatMapping(nm *netv1alpha1.NatMapping) error {
	clusterID := nm.Spec.ClusterID
	rules, err := getPreRoutingRulesPerNatMapping(nm)
	if err != nil {
		return err
	}
	return h.updateRulesPerChain(getClusterPreRoutingMappingChain(clusterID), rules, notKeepExistingRules, Append)
}

func getPreRoutingRulesPerTunnelEndpoint(tep *netv1alpha1.TunnelEndpoint) ([]IPTableRule, error) {
	// Check tep fields
	if err := liqonetutils.CheckTep(tep); err != nil {
		return nil, fmt.Errorf("invalid TunnelEndpoint resource: %w", err)
	}
	localPodCIDR := tep.Spec.LocalPodCIDR
	localRemappedPodCIDR, remotePodCIDR := liqonetutils.GetPodCIDRS(tep)

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
	existingRules, err := h.Ipt.List(getTableFromChain(chain), chain)
	if err != nil {
		return nil, err
	}
	rules := make([]string, 0)
	ruleToRemove := "-N " + chain
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
	chainsList, err := h.Ipt.ListChains(table)
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
	err = h.Ipt.NewChain(table, newChain)
	if err != nil {
		return fmt.Errorf("unable to create %s chain in %s table: %w", newChain, table, err)
	}
	klog.Infof("Created chain %s in table %s", newChain, table)
	return nil
}

func (h IPTHandler) insertLiqoRuleIfNotExists(chain string, rule IPTableRule) error {
	table := getTableFromChain(chain)
	// Get the list of rules for the specified chain
	existingRules, err := h.Ipt.List(table, chain)
	if err != nil {
		return fmt.Errorf("unable to get the rules in %s chain in %s table : %w", chain, table, err)
	}
	// Check if the rule exists and at the same time if it exists more then once
	numOccurrences := 0
	for _, existingRule := range existingRules {
		if strings.Contains(existingRule, rule.String()) {
			numOccurrences++
		}
	}
	// If the occurrences if greater then one, remove the rule
	if numOccurrences > 1 {
		for i := 0; i < numOccurrences; i++ {
			if err = h.Ipt.Delete(table, chain, rule...); err != nil {
				return fmt.Errorf("unable to delete iptable rule %q: %w", rule, err)
			}
		}
		if err = h.Ipt.Insert(table, chain, 1, rule...); err != nil {
			return fmt.Errorf("unable to insert iptable rule %q: %w", rule, err)
		}
	}
	if numOccurrences == 1 {
		// If the occurrence is one then check the position and if not at the first one we delete and reinsert it
		if strings.Contains(existingRules[0], rule.String()) {
			return nil
		}
		if err = h.Ipt.Delete(table, chain, rule...); err != nil {
			return fmt.Errorf("unable to delete iptable rule %q: %w", rule, err)
		}
		if err = h.Ipt.Insert(table, chain, 1, rule...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule %q: %w", rule, err)
		}
		return nil
	}
	if numOccurrences == 0 {
		// If the occurrence is zero then insert the rule in first position
		if err = h.Ipt.Insert(table, chain, 1, rule...); err != nil {
			return fmt.Errorf("unable to insert iptable rule %q: %w", rule, err)
		}
		klog.Infof("Inserted rule '%s' in chain %s of table %s", rule, chain, table)
	}
	return nil
}

// checkRuleIsOutdated checks if a rule is outdated,
// with respect to the new rules.
func (itr IPTableRule) checkRuleIsOutdated(newRules []IPTableRule) bool {
	for _, newRule := range newRules {
		if strings.Contains(itr.String(), newRule.String()) {
			return false
		}
	}
	return true
}

// Function to update specific rules in a given chain.
func (h IPTHandler) updateSpecificRulesPerChain(
	chain string,
	existingRules []string,
	newRules []IPTableRule,
	keepExistingRules bool,
	ruleInsertionStrategy RuleInsertionStrategyType,
) error {
	// Get iptables table
	table := getTableFromChain(chain)

	for _, existingRuleString := range existingRules {
		if keepExistingRules {
			break
		}
		existingRule, err := ParseRule(existingRuleString)
		if err != nil {
			return fmt.Errorf("cannot parse rule %q: %w", existingRuleString, err)
		}

		// Remove existing rule that is not in the set of new rules,
		// it is outdated.
		outdated := existingRule.checkRuleIsOutdated(newRules)
		if outdated {
			if err := h.Ipt.Delete(table, chain, existingRule...); err != nil {
				return fmt.Errorf("unable to delete outdated rule %s from chain %s (table %s): %w",
					existingRule, chain, table, err)
			}
			klog.Infof("Deleted outdated rule %s from chain %s (table %s)", existingRule, chain, table)
		}
	}
	err := h.insertRulesIfNotPresent(table, chain, newRules, ruleInsertionStrategy)
	if err != nil {
		return fmt.Errorf("cannot add new rules in chain %s (table %s): %w", chain, table, err)
	}
	return nil
}

// Function to updates rules in a given chain.
func (h IPTHandler) updateRulesPerChain(
	chain string,
	newRules []IPTableRule,
	keepExistingRules bool,
	ruleInsertionStrategy RuleInsertionStrategyType,
) error {
	existingRules, err := h.ListRulesInChain(chain)
	if err != nil {
		return fmt.Errorf("cannot list rules in chain %s (table %s): %w", chain, getTableFromChain(chain), err)
	}
	return h.updateSpecificRulesPerChain(chain, existingRules, newRules, keepExistingRules, ruleInsertionStrategy)
}

func (h IPTHandler) insertRulesIfNotPresent(table, chain string, rules []IPTableRule, ruleInsertionStrategy RuleInsertionStrategyType) error {
	for _, rule := range rules {
		exists, err := h.Ipt.Exists(table, chain, rule...)
		if err != nil {
			klog.Errorf("unable to check if rule '%s' exists in chain %s in table %s: %w", rule, chain, table, err)
			return err
		}
		if !exists {
			switch ruleInsertionStrategy {
			case Prepend:
				if err := h.Ipt.Insert(table, chain, 1, rule...); err != nil {
					return err
				}
			case Append:
				if err := h.Ipt.Append(table, chain, rule...); err != nil {
					return err
				}
			}
			klog.Infof("%s rule '%s' in chain %s (table %s)", ruleInsertionStrategy, rule, chain, table)
		}
	}
	return nil
}

func getClusterForwardExtRules(tep *netv1alpha1.TunnelEndpoint) ([]IPTableRule, error) {
	if err := liqonetutils.CheckTep(tep); err != nil {
		return nil, fmt.Errorf("invalid TunnelEndpoint resource: %w", err)
	}
	return []IPTableRule{
		{"-m", "comment", "--comment",
			// WARNING: Never use double-quotes inside the comment, otherwise IpTableRule parser will fail
			fmt.Sprintf("Avoid forwarding '%s' remapped %s to %s", tep.Spec.ClusterIdentity.ClusterName, consts.ExternalCIDR, consts.GatewayVethName),
			"-j", DROP},
	}, nil
}

func getClusterForwardRules(tep *netv1alpha1.TunnelEndpoint) ([]IPTableRule, error) {
	if err := liqonetutils.CheckTep(tep); err != nil {
		return nil, fmt.Errorf("invalid TunnelEndpoint resource: %w", err)
	}

	return []IPTableRule{
		{"-j", DROP},
	}, nil
}

func getPostroutingRules(tep *netv1alpha1.TunnelEndpoint) ([]IPTableRule, error) {
	if err := liqonetutils.CheckTep(tep); err != nil {
		return nil, fmt.Errorf("invalid TunnelEndpoint resource: %w", err)
	}
	localPodCIDR := tep.Spec.LocalPodCIDR
	localRemappedPodCIDR, remotePodCIDR := liqonetutils.GetPodCIDRS(tep)
	_, remoteExternalCIDR := liqonetutils.GetExternalCIDRS(tep)
	if localRemappedPodCIDR != consts.DefaultCIDRValue {
		// Get the first IP address from the podCIDR of the local cluster
		// in this case it is the podCIDR to which the local podCIDR has bee remapped by the remote peering cluster
		natIP, err := liqonetutils.GetFirstIP(localRemappedPodCIDR)
		if err != nil {
			klog.Errorf("Unable to get the IP from localPodCidr %s for remote cluster %s used to NAT the traffic from localhosts to remote hosts",
				localRemappedPodCIDR, tep.Spec.ClusterIdentity)
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
	natIP, err := liqonetutils.GetFirstIP(localPodCIDR)
	if err != nil {
		klog.Errorf("Unable to get the IP from localPodCidr %s for cluster %v used to NAT the traffic from localhosts to remote hosts",
			tep.Spec.RemotePodCIDR, tep.Spec.ClusterIdentity)
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
	if err := liqonetutils.CheckTep(tep); err != nil {
		return nil, fmt.Errorf("invalid TunnelEndpoint resource: %w", err)
	}
	clusterID := tep.Spec.ClusterIdentity.ClusterID
	localRemappedPodCIDR, remotePodCIDR := liqonetutils.GetPodCIDRS(tep)
	localRemappedExternalCIDR, remoteExternalCIDR := liqonetutils.GetExternalCIDRS(tep)

	// Init chain rules
	chainRules := make(map[string][]IPTableRule)
	chainRules[liqonetPostroutingChain] = make([]IPTableRule, 0)
	chainRules[liqonetPreroutingChain] = make([]IPTableRule, 0)
	chainRules[liqonetForwardingChain] = make([]IPTableRule, 0)

	// For these rules, source in not necessary since
	// the remotePodCIDR is unique in home cluster
	chainRules[liqonetPostroutingChain] = append(chainRules[liqonetPostroutingChain],
		IPTableRule{
			"-d", remotePodCIDR,
			"-m", "comment", "--comment", getClusterPostRoutingChainComment(tep.Spec.ClusterIdentity.ClusterName, consts.PodCIDR),
			"-j", getClusterPostRoutingChain(clusterID)},
		IPTableRule{
			"-d", remoteExternalCIDR,
			"-m", "comment", "--comment", getClusterPostRoutingChainComment(tep.Spec.ClusterIdentity.ClusterName, consts.ExternalCIDR),
			"-j", getClusterPostRoutingChain(clusterID)})

	chainRules[liqonetForwardingChain] = append(chainRules[liqonetForwardingChain],
		IPTableRule{
			"-s", remotePodCIDR,
			"-d", localRemappedExternalCIDR,
			"-j", getClusterForwardExtChain(clusterID)},
		// rule accepting packets marked by connection tracking as:
		// - ESTABLISHED, necessary to allow response traffic from addresses that cannot start connections
		// - RELATED, useful to allow possible traffic from a new connection, but associated with
		//   an existing one,e.g. an FTP data transfer, or an ICMP error
		IPTableRule{
			"-m", conntrackModule,
			"--ctstate", (ESTABLISHED + "," + RELATED),
			"-j", ACCEPT},
		IPTableRule{
			"-s", remotePodCIDR,
			"-m", "comment", "--comment",
			// WARNING: Never use double-quotes inside the comment, otherwise IpTableRule parser will fail
			fmt.Sprintf("Contains rules allowing traffic segregation with '%s' cluster", tep.Spec.ClusterIdentity.ClusterName),
			"-j", getClusterForwardChain(clusterID)},
	)

	chainRules[liqonetPreroutingChain] = append(chainRules[liqonetPreroutingChain],
		IPTableRule{
			"-s", remotePodCIDR,
			"-d", localRemappedExternalCIDR,
			"-m", "comment", "--comment", getClusterPreRoutingChainComment(tep.Spec.ClusterIdentity.ClusterName, consts.ExternalCIDR),
			"-j", getClusterPreRoutingMappingChain(clusterID)})

	if localRemappedPodCIDR != consts.DefaultCIDRValue {
		// For the following rule, source is necessary
		// because more remote clusters could have
		// remapped home PodCIDR in the same way, then only use dst is not enough.
		chainRules[liqonetPreroutingChain] = append(chainRules[liqonetPreroutingChain],
			IPTableRule{
				"-s", remotePodCIDR,
				"-d", localRemappedPodCIDR,
				"-m", "comment", "--comment", getClusterPreRoutingChainComment(tep.Spec.ClusterIdentity.ClusterName, consts.PodCIDR),
				"-j", getClusterPreRoutingChain(clusterID)})
	}
	return chainRules, nil
}

func getClusterPreRoutingChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterPreRoutingChainComment(clusterName, typeCIDR string) string {
	// WARNING: Never use double-quotes inside the comment, otherwise IpTableRule parser will fail.
	return fmt.Sprintf("DNAT '%s' traffic for local %s", clusterName, typeCIDR)
}

func getClusterPostRoutingChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetPostroutingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterPostRoutingChainComment(clusterName, typeCIDR string) string {
	// WARNING: Never use double-quotes inside the comment, otherwise IpTableRule parser will fail
	return fmt.Sprintf("SNAT local traffic for '%s' %s", clusterName, typeCIDR)
}

func getClusterForwardExtChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetForwardingExtClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterForwardChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetForwardingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterPreRoutingMappingChain(clusterID string) string {
	return fmt.Sprintf("%s%s", liqonetPreRoutingMappingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func getClusterIPSetForOffloadedPods(clusterID string) string {
	return fmt.Sprintf("%s-%s", liqonetOffloadedPodsIPSetPrefix, strings.Split(clusterID, "-")[0])
}

// Function that returns the set of Liqo default chains.
// Value is the Liqo chain, key is the related default chain.
// Example: key: PREROUTING, value: LIQO-PREROUTING.
func getLiqoChains() map[string]string {
	return map[string]string{
		preroutingChain:  liqonetPreroutingChain,
		postroutingChain: liqonetPostroutingChain,
		forwardChain:     liqonetForwardingChain,
	}
}

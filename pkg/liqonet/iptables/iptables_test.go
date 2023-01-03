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
	"fmt"
	"os"
	"strings"

	. "github.com/coreos/go-iptables/iptables"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"

	discv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/errors"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
)

const (
	clusterName1          = "clusterName1"
	clusterID1            = "clusterID1"
	invalidValue          = "an invalid value"
	localNATPodCIDRValue  = "11.0.0.0/24"
	remoteNATPodCIDRValue = "10.70.0.0/24"
	oldIP1                = "10.0.0.2"
	oldIP2                = "12.0.0.4"
	newIP1                = "10.0.3.2"
	newIP2                = "10.0.5.2"
)

var (
	h           IPTHandler
	ipt         *IPTables
	tep         *netv1alpha1.TunnelEndpoint
	nm          *netv1alpha1.NatMapping
	natMappings = netv1alpha1.Mappings{
		oldIP1: newIP1,
		oldIP2: newIP2,
	}
	validTep = &netv1alpha1.TunnelEndpoint{
		Spec: netv1alpha1.TunnelEndpointSpec{
			ClusterIdentity:       discv1alpha1.ClusterIdentity{ClusterID: clusterID1, ClusterName: clusterName1},
			LocalPodCIDR:          "192.168.0.0/24",
			LocalNATPodCIDR:       "192.168.1.0/24",
			LocalExternalCIDR:     "192.168.3.0/24",
			LocalNATExternalCIDR:  "192.168.4.0/24",
			RemotePodCIDR:         "10.0.0.0/24",
			RemoteNATPodCIDR:      "10.60.0.0/24",
			RemoteExternalCIDR:    "10.0.1.0/24",
			RemoteNATExternalCIDR: "192.168.5.0/24",
		},
	}
)

var _ = Describe("iptables", func() {
	Describe("Init", func() {
		BeforeEach(func() {
			err := h.Terminate()
			Expect(err).ToNot(HaveOccurred())
		})
		Context("Call func", func() {
			It("should produce no errors and create Liqo chains", func() {
				err := h.Init()
				Expect(err).ToNot(HaveOccurred())

				// Retrieve NAT chains and Filter chains
				natChains, err := ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				// Check existence of LIQO-POSTROUTING chain
				Expect(natChains).To(ContainElement(liqonetPostroutingChain))
				// Check existence of rule in POSTROUTING
				postRoutingRules, err := h.ListRulesInChain(postroutingChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(postRoutingRules).To(ContainElement(fmt.Sprintf("-j %s", liqonetPostroutingChain)))

				// Check existence of LIQO-PREROUTING chain
				Expect(natChains).To(ContainElement(liqonetPostroutingChain))
				// Check existence of rule in PREROUTING
				preRoutingRules, err := h.ListRulesInChain(preroutingChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(preRoutingRules).To(ContainElement(fmt.Sprintf("-j %s", liqonetPreroutingChain)))

				// Check existence of LIQO-FORWARD chain
				Expect(filterChains).To(ContainElement(liqonetForwardingChain))
				// Check existence of rule in FORWARD
				forwardRules, err := h.ListRulesInChain(forwardChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(forwardRules).To(ContainElement(fmt.Sprintf("-j %s", liqonetForwardingChain)))
			})
		})

		Context("Call func twice", func() {
			It("should produce no errors and insert all the rules", func() {
				err := h.Init()
				Expect(err).ToNot(HaveOccurred())

				// Check only POSTROUTING chain and rules

				// Retrieve NAT chains
				natChains, err := ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())

				// Check existence of LIQO-POSTROUTING chain
				Expect(natChains).To(ContainElement(liqonetPostroutingChain))
				// Check existence of rule in POSTROUTING
				postRoutingRules, err := h.ListRulesInChain(postroutingChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(postRoutingRules).To(ContainElements([]string{
					fmt.Sprintf("-j %s", liqonetPostroutingChain),
				}))
			})
		})
	})

	Describe("EnsureChainRulesPerCluster", func() {
		BeforeEach(func() {
			err := h.EnsureChainsPerCluster(clusterID1)
			Expect(err).ToNot(HaveOccurred())
			tep = validTep.DeepCopy()
			nm = &netv1alpha1.NatMapping{
				Spec: netv1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: make(netv1alpha1.Mappings),
				},
			}
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).ToNot(HaveOccurred())
		})
		Context(fmt.Sprintf("If all parameters are valid and LocalNATPodCIDR is equal to "+
			"constant value %s in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It(`should add chain rules in POSTROUTING, INPUT, FORWARD but not in PREROUTING`, func() {
				tep.Spec.LocalNATPodCIDR = consts.DefaultCIDRValue

				Expect(h.EnsureChainRulesPerCluster(tep)).To(Succeed())

				// Check existence of rule in LIQO-POSTROUTING chain
				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).ToNot(HaveOccurred())
				expectedRules := []string{
					fmt.Sprintf("-d %s -m \"comment\" --comment %q -j %s",
						tep.Spec.RemoteNATPodCIDR,
						getClusterPostRoutingChainComment(tep.Spec.ClusterIdentity.ClusterName, consts.PodCIDR),
						getClusterPostRoutingChain(tep.Spec.ClusterIdentity.ClusterID)),
					fmt.Sprintf("-d %s -m \"comment\" --comment %q -j %s",
						tep.Spec.RemoteNATExternalCIDR,
						getClusterPostRoutingChainComment(tep.Spec.ClusterIdentity.ClusterName, consts.ExternalCIDR),
						getClusterPostRoutingChain(tep.Spec.ClusterIdentity.ClusterID))}
				Expect(normalizeRules(postRoutingRules)).To(ContainElements(normalizeRules(expectedRules)))

				// Check existence of rules in LIQO-PREROUTING chain
				// Rule for NAT-ting the PodCIDR should not be present
				preRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).ToNot(HaveOccurred())
				normalizeRules(preRoutingRules)
				expectedRule := fmt.Sprintf("-s %s -d %s -m \"comment\" --comment %s -j %s",
					tep.Spec.RemoteNATPodCIDR, tep.Spec.LocalNATExternalCIDR,
					getClusterPreRoutingChainComment(tep.Spec.ClusterIdentity.ClusterName, consts.ExternalCIDR),
					getClusterPreRoutingMappingChain(tep.Spec.ClusterIdentity.ClusterID))
				Expect(normalizeRuleString(expectedRule)).To(Equal(normalizeRuleString(preRoutingRules[0])))

				// Check existence of rule in LIQO-FORWARD chain
				forwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				localRemappedExternalCIDR, _ := liqonetutils.GetExternalCIDRS(tep)
				expectedRules = []string{
					fmt.Sprintf("-s %s -d %s -j %s", tep.Spec.RemoteNATPodCIDR, localRemappedExternalCIDR,
						getClusterForwardExtChain(tep.Spec.ClusterIdentity.ClusterID)),
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(normalizeRules(forwardRules)).To(ContainElements(normalizeRules(expectedRules)))
			})
		})

		Context(fmt.Sprintf("If all parameters are valid and LocalNATPodCIDR is not equal to "+
			"constant value %s in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It(`should add chain rules in POSTROUTING, INPUT, FORWARD and PREROUTING`, func() {
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).ToNot(HaveOccurred())

				// Check existence of rule in LIQO-PREROUTING chain
				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).ToNot(HaveOccurred())
				expectedRules := []string{
					fmt.Sprintf("-d %s -m \"comment\" --comment %q -j %s",
						tep.Spec.RemoteNATPodCIDR,
						getClusterPostRoutingChainComment(clusterName1, consts.PodCIDR),
						getClusterPostRoutingChain(tep.Spec.ClusterIdentity.ClusterID)),
					fmt.Sprintf("-d %s -m \"comment\" --comment %q -j %s",
						tep.Spec.RemoteNATExternalCIDR,
						getClusterPostRoutingChainComment(clusterName1, consts.ExternalCIDR),
						getClusterPostRoutingChain(tep.Spec.ClusterIdentity.ClusterID))}
				Expect(normalizeRules(postRoutingRules)).To(ContainElements(normalizeRules(expectedRules)))

				// Check existence of rule in LIQO-FORWARD chain
				forwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				Expect(err).ToNot(HaveOccurred())
				localRemappedExternalCIDR, _ := liqonetutils.GetExternalCIDRS(tep)
				expectedRules = []string{
					fmt.Sprintf("-s %s -d %s -j %s",
						tep.Spec.RemoteNATPodCIDR, localRemappedExternalCIDR,
						getClusterForwardExtChain(tep.Spec.ClusterIdentity.ClusterID)),
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(normalizeRules(forwardRules)).To(ContainElements(normalizeRules(expectedRules)))

				// Check existence of rule in LIQO-PREROUTING chain
				preRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).ToNot(HaveOccurred())
				expectedRules = []string{
					fmt.Sprintf("-s %s -d %s -m \"comment\" --comment %q -j %s",
						tep.Spec.RemoteNATPodCIDR, tep.Spec.LocalNATExternalCIDR,
						getClusterPreRoutingChainComment(clusterName1, consts.ExternalCIDR),
						getClusterPreRoutingMappingChain(tep.Spec.ClusterIdentity.ClusterID)),
					fmt.Sprintf("-s %s -d %s -m \"comment\" --comment %q -j %s",
						tep.Spec.RemoteNATPodCIDR, tep.Spec.LocalNATPodCIDR,
						getClusterPreRoutingChainComment(clusterName1, consts.PodCIDR),
						getClusterPreRoutingChain(tep.Spec.ClusterIdentity.ClusterID)),
				}
				Expect(normalizeRules(preRoutingRules)).To(ContainElements(normalizeRules(expectedRules)))
			})
		})

		Context(fmt.Sprintf("If RemoteNATPodCIDR is different from constant value %s "+
			"and is not a valid network in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.RemoteNATPodCIDR = invalidValue
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.RemoteNATPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})

		Context(fmt.Sprintf("If RemoteNATPodCIDR is equal to constant value %s "+
			"and RemotePodCIDR is not a valid network in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.RemoteNATPodCIDR = consts.DefaultCIDRValue
				tep.Spec.RemotePodCIDR = invalidValue
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.PodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})

		Context(fmt.Sprintf("If LocalNATPodCIDR is not equal to constant value %s "+
			"and is not a valid network in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.LocalNATPodCIDR = invalidValue
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.LocalNATPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})

		Context(fmt.Sprintf("If LocalNATPodCIDR is not equal to constant value %s "+
			"and is not a valid network in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.LocalNATPodCIDR = invalidValue
				err := h.Init()
				Expect(err).ToNot(HaveOccurred())
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.LocalNATPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})

		Context("If there are already some rules in chains but they are not in new rules", func() {
			It(`should remove existing rules that are not in the set of new rules and add new rules`, func() {
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).ToNot(HaveOccurred())

				clusterPostRoutingChain := liqonetPostroutingClusterChainPrefix + strings.Split(tep.Spec.ClusterIdentity.ClusterID, "-")[0]

				// Get rule that will be removed
				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).ToNot(HaveOccurred())
				outdatedRule := postRoutingRules[0]

				// Modify resource
				tep.Spec.RemoteNATPodCIDR = remoteNATPodCIDRValue

				// Second call
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).ToNot(HaveOccurred())
				newPostRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).ToNot(HaveOccurred())

				// Check if new rules has been added.
				expectedRules := []string{
					fmt.Sprintf(`-d %s -m "comment" --comment %q -j %s`,
						tep.Spec.RemoteNATPodCIDR,
						getClusterPostRoutingChainComment(clusterName1, consts.PodCIDR),
						clusterPostRoutingChain),
					fmt.Sprintf(`-d %s -m "comment" --comment %q -j %s`, tep.Spec.RemoteNATExternalCIDR,
						getClusterPostRoutingChainComment(clusterName1, consts.ExternalCIDR),
						clusterPostRoutingChain),
				}
				Expect(normalizeRules(newPostRoutingRules)).To(ContainElements(normalizeRules(expectedRules)))

				// Check if outdated rule has been removed
				Expect(normalizeRules(newPostRoutingRules)).ToNot(ContainElement(normalizeRuleString(outdatedRule)))
			})
		})
	})

	Describe("EnsureChainsPerCluster", func() {
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).ToNot(HaveOccurred())
			tep = validTep.DeepCopy()
			nm = &netv1alpha1.NatMapping{
				Spec: netv1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: make(netv1alpha1.Mappings),
				},
			}
		})
		Context("Passing an empty clusterID", func() {
			It("Should return a WrongParameter error", func() {
				err := h.EnsureChainsPerCluster("")
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", consts.ClusterIDLabelName, errors.StringNotEmpty)))
			})
		})
		Context("If chains do not exist yet", func() {
			It("Should create chains", func() {
				err := h.EnsureChainsPerCluster(clusterID1)
				Expect(err).ToNot(HaveOccurred())

				natChains, err := ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				// Check if filter chains have been created by function.
				Expect(filterChains).To(ContainElements(
					liqonetForwardingExtClusterChainPrefix + strings.Split(clusterID1, "-")[0],
				))

				// Check if nat chains have been created by function.
				Expect(natChains).To(ContainElements(
					liqonetPostroutingClusterChainPrefix+strings.Split(clusterID1, "-")[0],
					liqonetPreroutingClusterChainPrefix+strings.Split(clusterID1, "-")[0],
					liqonetPreRoutingMappingClusterChainPrefix+strings.Split(clusterID1, "-")[0],
				))
			})
		})
		Context("If chains already exist", func() {
			It("Should be a nop", func() {
				err := h.EnsureChainsPerCluster(clusterID1)
				Expect(err).ToNot(HaveOccurred())

				natChains, err := ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				// Check if filter chains have been created by function.
				Expect(filterChains).To(ContainElements(
					liqonetForwardingExtClusterChainPrefix + strings.Split(clusterID1, "-")[0],
				))

				// Check if nat chains have been created by function.
				Expect(natChains).To(ContainElements(
					liqonetPostroutingClusterChainPrefix+strings.Split(clusterID1, "-")[0],
					liqonetPreroutingClusterChainPrefix+strings.Split(clusterID1, "-")[0],
					liqonetPreRoutingMappingClusterChainPrefix+strings.Split(clusterID1, "-")[0],
				))

				err = h.EnsureChainsPerCluster(clusterID1)
				Expect(err).ToNot(HaveOccurred())

				// Get new chains and assert that they have not changed.
				newNatChains, err := ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				newFilterChains, err := ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				Expect(newNatChains).To(Equal(natChains))
				Expect(newFilterChains).To(Equal(filterChains))
			})
		})
	})

	Describe("Terminate", func() {
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).ToNot(HaveOccurred())
		})
		Context("If there is not a Liqo configuration", func() {
			It("should be a nop", func() {
				err := h.Terminate()
				Expect(err).ToNot(HaveOccurred())
			})
		})
		Context("If there is a Liqo configuration and Liqo chains are not empty", func() {
			It("should remove Liqo configuration", func() {
				err := h.Init()
				Expect(err).ToNot(HaveOccurred())

				// Add a remote cluster config and do not terminate it
				err = h.EnsureChainsPerCluster(clusterID1)
				Expect(err).ToNot(HaveOccurred())
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).ToNot(HaveOccurred())

				err = h.Terminate()
				Expect(err).ToNot(HaveOccurred())

				// Check if Liqo chains do exist
				natChains, err := h.ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				filterChains, err := h.ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				Expect(natChains).ToNot(ContainElements(getLiqoChains()))
				Expect(filterChains).ToNot(ContainElements(getLiqoChains()))

				// Check if Liqo rules have been removed

				postRoutingRules, err := h.ListRulesInChain(postroutingChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(postRoutingRules).ToNot(ContainElements([]string{
					fmt.Sprintf("-j %s", liqonetPostroutingChain),
					fmt.Sprintf("-j %s", MASQUERADE),
				}))

				preRoutingRules, err := h.ListRulesInChain(preroutingChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(preRoutingRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetPreroutingChain)))

				forwardRules, err := h.ListRulesInChain(forwardChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(forwardRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetForwardingChain)))
			})
		})
		Context("If there is a Liqo configuration and Liqo chains are empty", func() {
			It("should remove Liqo configuration", func() {
				err := h.Init()
				Expect(err).ToNot(HaveOccurred())

				err = h.Terminate()
				Expect(err).ToNot(HaveOccurred())

				// Check if Liqo chains do exist
				natChains, err := h.ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				filterChains, err := h.ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				Expect(natChains).ToNot(ContainElements(getLiqoChains()))
				Expect(filterChains).ToNot(ContainElements(getLiqoChains()))

				// Check if Liqo rules have been removed

				postRoutingRules, err := h.ListRulesInChain(postroutingChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(postRoutingRules).ToNot(ContainElements([]string{
					fmt.Sprintf("-j %s", liqonetPostroutingChain),
				}))

				preRoutingRules, err := h.ListRulesInChain(preroutingChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(preRoutingRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetPreroutingChain)))

				forwardRules, err := h.ListRulesInChain(forwardChain)
				Expect(err).ToNot(HaveOccurred())
				Expect(forwardRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetForwardingChain)))
			})
		})
	})

	Describe("RemoveIPTablesConfigurationPerCluster", func() {
		Context("If there are no iptables rules/chains related to remote cluster", func() {
			It("should be a nop", func() {
				// Read current configuration in order to compare it
				// with the configuration after func call.
				natChains, err := ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).ToNot(HaveOccurred())
				forwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				Expect(err).ToNot(HaveOccurred())
				preRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).ToNot(HaveOccurred())

				err = h.RemoveIPTablesConfigurationPerCluster(tep)
				Expect(err).ToNot(HaveOccurred())

				newNatChains, err := ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				newFilterChains, err := ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				newPostRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).ToNot(HaveOccurred())
				newForwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				Expect(err).ToNot(HaveOccurred())
				newPreRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).ToNot(HaveOccurred())

				// Assert configs are equal
				Expect(natChains).To(Equal(newNatChains))
				Expect(filterChains).To(Equal(newFilterChains))
				Expect(postRoutingRules).To(Equal(newPostRoutingRules))
				Expect(preRoutingRules).To(Equal(newPreRoutingRules))
				Expect(forwardRules).To(Equal(newForwardRules))
			})
		})
		Context("If cluster has an iptables configuration", func() {
			It("should delete chains and rules per cluster", func() {
				err := h.EnsureChainsPerCluster(tep.Spec.ClusterIdentity.ClusterID)
				Expect(err).ToNot(HaveOccurred())

				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).ToNot(HaveOccurred())

				// Get chains related to cluster
				natChains, err := ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				// If chain contains clusterID, then it is related to cluster
				natChainsPerCluster := getSliceContainingString(natChains, tep.Spec.ClusterIdentity.ClusterID)
				filterChainsPerCluster := getSliceContainingString(filterChains, tep.Spec.ClusterIdentity.ClusterID)

				// Get rules related to cluster in each chain
				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).ToNot(HaveOccurred())
				forwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				Expect(err).ToNot(HaveOccurred())
				preRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).ToNot(HaveOccurred())

				clusterPostRoutingRules := getSliceContainingString(postRoutingRules, tep.Spec.ClusterIdentity.ClusterID)
				clusterPreRoutingRules := getSliceContainingString(preRoutingRules, tep.Spec.ClusterIdentity.ClusterID)
				clusterForwardRules := getSliceContainingString(forwardRules, tep.Spec.ClusterIdentity.ClusterID)

				err = h.RemoveIPTablesConfigurationPerCluster(tep)
				Expect(err).ToNot(HaveOccurred())

				// Read config after call
				newNatChains, err := ipt.ListChains(natTable)
				Expect(err).ToNot(HaveOccurred())
				newFilterChains, err := ipt.ListChains(filterTable)
				Expect(err).ToNot(HaveOccurred())

				newPostRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).ToNot(HaveOccurred())
				newForwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				Expect(err).ToNot(HaveOccurred())
				newPreRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).ToNot(HaveOccurred())

				// Check chains have been removed
				Expect(newNatChains).ToNot(ContainElements(natChainsPerCluster))
				Expect(newFilterChains).ToNot(ContainElements(filterChainsPerCluster))

				// Check rules have been removed
				Expect(newPostRoutingRules).ToNot(ContainElements(clusterPostRoutingRules))
				Expect(newForwardRules).ToNot(ContainElements(clusterForwardRules))
				Expect(newPreRoutingRules).ToNot(ContainElements(clusterPreRoutingRules))
			})
		})
	})

	Describe("EnsurePostroutingRules", func() {
		BeforeEach(func() {
			err := h.EnsureChainsPerCluster(clusterID1)
			Expect(err).ToNot(HaveOccurred())
			tep = validTep.DeepCopy()
			nm = &netv1alpha1.NatMapping{
				Spec: netv1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: make(netv1alpha1.Mappings),
				},
			}
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).ToNot(HaveOccurred())
		})
		Context("If tep has an empty clusterID", func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.ClusterIdentity.ClusterID = ""
				err := h.EnsurePostroutingRules(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.ClusterIDLabelName, errors.StringNotEmpty)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context("If tep has an invalid LocalPodCIDR", func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.LocalPodCIDR = invalidValue
				err := h.EnsurePostroutingRules(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.LocalPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context("If tep has an invalid LocalNATPodCIDR", func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.LocalNATPodCIDR = invalidValue
				err := h.EnsurePostroutingRules(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.LocalNATPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context(fmt.Sprintf("If tep has Status.RemoteNATPodCIDR = %s and an invalid Spec.PodCIDR",
			consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.RemoteNATPodCIDR = consts.DefaultCIDRValue
				tep.Spec.RemotePodCIDR = invalidValue
				err := h.EnsurePostroutingRules(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.PodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context(fmt.Sprintf("If tep has Status.RemoteNATExternalCIDR = %s and an invalid Spec.ExternalCIDR",
			consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.RemoteNATExternalCIDR = consts.DefaultCIDRValue
				tep.Spec.RemoteExternalCIDR = invalidValue
				err := h.EnsurePostroutingRules(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.ExternalCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context(fmt.Sprintf("If tep has an invalid Status.RemoteNATExternalCIDR != %s",
			consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.RemoteNATExternalCIDR = invalidValue
				err := h.EnsurePostroutingRules(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.RemoteNATExternalCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context(fmt.Sprintf("If tep has an invalid Status.RemoteNATPodCIDR != %s",
			consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.RemoteNATPodCIDR = invalidValue
				err := h.EnsurePostroutingRules(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.RemoteNATPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context("Call with different parameters", func() {
			It("should remove old rules and insert updated ones", func() {
				// First call with default tep
				err := h.EnsurePostroutingRules(tep)
				Expect(err).ToNot(HaveOccurred())

				// Get inserted rules
				postRoutingRules, err := h.ListRulesInChain(getClusterPostRoutingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())

				// Edit tep
				tep.Spec.LocalNATPodCIDR = localNATPodCIDRValue

				// Second call
				err = h.EnsurePostroutingRules(tep)
				Expect(err).ToNot(HaveOccurred())

				// Get inserted rules
				newPostRoutingRules, err := h.ListRulesInChain(getClusterPostRoutingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())

				Expect(newPostRoutingRules).ToNot(ContainElements(postRoutingRules))
				Expect(newPostRoutingRules).To(ContainElements([]string{
					fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATPodCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
					fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATExternalCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
					fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATPodCIDR,
						SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
					fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATExternalCIDR,
						SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
				}))
			})
		})
		DescribeTable("EnsurePostroutingRules",
			func(editTep func(), getExpectedRules func() []string) {
				editTep()
				err := h.EnsurePostroutingRules(tep)
				Expect(err).ToNot(HaveOccurred())
				postRoutingRules, err := h.ListRulesInChain(getClusterPostRoutingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())
				expectedRules := getExpectedRules()
				Expect(postRoutingRules).To(ContainElements(expectedRules))
			},
			Entry(
				fmt.Sprintf("RemoteNATExternalCIDR != %s, RemoteNATPodCIDR != %s, LocalNATPodCIDR != %s", consts.DefaultCIDRValue,
					consts.DefaultCIDRValue, consts.DefaultCIDRValue),
				func() {},
				func() []string {
					return []string{
						fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATPodCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
						fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATExternalCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATPodCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATExternalCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
					}
				},
			),
			Entry(
				fmt.Sprintf("RemoteNATExternalCIDR != %s, RemoteNATPodCIDR != %s, LocalNATPodCIDR = %s", consts.DefaultCIDRValue,
					consts.DefaultCIDRValue, consts.DefaultCIDRValue),
				func() { tep.Spec.LocalNATPodCIDR = consts.DefaultCIDRValue },
				func() []string {
					return []string{fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATPodCIDR,
						SNAT, mustGetFirstIP(tep.Spec.LocalPodCIDR)),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATExternalCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalPodCIDR))}
				},
			),
			Entry(
				fmt.Sprintf("RemoteNATExternalCIDR != %s, RemoteNATPodCIDR = %s, LocalNATPodCIDR != %s", consts.DefaultCIDRValue,
					consts.DefaultCIDRValue, consts.DefaultCIDRValue),
				func() { tep.Spec.RemoteNATPodCIDR = consts.DefaultCIDRValue },
				func() []string {
					return []string{
						fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemotePodCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
						fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATExternalCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemotePodCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATExternalCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
					}
				},
			),
			Entry(fmt.Sprintf("RemoteNATExternalCIDR != %s, RemoteNATPodCIDR = %s, LocalNATPodCIDR = %s", consts.DefaultCIDRValue,
				consts.DefaultCIDRValue, consts.DefaultCIDRValue),
				func() {
					tep.Spec.RemoteNATPodCIDR = consts.DefaultCIDRValue
					tep.Spec.LocalNATPodCIDR = consts.DefaultCIDRValue
				},
				func() []string {
					return []string{fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemotePodCIDR,
						SNAT, mustGetFirstIP(tep.Spec.LocalPodCIDR)),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATExternalCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalPodCIDR))}
				},
			),
			Entry(
				fmt.Sprintf("RemoteNATExternalCIDR = %s, RemoteNATPodCIDR != %s, LocalNATPodCIDR != %s", consts.DefaultCIDRValue,
					consts.DefaultCIDRValue, consts.DefaultCIDRValue),
				func() { tep.Spec.RemoteNATExternalCIDR = consts.DefaultCIDRValue },
				func() []string {
					return []string{
						fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATPodCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
						fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteExternalCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATPodCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteExternalCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
					}
				},
			),
			Entry(
				fmt.Sprintf("RemoteNATExternalCIDR = %s, RemoteNATPodCIDR != %s, LocalNATPodCIDR = %s", consts.DefaultCIDRValue,
					consts.DefaultCIDRValue, consts.DefaultCIDRValue),
				func() {
					tep.Spec.LocalNATPodCIDR = consts.DefaultCIDRValue
					tep.Spec.RemoteNATExternalCIDR = consts.DefaultCIDRValue
				},
				func() []string {
					return []string{fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteNATPodCIDR,
						SNAT, mustGetFirstIP(tep.Spec.LocalPodCIDR)),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteExternalCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalPodCIDR))}
				},
			),
			Entry(
				fmt.Sprintf("RemoteNATExternalCIDR = %s, RemoteNATPodCIDR = %s, LocalNATPodCIDR != %s", consts.DefaultCIDRValue,
					consts.DefaultCIDRValue, consts.DefaultCIDRValue),
				func() {
					tep.Spec.RemoteNATPodCIDR = consts.DefaultCIDRValue
					tep.Spec.RemoteNATExternalCIDR = consts.DefaultCIDRValue
				},
				func() []string {
					return []string{
						fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemotePodCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
						fmt.Sprintf("-s %s -d %s -j %s --to %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteExternalCIDR, NETMAP, tep.Spec.LocalNATPodCIDR),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemotePodCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteExternalCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalNATPodCIDR)),
					}
				},
			),
			Entry(fmt.Sprintf("RemoteNATExternalCIDR = %s, RemoteNATPodCIDR = %s, LocalNATPodCIDR = %s", consts.DefaultCIDRValue,
				consts.DefaultCIDRValue, consts.DefaultCIDRValue),
				func() {
					tep.Spec.RemoteNATPodCIDR = consts.DefaultCIDRValue
					tep.Spec.LocalNATPodCIDR = consts.DefaultCIDRValue
					tep.Spec.RemoteNATExternalCIDR = consts.DefaultCIDRValue
				},
				func() []string {
					return []string{fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemotePodCIDR,
						SNAT, mustGetFirstIP(tep.Spec.LocalPodCIDR)),
						fmt.Sprintf("! -s %s -d %s -j %s --to-source %s", tep.Spec.LocalPodCIDR, tep.Spec.RemoteExternalCIDR,
							SNAT, mustGetFirstIP(tep.Spec.LocalPodCIDR))}
				},
			),
		)
	})
	Describe("EnsurePreroutingRulesPerTunnelEndpoint", func() {
		BeforeEach(func() {
			err := h.EnsureChainsPerCluster(clusterID1)
			Expect(err).ToNot(HaveOccurred())
			tep = validTep.DeepCopy()
			nm = &netv1alpha1.NatMapping{
				Spec: netv1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: make(netv1alpha1.Mappings),
				},
			}
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).ToNot(HaveOccurred())
		})
		Context("If tep has an empty clusterID", func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.ClusterIdentity.ClusterID = ""
				err := h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.ClusterIDLabelName, errors.StringNotEmpty)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context("If tep has an invalid LocalPodCIDR", func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.LocalPodCIDR = invalidValue
				err := h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.LocalPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context("If tep has an invalid LocalNATPodCIDR", func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.LocalNATPodCIDR = invalidValue
				err := h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.LocalNATPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context(fmt.Sprintf("If tep has Status.RemoteNATPodCIDR = %s and an invalid Spec.PodCIDR",
			consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.RemoteNATPodCIDR = consts.DefaultCIDRValue
				tep.Spec.RemotePodCIDR = invalidValue
				err := h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.PodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context(fmt.Sprintf("If tep has an invalid Status.RemoteNATPodCIDR != %s",
			consts.DefaultCIDRValue), func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.RemoteNATPodCIDR = invalidValue
				err := h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.RemoteNATPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})
		Context("Call with different parameters", func() {
			It("should remove old rules and insert updated ones", func() {
				// First call with default tep
				err := h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).ToNot(HaveOccurred())

				// Get inserted rules
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())

				// Edit tep
				tep.Spec.LocalNATPodCIDR = localNATPodCIDRValue

				// Second call
				err = h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).ToNot(HaveOccurred())

				// Get inserted rules
				newPreRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())

				Expect(newPreRoutingRules).ToNot(ContainElements(preRoutingRules))
				Expect(newPreRoutingRules).To(ContainElement(fmt.Sprintf("-s %s -d %s -j %s --to %s",
					tep.Spec.RemoteNATPodCIDR, tep.Spec.LocalNATPodCIDR, NETMAP, tep.Spec.LocalPodCIDR)))
			})
		})
		DescribeTable("EnsurePreroutingRulesPerTunnelEndpoint",
			func(editTep func(), getExpectedRules func() []string) {
				editTep()
				err := h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).ToNot(HaveOccurred())
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())
				expectedRules := getExpectedRules()
				Expect(preRoutingRules).To(ContainElements(expectedRules))
			},
			Entry(
				fmt.Sprintf("LocalNATPodCIDR != %s", consts.DefaultCIDRValue),
				func() {},
				func() []string {
					return []string{fmt.Sprintf("-s %s -d %s -j %s --to %s",
						tep.Spec.RemoteNATPodCIDR, tep.Spec.LocalNATPodCIDR, NETMAP, tep.Spec.LocalPodCIDR)}
				},
			),
			Entry(
				fmt.Sprintf("LocalNATPodCIDR = %s", consts.DefaultCIDRValue),
				func() { tep.Spec.LocalNATPodCIDR = consts.DefaultCIDRValue },
				func() []string { return []string{} },
			),
		)
	})
	Describe("EnsurePreroutingRulesPerNatMapping", func() {
		BeforeEach(func() {
			err := h.EnsureChainsPerCluster(clusterID1)
			Expect(err).ToNot(HaveOccurred())
			tep = validTep.DeepCopy()
			nm = &netv1alpha1.NatMapping{
				Spec: netv1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: natMappings,
				},
			}
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).ToNot(HaveOccurred())
		})
		Context("If nm has an empty clusterID", func() {
			It("should return a WrongParameter error", func() {
				oldValue := nm.Spec.ClusterID
				nm.Spec.ClusterID = ""
				err := h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", consts.ClusterIDLabelName, errors.StringNotEmpty)))
				nm.Spec.ClusterID = oldValue
			})
		})
		Context("Call with same mappings", func() {
			It("second call should be a nop", func() {
				// First call with default nm
				err := h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).ToNot(HaveOccurred())

				// Get inserted rules
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())

				// Second call
				err = h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).ToNot(HaveOccurred())

				// Get inserted rules
				newPreRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())

				Expect(newPreRoutingRules).To(ContainElements(preRoutingRules))
			})
		})
		Context("Call with different mappings", func() {
			It("should delete old mappings and add new ones", func() {
				oldIP := "20.0.0.2"
				newIP := "30.0.0.2"
				newMappings := netv1alpha1.Mappings{
					oldIP: newIP,
				}
				// First call with default nm
				err := h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).ToNot(HaveOccurred())

				// Get inserted rules
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())

				nm.Spec.ClusterMappings = newMappings

				// Second call
				err = h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).ToNot(HaveOccurred())

				// Get inserted rules
				newPreRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())

				Expect(newPreRoutingRules).ToNot(ContainElements(preRoutingRules))
				Expect(newPreRoutingRules).To(ContainElements(
					fmt.Sprintf("-d %s -j %s --to-destination %s", newIP, DNAT, oldIP),
				))
			})
		})
		Context("Call once", func() {
			It("should insert appropriate rules", func() {
				err := h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).ToNot(HaveOccurred())

				// Get inserted rules
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).ToNot(HaveOccurred())

				Expect(preRoutingRules).To(ContainElements(
					fmt.Sprintf("-d %s -j %s --to-destination %s", newIP1, DNAT, oldIP1),
					fmt.Sprintf("-d %s -j %s --to-destination %s", newIP2, DNAT, oldIP2),
				))
			})
		})
	})
	Describe("Utilities", func() {
		var (
			words = []string{"word0", "word1", "word2", "word3"}
		)
		DescribeTable("IPTableRule Parser",
			func(rule string, expectedIptr IPTableRule) {
				iptr, err := ParseRule(rule)
				Expect(err).ToNot(HaveOccurred())
				Expect(iptr).To(Equal(expectedIptr))
			},
			Entry(`should parse: word0`,
				words[0],
				IPTableRule{words[0]}),
			Entry(`should parse: "word0"`,
				fmt.Sprintf("%q", words[0]),
				IPTableRule{words[0]}),
			Entry(`should parse: word0 word1 word2 word3`,
				fmt.Sprintf("%s %s %s %s", words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], words[1], words[2], words[3]}),
			Entry(`should parse: "word0" "word1" "word2" "word3"`,
				fmt.Sprintf("%q %q %q %q", words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], words[1], words[2], words[3]}),
			Entry(`should parse: word0 "word1" word2 "word3"`,
				fmt.Sprintf("%s %q %s %q", words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], words[1], words[2], words[3]}),
			Entry(`should parse: "word0" word1 "word2" word3`,
				fmt.Sprintf("%q %s %q %s", words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], words[1], words[2], words[3]}),
			Entry(`should parse: "word0" "word1" word2 word3`,
				fmt.Sprintf("%q %q %s %s", words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], words[1], words[2], words[3]}),
			Entry(`should parse: word0 word1 "word2" "word3"`,
				fmt.Sprintf("%s %s %q %q", words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], words[1], words[2], words[3]}),
			Entry(`should parse: "word0" word1 word2 "word3"`,
				fmt.Sprintf("%q %s %s %q", words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], words[1], words[2], words[3]}),
			Entry(`should parse: word0 "word1" "word2" word3`,
				fmt.Sprintf("%s %q %q %s", words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], words[1], words[2], words[3]}),
			Entry(`should parse: "\'word0\'"`,
				fmt.Sprintf(`"\'%s\'"`, words[0]),
				IPTableRule{fmt.Sprintf(`'%s'`, words[0])}),
			Entry(`should parse: word0 "\'word1\'"`,
				fmt.Sprintf(`%s "\'%s\'"`, words[0], words[1]),
				IPTableRule{words[0], fmt.Sprintf(`'%s'`, words[1])}),
			Entry(`should parse: "\'word0\'" word1`,
				fmt.Sprintf(`"\'%s\'" %s`, words[0], words[1]),
				IPTableRule{fmt.Sprintf(`'%s'`, words[0]), words[1]}),
			Entry(`should parse: word0 "\'word1\'" word2`,
				fmt.Sprintf(`%s "\'%s\'" %s`, words[0], words[1], words[2]),
				IPTableRule{words[0], fmt.Sprintf(`'%s'`, words[1]), words[2]}),
			Entry(`should parse: "\'word0 word1\'"`,
				fmt.Sprintf(`"\'%s %s\'"`, words[0], words[1]),
				IPTableRule{fmt.Sprintf(`'%s'`, fmt.Sprintf("%s %s", words[0], words[1]))}),
			Entry(`should parse: word0 "\'word1 word2\'"`,
				fmt.Sprintf(`%s "\'%s %s\'"`, words[0], words[1], words[2]),
				IPTableRule{words[0], fmt.Sprintf(`'%s'`, fmt.Sprintf("%s %s", words[1], words[2]))}),
			Entry(`should parse: "\'word0 word1\'" word2`,
				fmt.Sprintf(`"\'%s %s\'" %s`, words[0], words[1], words[2]),
				IPTableRule{fmt.Sprintf(`'%s'`, fmt.Sprintf("%s %s", words[0], words[1])), words[2]}),
			Entry(`should parse: word0 "\'word1 word2\'" word3`,
				fmt.Sprintf(`%s "\'%s %s\'" %s`, words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], fmt.Sprintf(`'%s'`, fmt.Sprintf("%s %s", words[1], words[2])), words[3]}),
			Entry(`should parse: "\'word0\'" "\'word1\'"`,
				fmt.Sprintf(`"\'%s\'" "\'%s\'"`, words[0], words[1]),
				IPTableRule{fmt.Sprintf(`'%s'`, words[0]), fmt.Sprintf(`'%s'`, words[1])}),
			Entry(`should parse: word0 "\'word1\'" word2 "\'word3\'"`,
				fmt.Sprintf(`%s "\'%s\'" %s "\'%s\'"`, words[0], words[1], words[2], words[3]),
				IPTableRule{words[0], fmt.Sprintf(`'%s'`, words[1]), words[2], fmt.Sprintf(`'%s'`, words[3])}),
			Entry(`should parse: "\'word0\'" word1 "\'word2\'" word3`,
				fmt.Sprintf(`"\'%s\'" %s "\'%s\'" %s`, words[0], words[1], words[2], words[3]),
				IPTableRule{fmt.Sprintf(`'%s'`, words[0]), words[1], fmt.Sprintf(`'%s'`, words[2]), words[3]}),
		)
	})
})

func mustGetFirstIP(network string) string {
	firstIP, err := liqonetutils.GetFirstIP(network)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	return firstIP
}

// normalizeRules returns a slice of strings in which each string is a normalized representation of the rule.
func normalizeRules(rules []string) []string {
	for i, v := range rules {
		rules[i] = normalizeRuleString(v)
	}
	return rules
}

// normalizeRuleString returns a normalized string representation of the rule.
func normalizeRuleString(rule string) string {
	return strings.ReplaceAll(strings.ReplaceAll(rule, "\"", ""), "\\", "")
}

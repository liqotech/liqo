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
	"os"
	"strings"

	. "github.com/coreos/go-iptables/iptables"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

const (
	clusterID1            = "cluster1"
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
	tep         *v1alpha1.TunnelEndpoint
	nm          *v1alpha1.NatMapping
	natMappings = v1alpha1.Mappings{
		oldIP1: newIP1,
		oldIP2: newIP2,
	}
	validTep = &v1alpha1.TunnelEndpoint{
		Spec: v1alpha1.TunnelEndpointSpec{
			ClusterID:             clusterID1,
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
			Expect(err).To(BeNil())
		})
		Context("Call func", func() {
			It("should produce no errors and create Liqo chains", func() {
				err := h.Init()
				Expect(err).To(BeNil())

				// Retrieve NAT chains and Filter chains
				natChains, err := ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				// Check existence of LIQO-POSTROUTING chain
				Expect(natChains).To(ContainElement(liqonetPostroutingChain))
				// Check existence of rule in POSTROUTING
				postRoutingRules, err := h.ListRulesInChain(postroutingChain)
				Expect(err).To(BeNil())
				Expect(postRoutingRules).To(ContainElement(fmt.Sprintf("-j %s", liqonetPostroutingChain)))

				// Check existence of LIQO-PREROUTING chain
				Expect(natChains).To(ContainElement(liqonetPostroutingChain))
				// Check existence of rule in PREROUTING
				preRoutingRules, err := h.ListRulesInChain(preroutingChain)
				Expect(err).To(BeNil())
				Expect(preRoutingRules).To(ContainElement(fmt.Sprintf("-j %s", liqonetPreroutingChain)))

				// Check existence of LIQO-FORWARD chain
				Expect(filterChains).To(ContainElement(liqonetForwardingChain))
				// Check existence of rule in FORWARD
				forwardRules, err := h.ListRulesInChain(forwardChain)
				Expect(err).To(BeNil())
				Expect(forwardRules).To(ContainElement(fmt.Sprintf("-j %s", liqonetForwardingChain)))

				// Check existence of LIQO-INPUT chain
				Expect(filterChains).To(ContainElement(liqonetInputChain))
				// Check existence of rule in INPUT
				inputRules, err := h.ListRulesInChain(inputChain)
				Expect(err).To(BeNil())
				Expect(inputRules).To(ContainElement(fmt.Sprintf("-j %s", liqonetInputChain)))
			})
		})

		Context("Call func twice", func() {
			It("should produce no errors and insert all the rules", func() {
				err := h.Init()
				Expect(err).To(BeNil())

				// Check only POSTROUTING chain and rules

				// Retrieve NAT chains
				natChains, err := ipt.ListChains(natTable)
				Expect(err).To(BeNil())

				// Check existence of LIQO-POSTROUTING chain
				Expect(natChains).To(ContainElement(liqonetPostroutingChain))
				// Check existence of rule in POSTROUTING
				postRoutingRules, err := h.ListRulesInChain(postroutingChain)
				Expect(err).To(BeNil())
				Expect(postRoutingRules).To(ContainElements([]string{
					fmt.Sprintf("-j %s", liqonetPostroutingChain),
				}))
			})
		})
	})

	Describe("EnsureChainRulesPerCluster", func() {
		BeforeEach(func() {
			err := h.EnsureChainsPerCluster(clusterID1)
			Expect(err).To(BeNil())
			tep = validTep.DeepCopy()
			nm = &v1alpha1.NatMapping{
				Spec: v1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: make(v1alpha1.Mappings),
				},
			}
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).To(BeNil())
		})
		Context(fmt.Sprintf("If all parameters are valid and LocalNATPodCIDR is equal to "+
			"constant value %s in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It(`should add chain rules in POSTROUTING, INPUT, FORWARD but not in PREROUTING`, func() {
				tep.Spec.LocalNATPodCIDR = consts.DefaultCIDRValue

				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				// Check existence of rule in LIQO-POSTROUTING chain
				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				expectedRules := []string{
					fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATPodCIDR, getClusterPostRoutingChain(tep.Spec.ClusterID)),
					fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATExternalCIDR, getClusterPostRoutingChain(tep.Spec.ClusterID))}
				Expect(postRoutingRules).To(ContainElements(expectedRules))

				// Check existence of rules in LIQO-PREROUTING chain
				// Rule for NAT-ting the PodCIDR should not be present
				preRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				expectedRule := fmt.Sprintf("-s %s -d %s -j %s", tep.Spec.RemoteNATPodCIDR, tep.Spec.LocalNATExternalCIDR,
					getClusterPreRoutingMappingChain(tep.Spec.ClusterID))
				Expect(expectedRule).To(Equal(preRoutingRules[0]))

				// Check existence of rule in LIQO-FORWARD chain
				forwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				expectedRule = fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATPodCIDR, getClusterForwardChain(tep.Spec.ClusterID))
				Expect(expectedRule).To(Equal(forwardRules[0]))

				// Check existence of rule in LIQO-INPUT chain
				inputRules, err := h.ListRulesInChain(liqonetInputChain)
				expectedRule = fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATPodCIDR, getClusterInputChain(tep.Spec.ClusterID))
				Expect(expectedRule).To(Equal(inputRules[0]))

			})
		})

		Context(fmt.Sprintf("If all parameters are valid and LocalNATPodCIDR is not equal to "+
			"constant value %s in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It(`should add chain rules in POSTROUTING, INPUT, FORWARD and PREROUTING`, func() {
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				// Check existence of rule in LIQO-PREROUTING chain
				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				expectedRules := []string{
					fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATPodCIDR, getClusterPostRoutingChain(tep.Spec.ClusterID)),
					fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATExternalCIDR, getClusterPostRoutingChain(tep.Spec.ClusterID))}
				Expect(postRoutingRules).To(ContainElements(expectedRules))

				// Check existence of rule in LIQO-FORWARD chain
				forwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				expectedRule := fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATPodCIDR, getClusterForwardChain(tep.Spec.ClusterID))
				Expect(expectedRule).To(Equal(forwardRules[0]))

				// Check existence of rule in LIQO-INPUT chain
				inputRules, err := h.ListRulesInChain(liqonetInputChain)
				expectedRule = fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATPodCIDR, getClusterInputChain(tep.Spec.ClusterID))
				Expect(expectedRule).To(Equal(inputRules[0]))

				// Check existence of rule in LIQO-PREROUTING chain
				preRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				expectedRules = []string{
					fmt.Sprintf("-s %s -d %s -j %s", tep.Spec.RemoteNATPodCIDR,
						tep.Spec.LocalNATPodCIDR, getClusterPreRoutingChain(tep.Spec.ClusterID)),
					fmt.Sprintf("-s %s -d %s -j %s", tep.Spec.RemoteNATPodCIDR, tep.Spec.LocalNATExternalCIDR,
						getClusterPreRoutingMappingChain(tep.Spec.ClusterID)),
				}
				Expect(preRoutingRules).To(ContainElements(expectedRules))
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
				Expect(err).To(BeNil())
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(MatchError(fmt.Sprintf("invalid TunnelEndpoint resource: %s must be %s", consts.LocalNATPodCIDR, errors.ValidCIDR)))
				tep = validTep.DeepCopy() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})

		Context("If there are already some rules in chains but they are not in new rules", func() {
			It(`should remove existing rules that are not in the set of new rules and add new rules`, func() {
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				clusterPostRoutingChain := strings.Join([]string{liqonetPostroutingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")

				// Get rule that will be removed
				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				outdatedRule := postRoutingRules[0]

				// Modify resource
				tep.Spec.RemoteNATPodCIDR = remoteNATPodCIDRValue

				// Second call
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())
				newPostRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)

				// Check if new rules has been added.
				expectedRules := []string{
					fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATPodCIDR, clusterPostRoutingChain),
					fmt.Sprintf("-d %s -j %s", tep.Spec.RemoteNATExternalCIDR, clusterPostRoutingChain),
				}
				Expect(newPostRoutingRules).To(ContainElements(expectedRules))

				// Check if outdated rule has been removed
				Expect(newPostRoutingRules).ToNot(ContainElement(outdatedRule))
			})
		})
	})

	Describe("EnsureChainsPerCluster", func() {
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).To(BeNil())
			tep = validTep.DeepCopy()
			nm = &v1alpha1.NatMapping{
				Spec: v1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: make(v1alpha1.Mappings),
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
				Expect(err).To(BeNil())

				natChains, err := ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				// Check if filter chains have been created by function.
				Expect(filterChains).To(ContainElements(
					strings.Join([]string{liqonetForwardingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{liqonetInputClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
				))

				// Check if nat chains have been created by function.
				Expect(natChains).To(ContainElements(
					strings.Join([]string{liqonetPostroutingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{liqonetPreroutingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{liqonetPreRoutingMappingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
				))
			})
		})
		Context("If chains already exist", func() {
			It("Should be a nop", func() {
				err := h.EnsureChainsPerCluster(clusterID1)
				Expect(err).To(BeNil())

				natChains, err := ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				// Check if filter chains have been created by function.
				Expect(filterChains).To(ContainElements(
					strings.Join([]string{liqonetForwardingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{liqonetInputClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
				))

				// Check if nat chains have been created by function.
				Expect(natChains).To(ContainElements(
					strings.Join([]string{liqonetPostroutingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{liqonetPreroutingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{liqonetPreRoutingMappingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
				))

				err = h.EnsureChainsPerCluster(clusterID1)
				Expect(err).To(BeNil())

				// Get new chains and assert that they have not changed.
				newNatChains, err := ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				newFilterChains, err := ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				Expect(newNatChains).To(Equal(natChains))
				Expect(newFilterChains).To(Equal(filterChains))
			})
		})
	})

	Describe("Terminate", func() {
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).To(BeNil())
		})
		Context("If there is not a Liqo configuration", func() {
			It("should be a nop", func() {
				err := h.Terminate()
				Expect(err).To(BeNil())
			})
		})
		Context("If there is a Liqo configuration and Liqo chains are not empty", func() {
			It("should remove Liqo configuration", func() {
				err := h.Init()
				Expect(err).To(BeNil())

				// Add a remote cluster config and do not terminate it
				err = h.EnsureChainsPerCluster(clusterID1)
				Expect(err).To(BeNil())
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				err = h.Terminate()
				Expect(err).To(BeNil())

				// Check if Liqo chains do exist
				natChains, err := h.ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				filterChains, err := h.ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				Expect(natChains).ToNot(ContainElements(getLiqoChains()))
				Expect(filterChains).ToNot(ContainElements(getLiqoChains()))

				// Check if Liqo rules have been removed

				postRoutingRules, err := h.ListRulesInChain(postroutingChain)
				Expect(err).To(BeNil())
				Expect(postRoutingRules).ToNot(ContainElements([]string{
					fmt.Sprintf("-j %s", liqonetPostroutingChain),
					fmt.Sprintf("-j %s", MASQUERADE),
				}))

				preRoutingRules, err := h.ListRulesInChain(preroutingChain)
				Expect(err).To(BeNil())
				Expect(preRoutingRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetPreroutingChain)))

				forwardRules, err := h.ListRulesInChain(forwardChain)
				Expect(err).To(BeNil())
				Expect(forwardRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetForwardingChain)))

				inputRules, err := h.ListRulesInChain(inputChain)
				Expect(err).To(BeNil())
				Expect(inputRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetInputChain)))
			})
		})
		Context("If there is a Liqo configuration and Liqo chains are empty", func() {
			It("should remove Liqo configuration", func() {
				err := h.Init()
				Expect(err).To(BeNil())

				err = h.Terminate()
				Expect(err).To(BeNil())

				// Check if Liqo chains do exist
				natChains, err := h.ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				filterChains, err := h.ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				Expect(natChains).ToNot(ContainElements(getLiqoChains()))
				Expect(filterChains).ToNot(ContainElements(getLiqoChains()))

				// Check if Liqo rules have been removed

				postRoutingRules, err := h.ListRulesInChain(postroutingChain)
				Expect(err).To(BeNil())
				Expect(postRoutingRules).ToNot(ContainElements([]string{
					fmt.Sprintf("-j %s", liqonetPostroutingChain),
				}))

				preRoutingRules, err := h.ListRulesInChain(preroutingChain)
				Expect(err).To(BeNil())
				Expect(preRoutingRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetPreroutingChain)))

				forwardRules, err := h.ListRulesInChain(forwardChain)
				Expect(err).To(BeNil())
				Expect(forwardRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetForwardingChain)))

				inputRules, err := h.ListRulesInChain(inputChain)
				Expect(err).To(BeNil())
				Expect(inputRules).ToNot(ContainElement(fmt.Sprintf("-j %s", liqonetInputChain)))
			})
		})
	})

	Describe("RemoveIPTablesConfigurationPerCluster", func() {
		Context("If there are no iptables rules/chains related to remote cluster", func() {
			It("should be a nop", func() {
				// Read current configuration in order to compare it
				// with the configuration after func call.
				natChains, err := ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).To(BeNil())
				forwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				Expect(err).To(BeNil())
				inputRules, err := h.ListRulesInChain(liqonetInputChain)
				Expect(err).To(BeNil())
				preRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).To(BeNil())

				err = h.RemoveIPTablesConfigurationPerCluster(tep)
				Expect(err).To(BeNil())

				newNatChains, err := ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				newFilterChains, err := ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				newPostRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).To(BeNil())
				newForwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				Expect(err).To(BeNil())
				newInputRules, err := h.ListRulesInChain(liqonetInputChain)
				Expect(err).To(BeNil())
				newPreRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).To(BeNil())

				// Assert configs are equal
				Expect(natChains).To(Equal(newNatChains))
				Expect(filterChains).To(Equal(newFilterChains))
				Expect(postRoutingRules).To(Equal(newPostRoutingRules))
				Expect(preRoutingRules).To(Equal(newPreRoutingRules))
				Expect(forwardRules).To(Equal(newForwardRules))
				Expect(inputRules).To(Equal(newInputRules))
			})
		})
		Context("If cluster has an iptables configuration", func() {
			It("should delete chains and rules per cluster", func() {
				err := h.EnsureChainsPerCluster(tep.Spec.ClusterID)
				Expect(err).To(BeNil())

				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				// Get chains related to cluster
				natChains, err := ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				// If chain contains clusterID, then it is related to cluster
				natChainsPerCluster := getSliceContainingString(natChains, tep.Spec.ClusterID)
				filterChainsPerCluster := getSliceContainingString(filterChains, tep.Spec.ClusterID)

				// Get rules related to cluster in each chain
				postRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).To(BeNil())
				forwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				Expect(err).To(BeNil())
				inputRules, err := h.ListRulesInChain(liqonetInputChain)
				Expect(err).To(BeNil())
				preRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).To(BeNil())

				clusterPostRoutingRules := getSliceContainingString(postRoutingRules, tep.Spec.ClusterID)
				clusterPreRoutingRules := getSliceContainingString(preRoutingRules, tep.Spec.ClusterID)
				clusterForwardRules := getSliceContainingString(forwardRules, tep.Spec.ClusterID)
				clusterInputRules := getSliceContainingString(inputRules, tep.Spec.ClusterID)

				err = h.RemoveIPTablesConfigurationPerCluster(tep)
				Expect(err).To(BeNil())

				// Read config after call
				newNatChains, err := ipt.ListChains(natTable)
				Expect(err).To(BeNil())
				newFilterChains, err := ipt.ListChains(filterTable)
				Expect(err).To(BeNil())

				newPostRoutingRules, err := h.ListRulesInChain(liqonetPostroutingChain)
				Expect(err).To(BeNil())
				newForwardRules, err := h.ListRulesInChain(liqonetForwardingChain)
				Expect(err).To(BeNil())
				newInputRules, err := h.ListRulesInChain(liqonetInputChain)
				Expect(err).To(BeNil())
				newPreRoutingRules, err := h.ListRulesInChain(liqonetPreroutingChain)
				Expect(err).To(BeNil())

				// Check chains have been removed
				Expect(newNatChains).ToNot(ContainElements(natChainsPerCluster))
				Expect(newFilterChains).ToNot(ContainElements(filterChainsPerCluster))

				// Check rules have been removed
				Expect(newPostRoutingRules).ToNot(ContainElements(clusterPostRoutingRules))
				Expect(newForwardRules).ToNot(ContainElements(clusterForwardRules))
				Expect(newInputRules).ToNot(ContainElements(clusterInputRules))
				Expect(newPreRoutingRules).ToNot(ContainElements(clusterPreRoutingRules))
			})
		})
	})

	Describe("EnsurePostroutingRules", func() {
		BeforeEach(func() {
			err := h.EnsureChainsPerCluster(clusterID1)
			Expect(err).To(BeNil())
			tep = validTep.DeepCopy()
			nm = &v1alpha1.NatMapping{
				Spec: v1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: make(v1alpha1.Mappings),
				},
			}
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).To(BeNil())
		})
		Context("If tep has an empty clusterID", func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.ClusterID = ""
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
				Expect(err).To(BeNil())

				// Get inserted rules
				postRoutingRules, err := h.ListRulesInChain(getClusterPostRoutingChain(clusterID1))
				Expect(err).To(BeNil())

				// Edit tep
				tep.Spec.LocalNATPodCIDR = localNATPodCIDRValue

				// Second call
				err = h.EnsurePostroutingRules(tep)
				Expect(err).To(BeNil())

				// Get inserted rules
				newPostRoutingRules, err := h.ListRulesInChain(getClusterPostRoutingChain(clusterID1))
				Expect(err).To(BeNil())

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
				Expect(err).To(BeNil())
				postRoutingRules, err := h.ListRulesInChain(getClusterPostRoutingChain(clusterID1))
				Expect(err).To(BeNil())
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
			Expect(err).To(BeNil())
			tep = validTep.DeepCopy()
			nm = &v1alpha1.NatMapping{
				Spec: v1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: make(v1alpha1.Mappings),
				},
			}
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).To(BeNil())
		})
		Context("If tep has an empty clusterID", func() {
			It("should return a WrongParameter error", func() {
				tep.Spec.ClusterID = ""
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
				Expect(err).To(BeNil())

				// Get inserted rules
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingChain(clusterID1))
				Expect(err).To(BeNil())

				// Edit tep
				tep.Spec.LocalNATPodCIDR = localNATPodCIDRValue

				// Second call
				err = h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).To(BeNil())

				// Get inserted rules
				newPreRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingChain(clusterID1))
				Expect(err).To(BeNil())

				Expect(newPreRoutingRules).ToNot(ContainElements(preRoutingRules))
				Expect(newPreRoutingRules).To(ContainElement(fmt.Sprintf("-s %s -d %s -j %s --to %s",
					tep.Spec.RemoteNATPodCIDR, tep.Spec.LocalNATPodCIDR, NETMAP, tep.Spec.LocalPodCIDR)))
			})
		})
		DescribeTable("EnsurePreroutingRulesPerTunnelEndpoint",
			func(editTep func(), getExpectedRules func() []string) {
				editTep()
				err := h.EnsurePreroutingRulesPerTunnelEndpoint(tep)
				Expect(err).To(BeNil())
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingChain(clusterID1))
				Expect(err).To(BeNil())
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
			Expect(err).To(BeNil())
			tep = validTep.DeepCopy()
			nm = &v1alpha1.NatMapping{
				Spec: v1alpha1.NatMappingSpec{
					ClusterID:       clusterID1,
					ClusterMappings: natMappings,
				},
			}
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).To(BeNil())
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
				Expect(err).To(BeNil())

				// Get inserted rules
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).To(BeNil())

				// Second call
				err = h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).To(BeNil())

				// Get inserted rules
				newPreRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).To(BeNil())

				Expect(newPreRoutingRules).To(ContainElements(preRoutingRules))
			})
		})
		Context("Call with different mappings", func() {
			It("should delete old mappings and add new ones", func() {
				oldIP := "20.0.0.2"
				newIP := "30.0.0.2"
				newMappings := v1alpha1.Mappings{
					oldIP: newIP,
				}
				// First call with default nm
				err := h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).To(BeNil())

				// Get inserted rules
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).To(BeNil())

				nm.Spec.ClusterMappings = newMappings

				// Second call
				err = h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).To(BeNil())

				// Get inserted rules
				newPreRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).To(BeNil())

				Expect(newPreRoutingRules).ToNot(ContainElements(preRoutingRules))
				Expect(newPreRoutingRules).To(ContainElements(
					fmt.Sprintf("-d %s -j %s --to-destination %s", newIP, DNAT, oldIP),
				))
			})
		})
		Context("Call once", func() {
			It("should insert appropriate rules", func() {
				err := h.EnsurePreroutingRulesPerNatMapping(nm)
				Expect(err).To(BeNil())

				// Get inserted rules
				preRoutingRules, err := h.ListRulesInChain(getClusterPreRoutingMappingChain(clusterID1))
				Expect(err).To(BeNil())

				Expect(preRoutingRules).To(ContainElements(
					fmt.Sprintf("-d %s -j %s --to-destination %s", newIP1, DNAT, oldIP1),
					fmt.Sprintf("-d %s -j %s --to-destination %s", newIP2, DNAT, oldIP2),
				))
			})
		})
	})
})

func mustGetFirstIP(network string) string {
	firstIP, err := utils.GetFirstIP(network)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	return firstIP
}

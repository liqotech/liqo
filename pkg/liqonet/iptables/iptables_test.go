package iptables

import (
	"fmt"
	"strings"

	. "github.com/coreos/go-iptables/iptables"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"

	"github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/errors"
)

const (
	ifaceName              = "eth0"
	clusterID1             = "cluster1"
	invalidValue           = "an invalid value"
	remoteNATPodCIDRValue1 = "10.60.0.0/24"
	remoteNATPodCIDRValue2 = "10.70.0.0/24"
	longInterfaceName      = "a very long interface name"
)

var h IPTHandler
var ipt *IPTables
var tep *v1alpha1.TunnelEndpoint

var _ = Describe("iptables", func() {
	BeforeEach(func() {
		var err error
		h, err = NewIPTHandler()
		Expect(err).To(BeNil())
		ipt, err = New()
		Expect(err).To(BeNil())
		tep = forgeValidTEPResource()
	})
	Describe("Init", func() {
		Context("Call func with a valid parameter", func() {
			It("should produce no errors and create Liqo chains", func() {
				err := h.Init(ifaceName)
				Expect(err).To(BeNil())

				// Retrieve NAT chains and Filter chains
				natChains, err := ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(consts.FilterTable)
				Expect(err).To(BeNil())

				// Check existence of LIQO-POSTROUTING chain
				Expect(natChains).To(ContainElement(consts.LiqonetPostroutingChain))
				// Check existence of rule in POSTROUTING
				postRoutingRules, err := h.ListRulesInChain(consts.PostroutingChain)
				Expect(err).To(BeNil())
				Expect(postRoutingRules).To(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetPostroutingChain)))

				// Check existence of LIQO-PREROUTING chain
				Expect(natChains).To(ContainElement(consts.LiqonetPostroutingChain))
				// Check existence of rule in PREROUTING
				preRoutingRules, err := h.ListRulesInChain(consts.PreroutingChain)
				Expect(err).To(BeNil())
				Expect(preRoutingRules).To(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetPreroutingChain)))

				// Check existence of LIQO-FORWARD chain
				Expect(filterChains).To(ContainElement(consts.LiqonetForwardingChain))
				// Check existence of rule in FORWARD
				forwardRules, err := h.ListRulesInChain(consts.ForwardChain)
				Expect(err).To(BeNil())
				Expect(forwardRules).To(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetForwardingChain)))

				// Check existence of LIQO-INPUT chain
				Expect(filterChains).To(ContainElement(consts.LiqonetInputChain))
				// Check existence of rule in INPUT
				inputRules, err := h.ListRulesInChain(consts.InputChain)
				Expect(err).To(BeNil())
				Expect(inputRules).To(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetInputChain)))
			})
		})

		Context(fmt.Sprintf("Call func with string longer than %d", unix.IFNAMSIZ), func() {
			It("should produce an error", func() {
				err := h.Init(longInterfaceName)
				Expect(err).To(MatchError(fmt.Sprintf("a very long interface name must be %s%d", errors.MinorOrEqual, unix.IFNAMSIZ)))
			})
		})

		Context("Call func with empty string as parameter", func() {
			It("should produce an error", func() {
				err := h.Init("")
				Expect(err).To(MatchError(fmt.Sprintf("defaultIfaceName must be %s", errors.StringNotEmpty)))
			})
		})

		Context("Call func twice", func() {
			It("should produce no errors and insert all the rules", func() {
				err := h.Init(ifaceName)
				Expect(err).To(BeNil())

				// Check only POSTROUTING chain and rules

				// Retrieve NAT chains
				natChains, err := ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())

				// Check existence of LIQO-POSTROUTING chain
				Expect(natChains).To(ContainElement(consts.LiqonetPostroutingChain))
				// Check existence of rule in POSTROUTING
				postRoutingRules, err := h.ListRulesInChain(consts.PostroutingChain)
				Expect(err).To(BeNil())
				Expect(postRoutingRules).To(ContainElements([]string{
					fmt.Sprintf("-j %s", consts.LiqonetPostroutingChain),
				}))
			})
		})
	})

	Describe("EnsureChainRulesPerCluster", func() {
		BeforeEach(func() {
			err := h.Init(ifaceName)
			Expect(err).To(BeNil())
			err = h.EnsureChainsPerCluster(clusterID1)
			Expect(err).To(BeNil())
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).To(BeNil())
			err = h.Terminate(ifaceName)
			Expect(err).To(BeNil())
		})
		Context(fmt.Sprintf("If all parameters are valid and LocalNATPodCIDR is equal to "+
			"constant value %s in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It(`should add chain rules in POSTROUTING, INPUT, FORWARD but not in PREROUTING`, func() {
				tep.Status.LocalNATPodCIDR = consts.DefaultCIDRValue

				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				clusterPostRoutingChain := strings.Join([]string{consts.LiqonetPostroutingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterForwardChain := strings.Join([]string{consts.LiqonetForwardingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterInputChain := strings.Join([]string{consts.LiqonetInputClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")

				// Check existence of rule in LIQO-POSTROUTING chain
				postRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)
				expectedRule := fmt.Sprintf("-d %s -j %s", tep.Status.RemoteNATPodCIDR, clusterPostRoutingChain)
				Expect(expectedRule).To(Equal(postRoutingRules[0]))

				// Check existence of rule in LIQO-PREROUTING chain (should not exist)
				preRoutingRules, err := h.ListRulesInChain(consts.LiqonetPreroutingChain)
				Expect(preRoutingRules).To(HaveLen(0))

				// Check existence of rule in LIQO-FORWARD chain
				forwardRules, err := h.ListRulesInChain(consts.LiqonetForwardingChain)
				expectedRule = fmt.Sprintf("-d %s -j %s", tep.Status.RemoteNATPodCIDR, clusterForwardChain)
				Expect(expectedRule).To(Equal(forwardRules[0]))

				// Check existence of rule in LIQO-INPUT chain
				inputRules, err := h.ListRulesInChain(consts.LiqonetInputChain)
				expectedRule = fmt.Sprintf("-d %s -j %s", tep.Status.RemoteNATPodCIDR, clusterInputChain)
				Expect(expectedRule).To(Equal(inputRules[0]))

			})
		})

		Context(fmt.Sprintf("If all parameters are valid and LocalNATPodCIDR is not equal to "+
			"constant value %s in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It(`should add chain rules in POSTROUTING, INPUT, FORWARD and PREROUTING`, func() {
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				clusterPostRoutingChain := strings.Join([]string{consts.LiqonetPostroutingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterForwardChain := strings.Join([]string{consts.LiqonetForwardingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterInputChain := strings.Join([]string{consts.LiqonetInputClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterPreRoutingChain := strings.Join([]string{consts.LiqonetPreroutingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")

				// Check existence of rule in LIQO-PREROUTING chain
				postRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)
				expectedRule := fmt.Sprintf("-d %s -j %s", tep.Status.RemoteNATPodCIDR, clusterPostRoutingChain)
				Expect(expectedRule).To(Equal(postRoutingRules[0]))

				// Check existence of rule in LIQO-FORWARD chain
				forwardRules, err := h.ListRulesInChain(consts.LiqonetForwardingChain)
				expectedRule = fmt.Sprintf("-d %s -j %s", tep.Status.RemoteNATPodCIDR, clusterForwardChain)
				Expect(expectedRule).To(Equal(forwardRules[0]))

				// Check existence of rule in LIQO-INPUT chain
				inputRules, err := h.ListRulesInChain(consts.LiqonetInputChain)
				expectedRule = fmt.Sprintf("-d %s -j %s", tep.Status.RemoteNATPodCIDR, clusterInputChain)
				Expect(expectedRule).To(Equal(inputRules[0]))

				// Check existence of rule in LIQO-PREROUTING chain
				preRoutingRules, err := h.ListRulesInChain(consts.LiqonetPreroutingChain)
				expectedRule = fmt.Sprintf("-s %s -d %s -j %s", tep.Status.RemoteNATPodCIDR,
					tep.Status.LocalNATPodCIDR, clusterPreRoutingChain)
				Expect(expectedRule).To(Equal(preRoutingRules[0]))
			})
		})

		Context(fmt.Sprintf("If RemoteNATPodCIDR is different from constant value %s "+
			"and is not a valid network in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It("should return 'network/host not found' error", func() {
				tep.Status.RemoteNATPodCIDR = invalidValue
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
				tep = forgeValidTEPResource()
			})
		})

		Context(fmt.Sprintf("If RemoteNATPodCIDR is equal to constant value %s "+
			"and RemotePodCIDR is not a valid network in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It("should return 'network/host not found' error", func() {
				tep.Status.RemoteNATPodCIDR = consts.DefaultCIDRValue
				tep.Spec.PodCIDR = invalidValue
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
				tep = forgeValidTEPResource() // Otherwise RemoveIPTablesConfigurationPerCluster would fail in AfterEach
			})
		})

		Context(fmt.Sprintf("If LocalNATPodCIDR is not equal to constant value %s "+
			"and is not a valid network in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It("should return 'network/host not found' error", func() {
				tep.Status.LocalNATPodCIDR = invalidValue
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
				tep = forgeValidTEPResource()
			})
		})

		Context(fmt.Sprintf("If LocalNATPodCIDR is not equal to constant value %s "+
			"and is not a valid network in TunnelEndpoint resource", consts.DefaultCIDRValue), func() {
			It("should return 'network/host not found' error", func() {
				tep.Status.LocalNATPodCIDR = invalidValue
				err := h.Init(ifaceName)
				Expect(err).To(BeNil())
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
				tep = forgeValidTEPResource()
			})
		})

		Context("If there are already some rules in chains but they are not in new rules", func() {
			It(`should remove existing rules that are not in the set of new rules and add new rules`, func() {
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				clusterPostRoutingChain := strings.Join([]string{consts.LiqonetPostroutingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")

				// Get rule that will be removed
				postRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)
				outdatedRule := postRoutingRules[0]

				// Modify resource
				tep.Status.RemoteNATPodCIDR = remoteNATPodCIDRValue2

				// Second call
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())
				newPostRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)

				// Check if new rule has been added.
				expectedRule := fmt.Sprintf("-d %s -j %s", tep.Status.RemoteNATPodCIDR, clusterPostRoutingChain)
				Expect(expectedRule).To(Equal(newPostRoutingRules[0]))

				// Check if outdated rule has been removed
				Expect(newPostRoutingRules).ToNot(ContainElement(outdatedRule))
			})
		})

		Context("If there are already some rules in chains but they are not in new rules", func() {
			It(`should remove existing rules that are not in the set of new rules and add new rules`, func() {
				err := h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				clusterPostRoutingChain := strings.Join([]string{consts.LiqonetPostroutingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")

				// Get rule that will be removed
				postRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)
				outdatedRule := postRoutingRules[0]

				// Modify resource
				tep.Status.RemoteNATPodCIDR = remoteNATPodCIDRValue2

				// Second call
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())
				newPostRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)

				// Check if new rule has been added.
				expectedRule := fmt.Sprintf("-d %s -j %s", tep.Status.RemoteNATPodCIDR, clusterPostRoutingChain)
				Expect(expectedRule).To(Equal(newPostRoutingRules[0]))

				// Check if outdated rule has been removed
				Expect(newPostRoutingRules).ToNot(ContainElement(outdatedRule))
			})
		})
	})

	Describe("EnsureChainsPerCluster", func() {
		BeforeEach(func() {
			err := h.Init(ifaceName)
			Expect(err).To(BeNil())
			tep = forgeValidTEPResource()
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(tep)
			Expect(err).To(BeNil())
			err = h.Terminate(ifaceName)
			Expect(err).To(BeNil())
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

				natChains, err := ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(consts.FilterTable)
				Expect(err).To(BeNil())

				// Check if filter chains have been created by function.
				Expect(filterChains).To(ContainElements(
					strings.Join([]string{consts.LiqonetForwardingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{consts.LiqonetInputClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
				))

				// Check if nat chains have been created by function.
				Expect(natChains).To(ContainElements(
					strings.Join([]string{consts.LiqonetPostroutingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{consts.LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
				))
			})
		})
		Context("If chains already exist", func() {
			It("Should be a nop", func() {
				err := h.EnsureChainsPerCluster(clusterID1)
				Expect(err).To(BeNil())

				natChains, err := ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(consts.FilterTable)
				Expect(err).To(BeNil())

				// Check if filter chains have been created by function.
				Expect(filterChains).To(ContainElements(
					strings.Join([]string{consts.LiqonetForwardingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{consts.LiqonetInputClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
				))

				// Check if nat chains have been created by function.
				Expect(natChains).To(ContainElements(
					strings.Join([]string{consts.LiqonetPostroutingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
					strings.Join([]string{consts.LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID1, "-")[0]}, ""),
				))

				err = h.EnsureChainsPerCluster(clusterID1)
				Expect(err).To(BeNil())

				// Get new chains and assert that they have not changed.
				newNatChains, err := ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				newFilterChains, err := ipt.ListChains(consts.FilterTable)
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
		Context("Passing an invalid interface name", func() {
			It("should return a WrongParameterError", func() {
				err := h.Terminate(longInterfaceName)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s%d", longInterfaceName, errors.MinorOrEqual, unix.IFNAMSIZ)))
			})
		})
		Context("Passing an empty interface name", func() {
			It("should return a WrongParameterError", func() {
				err := h.Terminate("")
				Expect(err).To(MatchError(fmt.Sprintf("defaultIfaceName must be %s", errors.StringNotEmpty)))
			})
		})
		Context("If there is not a Liqo configuration", func() {
			It("should be a nop", func() {
				err := h.Terminate(ifaceName)
				Expect(err).To(BeNil())
			})
		})
		Context("If there is a Liqo configuration and Liqo chains are not empty", func() {
			It("should remove Liqo configuration", func() {
				err := h.Init(ifaceName)
				Expect(err).To(BeNil())

				// Add a remote cluster config and do not terminate it
				err = h.EnsureChainsPerCluster(clusterID1)
				Expect(err).To(BeNil())
				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				err = h.Terminate(ifaceName)
				Expect(err).To(BeNil())

				// Check if Liqo chains do exist
				natChains, err := h.ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				filterChains, err := h.ipt.ListChains(consts.FilterTable)
				Expect(err).To(BeNil())

				Expect(natChains).ToNot(ContainElements(getLiqoChains()))
				Expect(filterChains).ToNot(ContainElements(getLiqoChains()))

				// Check if Liqo rules have been removed

				postRoutingRules, err := h.ListRulesInChain(consts.PostroutingChain)
				Expect(err).To(BeNil())
				Expect(postRoutingRules).ToNot(ContainElements([]string{
					fmt.Sprintf("-j %s", consts.LiqonetPostroutingChain),
				}))

				preRoutingRules, err := h.ListRulesInChain(consts.PreroutingChain)
				Expect(err).To(BeNil())
				Expect(preRoutingRules).ToNot(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetPreroutingChain)))

				forwardRules, err := h.ListRulesInChain(consts.ForwardChain)
				Expect(err).To(BeNil())
				Expect(forwardRules).ToNot(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetForwardingChain)))

				inputRules, err := h.ListRulesInChain(consts.InputChain)
				Expect(err).To(BeNil())
				Expect(inputRules).ToNot(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetInputChain)))
			})
		})
		Context("If there is a Liqo configuration and Liqo chains are empty", func() {
			It("should remove Liqo configuration", func() {
				err := h.Init(ifaceName)
				Expect(err).To(BeNil())

				err = h.Terminate(ifaceName)
				Expect(err).To(BeNil())

				// Check if Liqo chains do exist
				natChains, err := h.ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				filterChains, err := h.ipt.ListChains(consts.FilterTable)
				Expect(err).To(BeNil())

				Expect(natChains).ToNot(ContainElements(getLiqoChains()))
				Expect(filterChains).ToNot(ContainElements(getLiqoChains()))

				// Check if Liqo rules have been removed

				postRoutingRules, err := h.ListRulesInChain(consts.PostroutingChain)
				Expect(err).To(BeNil())
				Expect(postRoutingRules).ToNot(ContainElements([]string{
					fmt.Sprintf("-j %s", consts.LiqonetPostroutingChain),
				}))

				preRoutingRules, err := h.ListRulesInChain(consts.PreroutingChain)
				Expect(err).To(BeNil())
				Expect(preRoutingRules).ToNot(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetPreroutingChain)))

				forwardRules, err := h.ListRulesInChain(consts.ForwardChain)
				Expect(err).To(BeNil())
				Expect(forwardRules).ToNot(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetForwardingChain)))

				inputRules, err := h.ListRulesInChain(consts.InputChain)
				Expect(err).To(BeNil())
				Expect(inputRules).ToNot(ContainElement(fmt.Sprintf("-j %s", consts.LiqonetInputChain)))
			})
		})
	})

	Describe("RemoveIPTablesConfigurationPerCluster", func() {
		BeforeEach(func() {
			err := h.Init(ifaceName)
			Expect(err).To(BeNil())
		})
		AfterEach(func() {
			err := h.Terminate(ifaceName)
			Expect(err).To(BeNil())
		})
		Context("If there are no iptables rules/chains related to remote cluster", func() {
			It("should be a nop", func() {
				// Read current configuration in order to compare it
				// with the configuration after func call.
				natChains, err := ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(consts.FilterTable)
				Expect(err).To(BeNil())

				postRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)
				Expect(err).To(BeNil())
				forwardRules, err := h.ListRulesInChain(consts.LiqonetForwardingChain)
				Expect(err).To(BeNil())
				inputRules, err := h.ListRulesInChain(consts.LiqonetInputChain)
				Expect(err).To(BeNil())
				preRoutingRules, err := h.ListRulesInChain(consts.LiqonetPreroutingChain)
				Expect(err).To(BeNil())

				err = h.RemoveIPTablesConfigurationPerCluster(tep)
				Expect(err).To(BeNil())

				newNatChains, err := ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				newFilterChains, err := ipt.ListChains(consts.FilterTable)
				Expect(err).To(BeNil())

				newPostRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)
				Expect(err).To(BeNil())
				newForwardRules, err := h.ListRulesInChain(consts.LiqonetForwardingChain)
				Expect(err).To(BeNil())
				newInputRules, err := h.ListRulesInChain(consts.LiqonetInputChain)
				Expect(err).To(BeNil())
				newPreRoutingRules, err := h.ListRulesInChain(consts.LiqonetPreroutingChain)
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
				// Init configuration
				tep := forgeValidTEPResource()

				err := h.EnsureChainsPerCluster(tep.Spec.ClusterID)
				Expect(err).To(BeNil())

				err = h.EnsureChainRulesPerCluster(tep)
				Expect(err).To(BeNil())

				// Get chains related to cluster
				natChains, err := ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(consts.FilterTable)
				Expect(err).To(BeNil())

				// If chain contains clusterID, then it is related to cluster
				natChainsPerCluster := getSliceContainingString(natChains, tep.Spec.ClusterID)
				filterChainsPerCluster := getSliceContainingString(filterChains, tep.Spec.ClusterID)

				// Get rules related to cluster in each chain
				postRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)
				Expect(err).To(BeNil())
				forwardRules, err := h.ListRulesInChain(consts.LiqonetForwardingChain)
				Expect(err).To(BeNil())
				inputRules, err := h.ListRulesInChain(consts.LiqonetInputChain)
				Expect(err).To(BeNil())
				preRoutingRules, err := h.ListRulesInChain(consts.LiqonetPreroutingChain)
				Expect(err).To(BeNil())

				// Rules contains a comment with the clusterID
				clusterPostRoutingRules := getSliceContainingString(postRoutingRules, tep.Spec.ClusterID)
				clusterPreRoutingRules := getSliceContainingString(preRoutingRules, tep.Spec.ClusterID)
				clusterForwardRules := getSliceContainingString(forwardRules, tep.Spec.ClusterID)
				clusterInputRules := getSliceContainingString(inputRules, tep.Spec.ClusterID)

				err = h.RemoveIPTablesConfigurationPerCluster(tep)
				Expect(err).To(BeNil())

				// Read config after call
				newNatChains, err := ipt.ListChains(consts.NatTable)
				Expect(err).To(BeNil())
				newFilterChains, err := ipt.ListChains(consts.FilterTable)
				Expect(err).To(BeNil())

				newPostRoutingRules, err := h.ListRulesInChain(consts.LiqonetPostroutingChain)
				Expect(err).To(BeNil())
				newForwardRules, err := h.ListRulesInChain(consts.LiqonetForwardingChain)
				Expect(err).To(BeNil())
				newInputRules, err := h.ListRulesInChain(consts.LiqonetInputChain)
				Expect(err).To(BeNil())
				newPreRoutingRules, err := h.ListRulesInChain(consts.LiqonetPreroutingChain)
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
})

func forgeValidTEPResource() *v1alpha1.TunnelEndpoint {
	return &v1alpha1.TunnelEndpoint{
		Spec: v1alpha1.TunnelEndpointSpec{
			ClusterID:     clusterID1,
			PodCIDR:       "10.0.0.0/24",
			ExternalCIDR:  "10.0.1.0/24",
			EndpointIP:    "172.10.0.2",
			BackendType:   "backendType",
			BackendConfig: make(map[string]string),
		},
		Status: v1alpha1.TunnelEndpointStatus{
			LocalPodCIDR:          "192.168.0.0/24",
			LocalNATPodCIDR:       "192.168.1.0/24",
			RemoteNATPodCIDR:      remoteNATPodCIDRValue1,
			LocalExternalCIDR:     "192.168.3.0/24",
			LocalNATExternalCIDR:  "192.168.4.0/24",
			RemoteNATExternalCIDR: "192.168.5.0/24",
		},
	}
}

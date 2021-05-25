package liqonet_test

import (
	"fmt"
	"strings"

	. "github.com/coreos/go-iptables/iptables"
	"github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	. "github.com/liqotech/liqo/pkg/liqonet"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
)

const (
	ifaceName    = "eth0"
	clusterName  = "cluster1"
	invalidValue = "an invalid value"
)

var h IPTablesHandler
var ipt *IPTables

var _ = Describe("iptables", func() {
	BeforeEach(func() {
		var err error
		h, err = NewIPTablesHandler()
		Expect(err).To(BeNil())
		ipt, err = New()
		Expect(err).To(BeNil())
	})
	Describe("CreateAndEnsureIPTablesChains", func() {
		Context("Call func with a valid parameter", func() {
			It("should produce no errors and create Liqo chains", func() {
				err := h.CreateAndEnsureIPTablesChains(ifaceName)
				Expect(err).To(BeNil())

				// Retrieve NAT chains and Filter chains
				natChains, err := ipt.ListChains(NatTable)
				Expect(err).To(BeNil())
				filterChains, err := ipt.ListChains(FilterTable)
				Expect(err).To(BeNil())

				// Check existence of LIQO-POSTROUTING chain
				Expect(natChains).To(ContainElement(LiqonetPostroutingChain))
				// Check existence of rule in POSTROUTING
				postRoutingRules, err := ipt.List(NatTable, "POSTROUTING")
				Expect(err).To(BeNil())
				Expect(postRoutingRules).To(ContainElement(forgeRule("POSTROUTING", "", "", "", "", "", LiqonetPostroutingChain, "")))

				// Check existence of LIQO-PREROUTING chain
				Expect(natChains).To(ContainElement(LiqonetPostroutingChain))
				// Check existence of rule in PREROUTING
				preRoutingRules, err := ipt.List(NatTable, "PREROUTING")
				Expect(err).To(BeNil())
				Expect(preRoutingRules).To(ContainElement(forgeRule("PREROUTING", "", "", "", "", "", LiqonetPreroutingChain, "")))

				// Check existence of LIQO-FORWARD chain
				Expect(filterChains).To(ContainElement(LiqonetForwardingChain))
				// Check existence of rule in FORWARD
				forwardRules, err := ipt.List(FilterTable, "FORWARD")
				Expect(err).To(BeNil())
				Expect(forwardRules).To(ContainElement(forgeRule("FORWARD", "", "", "", "", "", LiqonetForwardingChain, "")))

				// Check existence of LIQO-INPUT chain
				Expect(filterChains).To(ContainElement(LiqonetInputChain))
				// Check existence of rule in INPUT
				inputRules, err := ipt.List(FilterTable, "INPUT")
				Expect(err).To(BeNil())
				Expect(inputRules).To(ContainElement(forgeRule("INPUT", "", "", "udp", "udp", "", LiqonetInputChain, "")))

				// Check existence of rule in POSTROUTING for udp traffic toward defaultiface
				Expect(postRoutingRules).To(ContainElement(forgeRule("POSTROUTING", "", "", "", "", ifaceName, "MASQUERADE", "")))
			})
		})

		Context(fmt.Sprintf("Call func with string longer than %d", unix.IFNAMSIZ), func() {
			It("should produce an error", func() {
				err := h.CreateAndEnsureIPTablesChains("a very long interface name")
				Expect(err).To(MatchError(fmt.Sprintf("a very long interface name must be %s%d", MinorOrEqual, unix.IFNAMSIZ)))
			})
		})

		Context("Call func with empty string as parameter", func() {
			It("should produce an error", func() {
				err := h.CreateAndEnsureIPTablesChains("")
				Expect(err).To(MatchError(fmt.Sprintf("parameter must be %s", StringNotEmpty)))
			})
		})

		Context("Call func twice", func() {
			It("should produce no errors and insert all the rules", func() {
				err := h.CreateAndEnsureIPTablesChains(ifaceName)
				Expect(err).To(BeNil())

				// Check only POSTROUTING chain and rules

				// Retrieve NAT chains
				natChains, err := ipt.ListChains(NatTable)
				Expect(err).To(BeNil())

				// Check existence of LIQO-POSTROUTING chain
				Expect(natChains).To(ContainElement(LiqonetPostroutingChain))
				// Check existence of rule in POSTROUTING
				postRoutingRules, err := ipt.List(NatTable, "POSTROUTING")
				Expect(err).To(BeNil())
				Expect(postRoutingRules).To(ContainElement(forgeRule("POSTROUTING", "", "", "", "", "", LiqonetPostroutingChain, "")))

				// Check existence of rule in POSTROUTING for udp traffic toward defaultiface
				Expect(postRoutingRules).To(ContainElement(forgeRule("POSTROUTING", "", "", "", "", ifaceName, "MASQUERADE", "")))
			})
		})
	})

	Describe("EnsureChainRulespecsPerTep", func() {
		BeforeEach(func() {
			err := h.CreateAndEnsureIPTablesChains(ifaceName)
			Expect(err).To(BeNil())
			err = h.EnsureChainsPerCluster(clusterName)
			Expect(err).To(BeNil())
		})
		AfterEach(func() {
			err := h.RemoveIPTablesConfigurationPerCluster(clusterName)
			Expect(err).To(BeNil())
		})
		Context(fmt.Sprintf(`If all parameters are valid and LocalNATPodCIDR is equal to
		constant value %s in TunnelEndpoint resource`, liqoconst.DefaultCIDRValue), func() {
			It(`should add chain rules in POSTROUTING, INPUT, FORWARD but not in PREROUTING`, func() {
				tep := forgeValidTEPResource()
				tep.Status.LocalNATPodCIDR = liqoconst.DefaultCIDRValue

				err := h.EnsureChainRulespecsPerTep(tep)
				Expect(err).To(BeNil())

				clusterPostRoutingChain := strings.Join([]string{LiqonetPostroutingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterForwardChain := strings.Join([]string{LiqonetForwardingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterInputChain := strings.Join([]string{LiqonetInputClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")

				// Check existence of rule in LIQO-PREROUTING chain
				postRoutingRules, err := ipt.List(NatTable, LiqonetPostroutingChain)
				Expect(postRoutingRules).To(ContainElement(forgeRule(LiqonetPostroutingChain, "", tep.Status.RemoteNATPodCIDR, "", "",
					"", clusterPostRoutingChain, "")))

				// Check existence of rule in LIQO-PREROUTING chain (should not exist)
				preRoutingRules, err := ipt.List(NatTable, LiqonetPreroutingChain)
				// List a table will always return at least one element which
				// is '-N <table_name>'
				Expect(preRoutingRules).To(HaveLen(1))

				// Check existence of rule in LIQO-FORWARD chain
				forwardRules, err := ipt.List(FilterTable, LiqonetForwardingChain)
				Expect(forwardRules).To(ContainElement(forgeRule(LiqonetForwardingChain, "", tep.Status.RemoteNATPodCIDR, "", "",
					"", clusterForwardChain, "")))

				// Check existence of rule in LIQO-INPUT chain
				inputRules, err := ipt.List(FilterTable, LiqonetInputChain)
				Expect(inputRules).To(ContainElement(forgeRule(LiqonetInputChain, "", tep.Status.RemoteNATPodCIDR, "", "",
					"", clusterInputChain, "")))

			})
		})

		Context(fmt.Sprintf(`If all parameters are valid and LocalNATPodCIDR is not equal to
		constant value %s in TunnelEndpoint resource`, liqoconst.DefaultCIDRValue), func() {
			It(`should add chain rules in POSTROUTING, INPUT, FORWARD and PREROUTING`, func() {
				tep := forgeValidTEPResource() // Func returns a tep with LocalNATPodCIDR != DefaultCIDRValue

				err := h.EnsureChainRulespecsPerTep(tep)
				Expect(err).To(BeNil())

				clusterPostRoutingChain := strings.Join([]string{LiqonetPostroutingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterForwardChain := strings.Join([]string{LiqonetForwardingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterInputChain := strings.Join([]string{LiqonetInputClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")
				clusterPreRoutingChain := strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(tep.Spec.ClusterID, "-")[0]}, "")

				// Check existence of rule in LIQO-PREROUTING chain
				postRoutingRules, err := ipt.List(NatTable, LiqonetPostroutingChain)
				Expect(postRoutingRules).To(ContainElement(forgeRule(LiqonetPostroutingChain, "", tep.Status.RemoteNATPodCIDR, "", "",
					"", clusterPostRoutingChain, "")))

				// Check existence of rule in LIQO-PREROUTING chain
				preRoutingRules, err := ipt.List(NatTable, LiqonetPreroutingChain)
				Expect(preRoutingRules).To(ContainElement(forgeRule(LiqonetPreroutingChain, "", tep.Status.LocalNATPodCIDR, "", "",
					"", clusterPreRoutingChain, "")))

				// Check existence of rule in LIQO-FORWARD chain
				forwardRules, err := ipt.List(FilterTable, LiqonetForwardingChain)
				Expect(forwardRules).To(ContainElement(forgeRule(LiqonetForwardingChain, "", tep.Status.RemoteNATPodCIDR, "", "",
					"", clusterForwardChain, "")))

				// Check existence of rule in LIQO-INPUT chain
				inputRules, err := ipt.List(FilterTable, LiqonetInputChain)
				Expect(inputRules).To(ContainElement(forgeRule(LiqonetInputChain, "", tep.Status.RemoteNATPodCIDR, "", "",
					"", clusterInputChain, "")))

			})
		})

		Context(fmt.Sprintf(`If RemoteNATPodCIDR is different from constant value %s 
		and is not a valid network in TunnelEndpoint resource`, liqoconst.DefaultCIDRValue), func() {
			It("should return 'network/host not found' error", func() {
				tep := forgeValidTEPResource()
				tep.Status.RemoteNATPodCIDR = invalidValue
				err := h.EnsureChainRulespecsPerTep(tep)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
			})
		})

		Context(fmt.Sprintf(`If RemoteNATPodCIDR is equal to constant value %s 
		and RemotePodCIDR is not a valid network in TunnelEndpoint resource`, liqoconst.DefaultCIDRValue), func() {
			It("should return 'network/host not found' error", func() {
				tep := forgeValidTEPResource()
				tep.Status.RemoteNATPodCIDR = liqoconst.DefaultCIDRValue
				tep.Spec.PodCIDR = invalidValue
				err := h.EnsureChainRulespecsPerTep(tep)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
			})
		})

		Context(fmt.Sprintf(`If LocalNATPodCIDR is not equal to constant value %s 
		and is not a valid network in TunnelEndpoint resource`, liqoconst.DefaultCIDRValue), func() {
			It("should return 'network/host not found' error", func() {
				tep := forgeValidTEPResource()
				tep.Status.LocalNATPodCIDR = invalidValue
				err := h.EnsureChainRulespecsPerTep(tep)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
			})
		})

		Context(fmt.Sprintf(`If LocalNATPodCIDR is not equal to constant value %s 
		and is not a valid network in TunnelEndpoint resource`, liqoconst.DefaultCIDRValue), func() {
			It("should return 'network/host not found' error", func() {
				tep := forgeValidTEPResource()
				tep.Status.LocalNATPodCIDR = invalidValue
				err := h.CreateAndEnsureIPTablesChains(ifaceName)
				Expect(err).To(BeNil())
				err = h.EnsureChainRulespecsPerTep(tep)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
			})
		})
	})

	Describe("EnsureChainRulespecsPerNp", func() {
		BeforeEach(func() {
			err := h.CreateAndEnsureIPTablesChains(ifaceName)
			Expect(err).To(BeNil())
			err = h.EnsureChainsPerCluster(clusterName)
			Expect(err).To(BeNil())
		})
		Context("If ClusterID is the empty string in NatMapping resource", func() {
			It("should return WrongParameterError", func() {
				nm := forgeValidNatMapping()
				nm.Spec.ClusterID = ""
				err := h.EnsureChainRulespecsPerNm(nm)
				Expect(err).To(MatchError(fmt.Sprintf("parameter must be %s", StringNotEmpty)))
			})
		})
		Context("If PodCIDR is an invalid CIDR in NatMapping resource", func() {
			It("should return WrongParameterError", func() {
				nm := forgeValidNatMapping()
				nm.Spec.PodCIDR = invalidValue
				err := h.EnsureChainRulespecsPerNm(nm)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
			})
		})
		Context("If ExternalCIDR is an invalid CIDR in NatMapping resource", func() {
			It("should return WrongParameterError", func() {
				nm := forgeValidNatMapping()
				nm.Spec.ExternalCIDR = invalidValue
				err := h.EnsureChainRulespecsPerNm(nm)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be a valid network CIDR", invalidValue)))
			})
		})

		Context("If parameters are all valid", func() {
			It("should insert successfully chain rules", func() {
				nm := forgeValidNatMapping()
				err := h.EnsureChainRulespecsPerNm(nm)
				Expect(err).To(BeNil())

				// Check existence of rule in LIQO-PREROUTING chain
				preRoutingRules, err := ipt.List(NatTable, LiqonetPreroutingChain)
				clusterPreRoutingChain := strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(nm.Spec.ClusterID, "-")[0]}, "")
				Expect(preRoutingRules).To(ContainElement(forgeRule(LiqonetPreroutingChain, nm.Spec.PodCIDR, nm.Spec.ExternalCIDR, "", "",
					"", clusterPreRoutingChain, "")))
			})
		})

		Context("Call func more than once with same parameters", func() {
			It("should return no errors", func() {
				nm := forgeValidNatMapping()
				err := h.EnsureChainRulespecsPerNm(nm)
				Expect(err).To(BeNil())

				err = h.EnsureChainRulespecsPerNm(nm)
				Expect(err).To(BeNil())
			})
		})

		Context("Call func with updated resource", func() {
			It("should remove outdated rules and insert update ones", func() {
				nm := forgeValidNatMapping()
				err := h.EnsureChainRulespecsPerNm(nm)
				Expect(err).To(BeNil())

				// Check existence of rule in LIQO-PREROUTING chain
				preRoutingRules, err := ipt.List(NatTable, LiqonetPreroutingChain)
				clusterPreRoutingChain := strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(nm.Spec.ClusterID, "-")[0]}, "")
				Expect(preRoutingRules).To(ContainElement(forgeRule(LiqonetPreroutingChain, nm.Spec.PodCIDR, nm.Spec.ExternalCIDR, "", "",
					"", clusterPreRoutingChain, "")))
				oldRule := preRoutingRules[len(preRoutingRules)-1]

				// Update resource
				nm.Spec.PodCIDR = "10.0.4.0/24"

				// Second invocation
				err = h.EnsureChainRulespecsPerNm(nm)
				Expect(err).To(BeNil())

				// Check existence of rule in LIQO-PREROUTING chain
				preRoutingRules, err = ipt.List(NatTable, LiqonetPreroutingChain)
				Expect(preRoutingRules).To(ContainElement(forgeRule(LiqonetPreroutingChain, nm.Spec.PodCIDR, nm.Spec.ExternalCIDR, "", "",
					"", clusterPreRoutingChain, "")))
				Expect(preRoutingRules).ToNot(ContainElement(oldRule))
			})
		})
	})
})

// Helper function that forges a rule starting from its parameters.
// If an argument is the empty string, then it will be ignored.
func forgeRule(chain, src, dst, proto, match, output, action, to string) string {
	rule := strings.Builder{}
	if chain != "" {
		rule.WriteString(fmt.Sprintf("-A %s", chain))
	}
	if src != "" {
		rule.WriteString(fmt.Sprintf(" -s %s", src))
	}
	if dst != "" {
		rule.WriteString(fmt.Sprintf(" -d %s", dst))
	}
	if proto != "" {
		rule.WriteString(fmt.Sprintf(" -p %s", proto))
	}
	if match != "" {
		rule.WriteString(fmt.Sprintf(" -m %s", match))
	}
	if output != "" {
		rule.WriteString(fmt.Sprintf(" -o %s", output))
	}
	if action != "" {
		rule.WriteString(fmt.Sprintf(" -j %s", action))
	}
	if to != "" {
		rule.WriteString(fmt.Sprintf(" --to %s", to))
	}
	return rule.String()
}

func forgeValidTEPResource() *v1alpha1.TunnelEndpoint {
	return &v1alpha1.TunnelEndpoint{
		Spec: v1alpha1.TunnelEndpointSpec{
			ClusterID:     "cluster1",
			PodCIDR:       "10.0.0.0/24",
			ExternalCIDR:  "10.0.1.0/24",
			EndpointIP:    "172.10.0.2",
			BackendType:   "backendType",
			BackendConfig: make(map[string]string),
		},
		Status: v1alpha1.TunnelEndpointStatus{
			LocalPodCIDR:          "192.168.0.0/24",
			LocalNATPodCIDR:       "192.168.1.0/24",
			RemoteNATPodCIDR:      "172.16.0.0/24",
			LocalExternalCIDR:     "192.168.3.0/24",
			LocalNATExternalCIDR:  "192.168.4.0/24",
			RemoteNATExternalCIDR: "192.168.5.0/24",
		},
	}
}

func forgeValidNatMapping() *v1alpha1.NatMapping {
	return &v1alpha1.NatMapping{
		Spec: v1alpha1.NatMappingSpec{
			ClusterID:    clusterName,
			PodCIDR:      "10.0.0.0/24",
			ExternalCIDR: "10.0.1.0/24",
			Mappings: map[string]string{
				"10.200.0.6": "10.0.1.2",
			},
		},
	}
}

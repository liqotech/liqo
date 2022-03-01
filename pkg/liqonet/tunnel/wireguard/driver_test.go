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

package wireguard

import (
	"errors"
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	outOfRangeMin  = "0"
	outOfRangeMax  = "65536"
	intoTheRange   = "55555"
	notANumber     = "notANumber"
	invalidAddress = "unexistingAddress"
)

var (
	tep *netv1alpha1.TunnelEndpoint
)

var _ = Describe("Driver", func() {
	Describe("testing getVpnPortFromTep", func() {
		JustBeforeEach(func() {
			tep = &netv1alpha1.TunnelEndpoint{
				Spec: netv1alpha1.TunnelEndpointSpec{
					BackendConfig: map[string]string{liqoconst.ListeningPort: ""},
				},
			}
		})
		Context("out of range port", func() {
			It("port < than min acceptable value", func() {
				tep.Spec.BackendConfig[liqoconst.ListeningPort] = outOfRangeMin
				port, err := getTunnelPortFromTep(tep)
				Expect(port).To(BeNumerically("==", 0))
				Expect(err).To(MatchError(fmt.Sprintf("port {%s} should be greater than {%d} and minor than {%d}",
					outOfRangeMin, liqoconst.UDPMinPort, liqoconst.UDPMaxPort)))
			})

			It("port > than max acceptable value", func() {
				tep.Spec.BackendConfig[liqoconst.ListeningPort] = outOfRangeMax
				port, err := getTunnelPortFromTep(tep)
				Expect(port).To(BeNumerically("==", 0))
				Expect(err).To(MatchError(fmt.Sprintf("port {%s} should be greater than {%d} and minor than {%d}",
					outOfRangeMax, liqoconst.UDPMinPort, liqoconst.UDPMaxPort)))
			})

			It("port is not a valid number", func() {
				tep.Spec.BackendConfig[liqoconst.ListeningPort] = notANumber
				port, err := getTunnelPortFromTep(tep)
				Expect(port).To(BeNumerically("==", 0))
				Expect(errors.Unwrap(err)).To(MatchError(&strconv.NumError{
					Func: "ParseInt",
					Num:  notANumber,
					Err:  strconv.ErrSyntax,
				}))
			})

			It("port not set at all", func() {
				delete(tep.Spec.BackendConfig, liqoconst.ListeningPort)
				port, err := getTunnelPortFromTep(tep)
				Expect(port).To(BeNumerically("==", 0))
				Expect(err).To(MatchError(fmt.Sprintf("port not found in BackendConfig map using key {%s}", liqoconst.ListeningPort)))
			})
		})

		Context("in range port", func() {
			It("port within range", func() {
				tep.Spec.BackendConfig[liqoconst.ListeningPort] = intoTheRange
				expectedPort, err := strconv.ParseInt(intoTheRange, 10, 32)
				Expect(err).To(BeNil())
				port, err := getTunnelPortFromTep(tep)
				Expect(port).To(BeNumerically("==", expectedPort))
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("testing getTunnelAddressFromTep", func() {
		JustBeforeEach(func() {
			tep = &netv1alpha1.TunnelEndpoint{
				Spec: netv1alpha1.TunnelEndpointSpec{
					EndpointIP: "",
				},
			}
		})

		Context("protocol family ipv4", func() {
			It("address is in literal format", func() {
				tep.Spec.EndpointIP = ipv4Literal
				addr, err := getTunnelAddressFromTep(tep, addressResolverMock)
				Expect(addr.IP.String()).Should(ContainSubstring(ipv4Literal))
				Expect(err).To(BeNil())
			})

			It("address is in dns format", func() {
				tep.Spec.EndpointIP = ipv4Dns
				addr, err := getTunnelAddressFromTep(tep, addressResolverMock)
				Expect(addr.IP.String()).Should(ContainSubstring(ipv4Literal))
				Expect(err).To(BeNil())
			})

			It("address could not be found", func() {
				tep.Spec.EndpointIP = invalidAddress
				addr, err := getTunnelAddressFromTep(tep, addressResolverMock)
				Expect(addr).Should(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("protocol family ipv6", func() {
			It("address is in literal format", func() {
				tep.Spec.EndpointIP = ipv6Literal
				addr, err := getTunnelAddressFromTep(tep, addressResolverMock)
				Expect(addr.IP.String()).Should(ContainSubstring(ipv6Literal))
				Expect(err).To(BeNil())
			})

			It("address is in dns format", func() {
				tep.Spec.EndpointIP = ipv6Dns
				addr, err := getTunnelAddressFromTep(tep, addressResolverMock)
				Expect(addr.IP.String()).Should(ContainSubstring(ipv6Literal))
				Expect(err).To(BeNil())
			})

			It("address could not be found", func() {
				tep.Spec.EndpointIP = invalidAddress
				addr, err := getTunnelAddressFromTep(tep, addressResolverMock)
				Expect(addr).Should(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("testing getEndpoint", func() {
			JustBeforeEach(func() {
				tep = &netv1alpha1.TunnelEndpoint{
					Spec: netv1alpha1.TunnelEndpointSpec{
						EndpointIP:    "",
						BackendConfig: map[string]string{liqoconst.ListeningPort: ""},
					},
				}
			})

			Context("valid parameters", func() {
				It("valid port and address", func() {
					tep.Spec.EndpointIP = ipv4Dns
					tep.Spec.BackendConfig[liqoconst.ListeningPort] = intoTheRange
					udpAddr, err := getEndpoint(tep, addressResolverMock)
					Expect(udpAddr).NotTo(BeNil())
					Expect(udpAddr.IP.String()).Should(ContainSubstring(ipv4Literal))
					Expect(err).To(BeNil())
				})
			})

			Context("invalid parameters", func() {
				It("invalid port and valid address", func() {
					tep.Spec.EndpointIP = ipv4Dns
					tep.Spec.BackendConfig[liqoconst.ListeningPort] = outOfRangeMax
					udpAddr, err := getEndpoint(tep, addressResolverMock)
					Expect(udpAddr).To(BeNil())
					Expect(err).To(HaveOccurred())
				})

				It("invalid address and valid port", func() {
					tep.Spec.EndpointIP = "notExisting"
					tep.Spec.BackendConfig[liqoconst.ListeningPort] = intoTheRange
					udpAddr, err := getEndpoint(tep, addressResolverMock)
					Expect(udpAddr).To(BeNil())
					Expect(err).To(HaveOccurred())
				})

				It("invalid port and invalid address", func() {
					tep.Spec.EndpointIP = invalidAddress
					tep.Spec.BackendConfig[liqoconst.ListeningPort] = outOfRangeMin
					udpAddr, err := getEndpoint(tep, addressResolverMock)
					Expect(udpAddr).To(BeNil())
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})

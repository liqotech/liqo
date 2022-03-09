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

package getters_test

import (
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

var _ = Describe("DataGetters", func() {
	var (
		clusterIP  = "10.1.1.1"
		nodeIPAddr = "192.168.0.162"
		port       = corev1.ServicePort{
			Name:     "wireguard",
			Protocol: corev1.ProtocolUDP,
			Port:     5871,
			NodePort: 32444,
		}
		loadBalancerIP   = "10.0.0.1"
		loadBalancerHost = "testingWGEndpoint"

		svcTemplate = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					liqoconst.GatewayServiceLabelKey: liqoconst.GatewayServiceLabelValue,
				},
				Annotations: map[string]string{
					liqoconst.GatewayServiceAnnotationKey: nodeIPAddr,
				},
			},
			Spec: corev1.ServiceSpec{
				Ports:     []corev1.ServicePort{port},
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: clusterIP,
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							IP:       loadBalancerIP,
							Hostname: loadBalancerHost,
						},
					},
				},
			},
		}
	)

	Describe("endpoints getters", func() {
		var (
			epIP, epPort string
			err          error
			service      *corev1.Service
			serviceType  corev1.ServiceType
		)

		BeforeEach(func() {
			service = svcTemplate.DeepCopy()
		})

		checksOnFailure := func() {
			It("should return error", func() { Expect(err).Should(HaveOccurred()) })
			It("should return empty ip address", func() { Expect(epIP).Should(BeEmpty()) })
			It("should return empty port", func() { Expect(epPort).Should(BeEmpty()) })
		}

		Context("retrieval of WireGuard endpoint from service object", func() {
			JustBeforeEach(func() {
				epIP, epPort, err = getters.RetrieveWGEPFromService(service, liqoconst.GatewayServiceAnnotationKey, port.Name)
			})

			Context("service is of type NodePort", func() {
				BeforeEach(func() { service.Spec.Type = corev1.ServiceTypeNodePort })

				Context("when ip of the node has not been added as annotation to the service", func() {
					BeforeEach(func() { delete(service.Annotations, liqoconst.GatewayServiceAnnotationKey) })
					checksOnFailure()
				})

				Context("when port with given name does not exist", func() {
					BeforeEach(func() { service.Spec.Ports = nil })
					checksOnFailure()
				})

				Context("when node port has not been set", func() {
					BeforeEach(func() { service.Spec.Ports[0].NodePort = 0 })
					checksOnFailure()
				})

				Context("when service is ready", func() {
					It("should return nil", func() { Expect(err).ShouldNot(HaveOccurred()) })
					It("should return correct ip address", func() { Expect(epIP).To(Equal(nodeIPAddr)) })
					It("should return correct port number", func() { Expect(epPort).To(Equal(strconv.FormatInt(int64(port.NodePort), 10))) })
				})
			})

			Context("service is of type LoadBalancer", func() {
				BeforeEach(func() { service.Spec.Type = corev1.ServiceTypeLoadBalancer })
				Context("when the LoadBalancer IP has not been set", func() {
					BeforeEach(func() { service.Status.LoadBalancer.Ingress = nil })
					checksOnFailure()
				})

				Context("when port with given name does not exist", func() {
					BeforeEach(func() { service.Spec.Ports = nil })
					checksOnFailure()
				})

				Context("when neither ip address nor host has been set", func() {
					BeforeEach(func() {
						service.Status.LoadBalancer.Ingress[0].Hostname = ""
						service.Status.LoadBalancer.Ingress[0].IP = ""
					})
					checksOnFailure()
				})

				Context("when only the ip address has been set", func() {
					BeforeEach(func() { service.Status.LoadBalancer.Ingress[0].Hostname = "" })
					It("should return nil", func() { Expect(err).ShouldNot(HaveOccurred()) })
					It("should return correct ip address", func() { Expect(epIP).Should(Equal(loadBalancerIP)) })
					It("should return correct port number", func() { Expect(epPort).To(Equal(strconv.FormatInt(int64(port.Port), 10))) })
				})

				Context("when only the hostname has been set", func() {
					BeforeEach(func() { service.Status.LoadBalancer.Ingress[0].IP = "" })
					It("should return nil", func() { Expect(err).ShouldNot(HaveOccurred()) })
					It("should return correct ip address", func() { Expect(epIP).Should(Equal(loadBalancerHost)) })
					It("should return correct port number", func() { Expect(epPort).To(Equal(strconv.FormatInt(int64(port.Port), 10))) })
				})
			})

			Context("service is neither of type NodePort nor LoadBalancer", func() {
				checksOnFailure()
			})
		})

		Context("retrieval of endpoint from service object", func() {
			JustBeforeEach(func() {
				epIP, epPort, err = getters.RetrieveEndpointFromService(service, serviceType, port.Name)
			})

			Context("service is of type ClusterIP", func() {
				BeforeEach(func() {
					serviceType = corev1.ServiceTypeClusterIP
				})
				Context("when port with given name does not exist", func() {
					BeforeEach(func() { service.Spec.Ports = nil })
					checksOnFailure()
				})

				Context("when the ip address has not been set", func() {
					BeforeEach(func() { service.Spec.ClusterIP = "" })
					checksOnFailure()
				})

				Context("when the port number has not been set", func() {
					BeforeEach(func() { service.Spec.Ports[0].Port = 0 })
					checksOnFailure()
				})

				Context("when service is ready", func() {
					It("should return nil", func() { Expect(err).ShouldNot(HaveOccurred()) })
					It("should return correct ip address", func() { Expect(epIP).To(Equal(clusterIP)) })
					It("should return correct port number", func() { Expect(epPort).To(Equal(strconv.FormatInt(int64(port.Port), 10))) })
				})
			})

			Context("service is of type LoadBalancer", func() {
				BeforeEach(func() { serviceType = corev1.ServiceTypeLoadBalancer })
				Context("when the LoadBalancer IP has not been set", func() {
					BeforeEach(func() { service.Status.LoadBalancer.Ingress = nil })
					checksOnFailure()
				})

				Context("when port with given name does not exist", func() {
					BeforeEach(func() { service.Spec.Ports = nil })
					checksOnFailure()
				})

				Context("when neither ip address nor host has been set", func() {
					BeforeEach(func() {
						service.Status.LoadBalancer.Ingress[0].Hostname = ""
						service.Status.LoadBalancer.Ingress[0].IP = ""
					})
					checksOnFailure()
				})

				Context("when only the ip address has been set", func() {
					BeforeEach(func() { service.Status.LoadBalancer.Ingress[0].Hostname = "" })
					It("should return nil", func() { Expect(err).ShouldNot(HaveOccurred()) })
					It("should return correct ip address", func() { Expect(epIP).Should(Equal(loadBalancerIP)) })
					It("should return correct port number", func() { Expect(epPort).To(Equal(strconv.FormatInt(int64(port.Port), 10))) })
				})

				Context("when only the hostname has been set", func() {
					BeforeEach(func() { service.Status.LoadBalancer.Ingress[0].IP = "" })
					It("should return nil", func() { Expect(err).ShouldNot(HaveOccurred()) })
					It("should return correct ip address", func() { Expect(epIP).Should(Equal(loadBalancerHost)) })
					It("should return correct port number", func() { Expect(epPort).To(Equal(strconv.FormatInt(int64(port.Port), 10))) })
				})
			})

			Context("service is of type NodePort", func() {
				BeforeEach(func() { serviceType = corev1.ServiceTypeNodePort })
				checksOnFailure()
			})

			Context("service is not of correct type", func() {
				BeforeEach(func() { serviceType = "" })
				checksOnFailure()
			})
		})

	})

	Describe("testing retrieval of WireGuard public Key from secret object", func() {
		var (
			secret     *corev1.Secret
			correctKey = "cHVibGljLWtleS1vZi10aGUtY29ycmVjdC1sZW5ndGg="
			wrongKey   = "incorrect key"
		)

		BeforeEach(func() {
			secret = &corev1.Secret{
				Data: map[string][]byte{liqoconst.PublicKey: []byte(correctKey)},
			}
		})

		Context("when key with given name does not exist", func() {
			It("should return nil", func() {
				delete(secret.Data, liqoconst.PublicKey)
				_, err := getters.RetrieveWGPubKeyFromSecret(secret, liqoconst.PublicKey)
				Expect(err).NotTo(Succeed())
			})
		})

		Context("when key is wrong format", func() {
			It("should return err", func() {
				secret.Data[liqoconst.PublicKey] = []byte(wrongKey)
				_, err := getters.RetrieveWGPubKeyFromSecret(secret, liqoconst.PublicKey)
				Expect(err).NotTo(Succeed())
			})
		})

		Context("when key exists", func() {
			It("should return nil", func() {
				key, err := getters.RetrieveWGPubKeyFromSecret(secret, liqoconst.PublicKey)
				Expect(err).To(Succeed())
				Expect(key.String()).To(Equal(correctKey))
			})
		})
	})

	Describe("retrieval of clusterID from configmap", func() {
		var (
			clusterIdentity *discoveryv1alpha1.ClusterIdentity
			err             error
			cm              *corev1.ConfigMap
		)

		BeforeEach(func() {
			cm = &corev1.ConfigMap{
				Data: map[string]string{
					liqoconst.ClusterIDConfigMapKey:   "113b9ab3-7ed8-4e00-9d81-0481b111a80d",
					liqoconst.ClusterNameConfigMapKey: "cold-cherry",
				},
			}
		})

		JustBeforeEach(func() {
			clusterIdentity, err = getters.RetrieveClusterIDFromConfigMap(cm)
		})

		Context("when cluster id is not set", func() {
			BeforeEach(func() {
				delete(cm.Data, liqoconst.ClusterIDConfigMapKey)

			})

			It("should fail", func() {
				Expect(clusterIdentity).Should(BeNil())
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when cluster name is not set", func() {
			BeforeEach(func() {
				delete(cm.Data, liqoconst.ClusterNameConfigMapKey)

			})

			It("should fail", func() {
				Expect(clusterIdentity).Should(BeNil())
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when cluster identity is set", func() {
			It("should fail", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(clusterIdentity.ClusterName).Should(Equal(cm.Data[liqoconst.ClusterNameConfigMapKey]))
				Expect(clusterIdentity.ClusterID).Should(Equal(cm.Data[liqoconst.ClusterIDConfigMapKey]))
			})
		})

	})

	Describe("retrieval of network configuration from ipamstorage", func() {
		var (
			ipamStorage  *netv1alpha1.IpamStorage
			resNets      = []string{"10.1.0.0/16", "192.168.0.0/16"}
			podCIDR      = "10.200.0.0/16"
			serviceCIDR  = "10.150.2.0/24"
			externalCIDR = "10.201.0.0/16"
			netConfig    *getters.NetworkConfig
			err          error
		)

		checkOnError := func() {
			Expect(netConfig).To(BeNil())
			Expect(err).To(HaveOccurred())
		}

		BeforeEach(func() {
			ipamStorage = &netv1alpha1.IpamStorage{
				Spec: netv1alpha1.IpamSpec{
					ReservedSubnets: resNets,
					ExternalCIDR:    externalCIDR,
					PodCIDR:         podCIDR,
					ServiceCIDR:     serviceCIDR,
				},
			}
		})

		JustBeforeEach(func() {
			netConfig, err = getters.RetrieveNetworkConfiguration(ipamStorage)
		})

		Context("when podCIDR has not been set", func() {
			BeforeEach(func() {
				ipamStorage.Spec.PodCIDR = ""
			})

			It("should return error", checkOnError)
		})

		Context("when externalCIDR has not been set", func() {
			BeforeEach(func() {
				ipamStorage.Spec.ExternalCIDR = ""
			})

			It("should return error", checkOnError)
		})

		Context("when serviceCIDR has not been set", func() {
			BeforeEach(func() {
				ipamStorage.Spec.ServiceCIDR = ""
			})

			It("should return error", checkOnError)
		})

		Context("when all fields has been set", func() {
			It("should return configuration and nil", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(netConfig).NotTo(BeNil())
				Expect(netConfig.ServiceCIDR).To(Equal(serviceCIDR))
				Expect(netConfig.ExternalCIDR).To(Equal(externalCIDR))
				Expect(netConfig.PodCIDR).To(Equal(podCIDR))
				Expect(netConfig.ReservedSubnets).To(Equal(resNets))
			})
		})

	})
})

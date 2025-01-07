// Copyright 2019-2025 The Liqo Authors
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

var _ = Describe("DataGetters", func() {
	var (
		clusterIP = "10.1.1.1"
		port      = corev1.ServicePort{
			Name:     "test",
			Protocol: corev1.ProtocolUDP,
			Port:     5871,
			NodePort: 32444,
		}
		loadBalancerIP   = "10.0.0.1"
		loadBalancerHost = "testingEndpoint"

		svcTemplate = &corev1.Service{
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

	Describe("retrieval of clusterID from configmap", func() {
		var (
			clusterID liqov1beta1.ClusterID
			err       error
			cm        *corev1.ConfigMap
		)

		BeforeEach(func() {
			cm = &corev1.ConfigMap{
				Data: map[string]string{
					liqoconst.ClusterIDConfigMapKey: "113b9ab3-7ed8-4e00-9d81-0481b111a80d",
				},
			}
		})

		JustBeforeEach(func() {
			clusterID, err = getters.RetrieveClusterIDFromConfigMap(cm)
		})

		Context("when cluster id is not set", func() {
			BeforeEach(func() {
				delete(cm.Data, liqoconst.ClusterIDConfigMapKey)

			})

			It("should fail", func() {
				Expect(clusterID).Should(BeEmpty())
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when cluster identity is set", func() {
			It("should fail", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(string(clusterID)).Should(Equal(cm.Data[liqoconst.ClusterIDConfigMapKey]))
			})
		})

	})
})

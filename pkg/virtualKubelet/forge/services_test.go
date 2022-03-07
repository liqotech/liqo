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

package forge_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Services Forging", func() {
	BeforeEach(func() { forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP) })

	Describe("the RemoteService function", func() {
		var (
			input  *corev1.Service
			output *corev1apply.ServiceApplyConfiguration
		)

		BeforeEach(func() {
			input = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name", Namespace: "original",
					Labels:      map[string]string{"foo": "bar"},
					Annotations: map[string]string{"bar": "baz"},
				},
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort},
			}

			JustBeforeEach(func() { output = forge.RemoteService(input, "reflected") })

			It("should correctly set the name and namespace", func() {
				Expect(output.Name).To(PointTo(Equal("name")))
				Expect(output.Namespace).To(PointTo(Equal("reflected")))
			})

			It("should correctly set the labels", func() {
				Expect(output.Labels).To(HaveKeyWithValue("foo", "bar"))
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
			})
			It("should correctly set the annotations", func() {
				Expect(output.Annotations).To(HaveKeyWithValue("bar", "baz"))
			})
			It("should correctly set the spec", func() {
				Expect(output.Spec.Type).To(PointTo(Equal(corev1.ServiceTypeNodePort)))
			})
		})
	})

	Describe("the RemoteServiceSpec function", func() {
		var (
			getService = func(serviceType corev1.ServiceType, clusterIP string) *corev1.ServiceSpec {
				trafpol := corev1.ServiceInternalTrafficPolicyCluster
				ipfampol := corev1.IPFamilyPolicyPreferDualStack
				return &corev1.ServiceSpec{
					Type:     serviceType,
					Selector: map[string]string{"key": "value"},
					Ports:    []corev1.ServicePort{{Name: "port"}},

					AllocateLoadBalancerNodePorts: pointer.Bool(true),
					ExternalTrafficPolicy:         corev1.ServiceExternalTrafficPolicyTypeCluster,
					InternalTrafficPolicy:         &trafpol,
					IPFamilyPolicy:                &ipfampol,
					LoadBalancerClass:             pointer.String("class"),
					LoadBalancerSourceRanges:      []string{"0.0.0.0/0"},
					PublishNotReadyAddresses:      *pointer.Bool(true),
					SessionAffinity:               corev1.ServiceAffinityNone,
					ClusterIP:                     clusterIP,
				}
			}
		)

		type remoteServiceTestcase struct {
			input               *corev1.ServiceSpec
			expectedClusterIP   OmegaMatcher
			expectedServiceType OmegaMatcher
		}

		DescribeTable("RemoteServiceSpec table", func(c remoteServiceTestcase) {
			output := forge.RemoteServiceSpec(c.input.DeepCopy(), false)

			By("should correctly replicate the core fields", func() {
				Expect(output.Type).To(PointTo(c.expectedServiceType))
				Expect(output.Selector).To(HaveKeyWithValue("key", "value"))
				Expect(output.Ports).To(HaveLen(1))
			})

			By("should correctly replicate the accessory fields", func() {
				Expect(output.AllocateLoadBalancerNodePorts).To(PointTo(BeTrue()))
				Expect(output.ExternalTrafficPolicy).To(PointTo(Equal(corev1.ServiceExternalTrafficPolicyTypeCluster)))
				Expect(output.InternalTrafficPolicy).To(PointTo(Equal(corev1.ServiceInternalTrafficPolicyCluster)))
				Expect(output.IPFamilyPolicy).To(PointTo(Equal(corev1.IPFamilyPolicyPreferDualStack)))
				Expect(output.LoadBalancerClass).To(PointTo(Equal("class")))
				Expect(output.LoadBalancerSourceRanges).To(ConsistOf("0.0.0.0/0"))
				Expect(output.PublishNotReadyAddresses).To(PointTo(BeTrue()))
				Expect(output.SessionAffinity).To(PointTo(Equal(corev1.ServiceAffinityNone)))
				Expect(output.ClusterIP).To(c.expectedClusterIP)
			})
		}, Entry("NodePort Service", remoteServiceTestcase{
			input:               getService(corev1.ServiceTypeNodePort, ""),
			expectedClusterIP:   BeNil(),
			expectedServiceType: Equal(corev1.ServiceTypeNodePort),
		}), Entry("ClusterIP Service", remoteServiceTestcase{
			input:               getService(corev1.ServiceTypeClusterIP, ""),
			expectedClusterIP:   BeNil(),
			expectedServiceType: Equal(corev1.ServiceTypeClusterIP),
		}), Entry("Headless Service", remoteServiceTestcase{
			input:               getService(corev1.ServiceTypeClusterIP, corev1.ClusterIPNone),
			expectedClusterIP:   PointTo(Equal(corev1.ClusterIPNone)),
			expectedServiceType: Equal(corev1.ServiceTypeClusterIP),
		}))
	})

	Describe("the RemoteServicePorts function", func() {
		var (
			input               corev1.ServicePort
			output              []*corev1apply.ServicePortApplyConfiguration
			forceRemoteNodePort bool
		)

		BeforeEach(func() {
			input = corev1.ServicePort{
				Name: "HTTPS", Port: 443, TargetPort: intstr.FromInt(8443), Protocol: corev1.ProtocolTCP,
			}
			forceRemoteNodePort = false
		})

		JustBeforeEach(func() { output = forge.RemoteServicePorts([]corev1.ServicePort{input, input}, forceRemoteNodePort) })

		It("should return the correct number of ports", func() { Expect(output).To(HaveLen(2)) })
		It("should correctly replicate the port fields", func() {
			Expect(output[0].Name).To(PointTo(Equal("HTTPS")))
			Expect(output[0].Port).To(PointTo(BeNumerically("==", 443)))
			Expect(output[0].TargetPort).To(PointTo(Equal(intstr.FromInt(8443))))
			Expect(output[0].Protocol).To(PointTo(Equal(corev1.ProtocolTCP)))
			Expect(output[0].NodePort).To(PointTo(BeNumerically("==", 0)))
			Expect(output[0].AppProtocol).To(BeNil())
		})

		When("a node port is specified", func() {
			BeforeEach(func() {
				input.NodePort = 33333
			})
			It("should be omitted", func() { Expect(output[0].NodePort).To(BeNil()) })
		})

		When("an app protocol is specified", func() {
			BeforeEach(func() { input.AppProtocol = pointer.String("protocol") })
			It("should be replicated", func() { Expect(output[0].AppProtocol).To(PointTo(Equal("protocol"))) })
		})

		When("force remote node port is specified", func() {
			BeforeEach(func() {
				input.NodePort = 33333
				forceRemoteNodePort = true
			})
			It("should be replicated", func() { Expect(output[0].NodePort).To(PointTo(BeNumerically("==", 33333))) })
		})
	})
})

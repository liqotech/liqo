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

package forge_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	netv1apply "k8s.io/client-go/applyconfigurations/networking/v1"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Ingresses Forging", func() {
	ForgeIngressSpec := func(ing *netv1.Ingress) *netv1.Ingress {
		ing.Spec.DefaultBackend = &netv1.IngressBackend{
			Service: &netv1.IngressServiceBackend{
				Name: "default-backend",
				Port: netv1.ServiceBackendPort{
					Number: 80,
				},
			},
		}
		ing.Spec.Rules = []netv1.IngressRule{
			{
				Host: "example.com",
				IngressRuleValue: netv1.IngressRuleValue{
					HTTP: &netv1.HTTPIngressRuleValue{
						Paths: []netv1.HTTPIngressPath{
							{
								PathType: func() *netv1.PathType {
									pt := netv1.PathTypePrefix
									return &pt
								}(),
								Path: "/",
								Backend: netv1.IngressBackend{
									Service: &netv1.IngressServiceBackend{
										Name: "example-backend",
										Port: netv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
						},
					},
				},
			},
		}
		ing.Spec.TLS = []netv1.IngressTLS{
			{
				Hosts:      []string{"example.com"},
				SecretName: "example-secret",
			},
		}

		return ing
	}

	Describe("the RemoteIngress function", func() {
		var (
			input       *netv1.Ingress
			output      *netv1apply.IngressApplyConfiguration
			forgingOpts *forge.ForgingOpts
		)

		BeforeEach(func() {
			input = &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name", Namespace: "original",
					Labels:      map[string]string{"foo": "bar"},
					Annotations: map[string]string{"bar": "baz", "kubernetes.io/ingress.class": "nginx"},
				},
			}
			forgingOpts = testutil.FakeForgingOpts()
			ForgeIngressSpec(input)
		})

		JustBeforeEach(func() { output = forge.RemoteIngress(input, "reflected", false, "", forgingOpts) })

		It("should correctly set the name and namespace", func() {
			Expect(output.Name).To(PointTo(Equal("name")))
			Expect(output.Namespace).To(PointTo(Equal("reflected")))
		})

		It("should correctly set the labels", func() {
			Expect(output.Labels).To(HaveKeyWithValue("foo", "bar"))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
			Expect(output.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
		})
		It("should correctly set the annotations", func() {
			Expect(output.Annotations).To(HaveKeyWithValue("bar", "baz"))
			Expect(output.Annotations).ToNot(HaveKey("kubernetes.io/ingress.class"))
			Expect(output.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
		})
		It("should correctly set the spec", func() {
			Expect(output.Spec.DefaultBackend).To(PointTo(Equal(netv1apply.IngressBackendApplyConfiguration{
				Service: &netv1apply.IngressServiceBackendApplyConfiguration{
					Name: pointer.String("default-backend"),
					Port: &netv1apply.ServiceBackendPortApplyConfiguration{
						Number: pointer.Int32(80),
						Name:   pointer.String(""),
					},
				},
			})))
		})
	})

	Describe("the RemoteIngressSpec function", func() {
		var (
			getIngress = func(ingressClassName *string) *netv1.IngressSpec {
				ing := netv1.Ingress{}
				ForgeIngressSpec(&ing)

				ing.Spec.IngressClassName = ingressClassName

				return &ing.Spec
			}
		)

		type remoteIngressTestcase struct {
			input *netv1.IngressSpec
		}

		DescribeTable("RemoteIngressSpec table", func(c remoteIngressTestcase) {
			output := forge.RemoteIngressSpec(c.input.DeepCopy(), false, "")

			By("should not replicate the ingress class name", func() {
				Expect(output.IngressClassName).To(BeNil())
			})
		}, Entry("Ingress with IngressClass", remoteIngressTestcase{
			input: getIngress(pointer.String("nginx")),
		}), Entry("Ingress without IngressClass", remoteIngressTestcase{
			input: getIngress(nil),
		}))
	})

	Describe("the RemoteIngressBackend function", func() {
		var (
			input  netv1.IngressBackend
			output *netv1apply.IngressBackendApplyConfiguration
		)

		BeforeEach(func() {
			input = netv1.IngressBackend{
				Service: &netv1.IngressServiceBackend{
					Name: "example-backend",
					Port: netv1.ServiceBackendPort{
						Number: 80,
					},
				},
			}
		})

		JustBeforeEach(func() { output = forge.RemoteIngressBackend(&input) })

		It("should correctly replicate the backend fields", func() {
			Expect(output.Service.Name).To(PointTo(Equal("example-backend")))
			Expect(output.Service.Port.Number).To(PointTo(BeNumerically("==", 80)))
		})
	})

	Describe("the RemoteIngressService function", func() {
		var (
			input  netv1.IngressServiceBackend
			output *netv1apply.IngressServiceBackendApplyConfiguration
		)

		BeforeEach(func() {
			input = netv1.IngressServiceBackend{
				Name: "example-backend",
				Port: netv1.ServiceBackendPort{
					Number: 80,
					Name:   "example-port",
				},
			}
		})

		JustBeforeEach(func() { output = forge.RemoteIngressService(&input) })

		It("should correctly replicate the service fields", func() {
			Expect(output.Name).To(PointTo(Equal("example-backend")))
			Expect(output.Port.Number).To(PointTo(BeNumerically("==", 80)))
			Expect(output.Port.Name).To(PointTo(Equal("example-port")))
		})
	})

	Describe("the RemoteIngressRules function", func() {
		var (
			input  netv1.IngressRule
			output []*netv1apply.IngressRuleApplyConfiguration
		)

		BeforeEach(func() {
			input = netv1.IngressRule{
				Host: "example-host",
				IngressRuleValue: netv1.IngressRuleValue{
					HTTP: &netv1.HTTPIngressRuleValue{},
				},
			}
		})

		JustBeforeEach(func() { output = forge.RemoteIngressRules([]netv1.IngressRule{input, input}) })

		It("should return the correct number of rules", func() { Expect(output).To(HaveLen(2)) })
		It("should correctly replicate the IngressRules fields", func() {
			Expect(output[0].Host).To(PointTo(Equal("example-host")))
			Expect(output[0].HTTP).ToNot(BeNil())
		})
	})

	Describe("the RemoteIngressHTTP function", func() {
		var (
			input  netv1.HTTPIngressRuleValue
			output *netv1apply.HTTPIngressRuleValueApplyConfiguration
		)

		BeforeEach(func() {
			input = netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{
					{
						Backend: netv1.IngressBackend{
							Service: &netv1.IngressServiceBackend{},
						},
					},
				},
			}
		})

		JustBeforeEach(func() { output = forge.RemoteIngressHTTP(&input) })

		It("should correctly replicate the HTTPRule fields", func() {
			Expect(output.Paths).To(HaveLen(1))
		})
	})

	Describe("the RemoteIngressPaths function", func() {
		var (
			input  netv1.HTTPIngressPath
			output []*netv1apply.HTTPIngressPathApplyConfiguration
		)

		BeforeEach(func() {
			input = netv1.HTTPIngressPath{
				Backend: netv1.IngressBackend{
					Service: &netv1.IngressServiceBackend{},
				},
				Path: "/example-path",
				PathType: func() *netv1.PathType {
					p := netv1.PathTypePrefix
					return &p
				}(),
			}
		})

		JustBeforeEach(func() { output = forge.RemoteIngressPaths([]netv1.HTTPIngressPath{input, input}) })

		It("should return the correct number of paths", func() { Expect(output).To(HaveLen(2)) })
		It("should correctly replicate the HTTPIngressPath fields", func() {
			Expect(output[0].Path).To(PointTo(Equal("/example-path")))
			Expect(output[0].PathType).To(PointTo(Equal(netv1.PathTypePrefix)))
			Expect(output[0].Backend).ToNot(BeNil())
		})
	})

	Describe("the RemoteIngressTLS function", func() {
		var (
			input  netv1.IngressTLS
			output []*netv1apply.IngressTLSApplyConfiguration
		)

		BeforeEach(func() {
			input = netv1.IngressTLS{
				Hosts:      []string{"example-host"},
				SecretName: "example-secret",
			}
		})

		JustBeforeEach(func() { output = forge.RemoteIngressTLS([]netv1.IngressTLS{input, input}) })

		It("should return the correct number of tls configs", func() { Expect(output).To(HaveLen(2)) })
		It("should correctly replicate the IngressTLS fields", func() {
			Expect(output[0].Hosts).To(HaveLen(1))
			Expect(output[0].Hosts[0]).To(Equal("example-host"))
			Expect(output[0].SecretName).To(PointTo(Equal("example-secret")))
		})
	})
})

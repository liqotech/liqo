// Copyright 2019-2024 The Liqo Authors
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

package shadowendpointslicectrl

import (
	"bytes"
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	liqodiscovery "github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("ShadowEndpointSlice Controller", func() {
	const (
		shadowEpsNamespace string = "default"
		shadowEpsName      string = "test-shadow-eps"
		testFcID           string = "test-fc-id"
	)

	var (
		req = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      shadowEpsName,
				Namespace: shadowEpsNamespace,
			},
		}
		ctx               context.Context
		res               ctrl.Result
		err               error
		buffer            *bytes.Buffer
		fakeClientBuilder *fake.ClientBuilder
		fakeClient        client.WithWatch

		testShadowEps *vkv1alpha1.ShadowEndpointSlice
		testEps       *discoveryv1.EndpointSlice
		testFc        *discoveryv1alpha1.ForeignCluster
		testConf      *networkingv1alpha1.Configuration

		newFc = func(networkReady, apiServerReady bool) *discoveryv1alpha1.ForeignCluster {
			networkStatus := discoveryv1alpha1.ConditionStatusEstablished
			if !networkReady {
				networkStatus = discoveryv1alpha1.ConditionStatusError
			}

			apiServerStatus := discoveryv1alpha1.ConditionStatusEstablished
			if !apiServerReady {
				apiServerStatus = discoveryv1alpha1.ConditionStatusError
			}

			return &discoveryv1alpha1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testFcID,
					Labels: map[string]string{
						liqodiscovery.ClusterIDLabel: testFcID,
					},
				},
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					ClusterID: discoveryv1alpha1.ClusterID(testFcID),
				},
				Status: discoveryv1alpha1.ForeignClusterStatus{
					Modules: discoveryv1alpha1.Modules{
						Networking: discoveryv1alpha1.Module{
							Enabled: true,
							Conditions: []discoveryv1alpha1.Condition{
								{
									Type:   discoveryv1alpha1.NetworkConnectionStatusCondition,
									Status: networkStatus,
								},
							},
						},
					},
					Conditions: []discoveryv1alpha1.Condition{
						{
							Type:   discoveryv1alpha1.APIServerStatusCondition,
							Status: apiServerStatus,
						},
					},
				},
			}
		}

		newShadowEps = func(endpointsReady bool) *vkv1alpha1.ShadowEndpointSlice {
			ready := ptr.To(true)
			if !endpointsReady {
				ready = ptr.To(false)
			}

			return &vkv1alpha1.ShadowEndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shadowEpsName,
					Namespace: shadowEpsNamespace,
					Labels: map[string]string{
						discoveryv1.LabelManagedBy:   forge.EndpointSliceManagedBy,
						forge.LiqoOriginClusterIDKey: testFcID,
						"label1-key":                 "label1-value",
					},
					Annotations: map[string]string{
						"annotation1-key": "annotation1-value",
					},
				},
				Spec: vkv1alpha1.ShadowEndpointSliceSpec{
					Template: vkv1alpha1.EndpointSliceTemplate{
						Endpoints: []discoveryv1.Endpoint{{
							NodeName: ptr.To(testFcID),
							Conditions: discoveryv1.EndpointConditions{
								Ready: ready,
							},
							Addresses: []string{"10.10.0.1"},
						}},
						Ports:       []discoveryv1.EndpointPort{{Name: ptr.To("HTTPS")}},
						AddressType: discoveryv1.AddressTypeFQDN,
					},
				},
			}
		}

		newEps = func() *discoveryv1.EndpointSlice {
			return &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shadowEpsName,
					Namespace: shadowEpsNamespace,
					Labels: map[string]string{
						"existing-key": "existing-value",
					},
					Annotations: map[string]string{
						"existing-key": "existing-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:               "ShadowEndpointSlice",
							Name:               shadowEpsName,
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						},
					},
				},
			}

		}

		newConfiguration = func(remapped bool) *networkingv1alpha1.Configuration {
			var remappedPodCIDR, remappedExternalCIDR networkingv1alpha1.CIDR
			if remapped {
				remappedPodCIDR = "10.30.0.0/16"
				remappedExternalCIDR = "10.40.0.0/16"
			} else {
				remappedPodCIDR = "10.10.0.0/16"
				remappedExternalCIDR = "10.20.0.0/16"
			}

			return &networkingv1alpha1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
					Labels: map[string]string{
						consts.RemoteClusterID: testFcID,
					},
				},
				Spec: networkingv1alpha1.ConfigurationSpec{
					Remote: networkingv1alpha1.ClusterConfig{
						CIDR: networkingv1alpha1.ClusterConfigCIDR{
							Pod:      "10.10.0.0/16",
							External: "10.20.0.0/16",
						},
					},
				},
				Status: networkingv1alpha1.ConfigurationStatus{
					Remote: &networkingv1alpha1.ClusterConfig{
						CIDR: networkingv1alpha1.ClusterConfigCIDR{
							Pod:      remappedPodCIDR,
							External: remappedExternalCIDR,
						},
					},
				},
			}
		}
	)

	BeforeEach(func() {
		ctx = context.TODO()
		buffer = &bytes.Buffer{}
		klog.SetOutput(buffer)

		fakeClientBuilder = fake.NewClientBuilder().WithScheme(scheme.Scheme)
	})

	JustBeforeEach(func() {
		r := &Reconciler{
			Client: fakeClient,
			Scheme: scheme.Scheme,
		}
		_, err = r.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		klog.Flush()
	})

	When("shadowendpointslice is not found", func() {
		BeforeEach(func() {
			fakeClient = fakeClientBuilder.Build()
		})

		It("should ignore it", func() {
			Expect(res).To(BeZero())
			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("shadowendpointslice %q not found", req.NamespacedName)))
		})
	})

	When("endpointslice has already been created", func() {
		BeforeEach(func() {
			testShadowEps = newShadowEps(true)
			testEps = newEps()
			testFc = newFc(true, true)
			testConf = newConfiguration(false)
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps, testEps, testFc, testConf).Build()
		})

		It("should output the correct log", func() {
			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("endpointslice %q found running, will update it", klog.KObj(testEps))))
			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("updated endpointslice %q with success", klog.KObj(testEps))))
		})

		It("should update endpointslice metadata to shadowendpointslice metadata", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.Labels).To(HaveKeyWithValue(consts.ManagedByLabelKey, consts.ManagedByShadowEndpointSliceValue))
			for key, value := range testShadowEps.Labels {
				Expect(eps.Labels).To(HaveKeyWithValue(key, value))
			}
			for key, value := range testShadowEps.Annotations {
				Expect(eps.Annotations).To(HaveKeyWithValue(key, value))
			}
		})

		It("should keep existing labels and annotations", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.Labels).To(HaveKeyWithValue("existing-key", "existing-value"))
			Expect(eps.Annotations).To(HaveKeyWithValue("existing-key", "existing-value"))
		})

		It("should replicate endpoints, addressType and ports", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.AddressType).To(Equal(testShadowEps.Spec.Template.AddressType))
			Expect(eps.Endpoints).To(Equal(testShadowEps.Spec.Template.Endpoints))
			Expect(eps.Ports).To(Equal(testShadowEps.Spec.Template.Ports))
		})

		It("should keep owner reference", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.GetOwnerReferences()).To(ContainElement(
				MatchFields(IgnoreExtras, Fields{
					"Kind": Equal("ShadowEndpointSlice"),
					"Name": Equal(shadowEpsName),
				})),
			)
		})

	})

	When("endpointslice not already created", func() {
		BeforeEach(func() {
			testShadowEps = newShadowEps(true)
			testFc = newFc(true, true)
			testConf = newConfiguration(false)
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps.DeepCopy(), testFc, testConf).Build()
		})

		It("should output the correct log", func() {
			Expect(buffer.String()).To(ContainSubstring(
				fmt.Sprintf("created endpointslice %q for shadowendpointslice %q", klog.KObj(testShadowEps), klog.KObj(testShadowEps))))
		})

		It("should create endpointslice metadata with shadowendpointslice metadata", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.Labels).To(HaveKeyWithValue(consts.ManagedByLabelKey, consts.ManagedByShadowEndpointSliceValue))
			for key, value := range testShadowEps.Labels {
				Expect(eps.Labels).To(HaveKeyWithValue(key, value))
			}
			for key, value := range testShadowEps.Annotations {
				Expect(eps.Annotations).To(HaveKeyWithValue(key, value))
			}

		})

		It("should replicate endpoints, addressType and ports", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.AddressType).To(Equal(testShadowEps.Spec.Template.AddressType))
			Expect(eps.Endpoints).To(Equal(testShadowEps.Spec.Template.Endpoints))
			Expect(eps.Ports).To(Equal(testShadowEps.Spec.Template.Ports))
		})

		It("should set owner reference", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.GetOwnerReferences()).To(ContainElement(
				MatchFields(IgnoreExtras, Fields{
					"Kind": Equal("ShadowEndpointSlice"),
					"Name": Equal(shadowEpsName),
				})),
			)
		})
	})

	When("network remapping is enabled", func() {
		BeforeEach(func() {
			testShadowEps = newShadowEps(true)
			testFc = newFc(true, true)
			testConf = newConfiguration(true)
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps.DeepCopy(), testFc, testConf).Build()
		})

		It("should remap ep ip", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			for i := range testShadowEps.Spec.Template.Endpoints {
				Expect(eps.Endpoints[i].Addresses).To(HaveLen(1))
				Expect(eps.Endpoints[i].Addresses[0]).To(HavePrefix("10.30."))
			}
		})
	})

	When("foreign cluster network not ready and endpoints ready", func() {
		BeforeEach(func() {
			testShadowEps = newShadowEps(true)
			testFc = newFc(false, true)
			testConf = newConfiguration(true)
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps, testFc, testConf).Build()
		})

		It("should set remote endpoints to not ready", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			for i := range eps.Endpoints {
				Expect(eps.Endpoints[i].Conditions.Ready).To(PointTo(BeFalse()))
			}
		})
	})

	When("foreign cluster API server not ready and endpoints ready", func() {
		BeforeEach(func() {
			testShadowEps = newShadowEps(true)
			testFc = newFc(true, false)
			testConf = newConfiguration(true)
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps, testFc, testConf).Build()
		})

		It("should set remote endpoints to not ready", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			for i := range eps.Endpoints {
				Expect(eps.Endpoints[i].Conditions.Ready).To(PointTo(BeFalse()))
			}
		})
	})

	When("foreign cluster network and API server are not ready and endpoints ready", func() {
		BeforeEach(func() {
			testShadowEps = newShadowEps(true)
			testFc = newFc(false, false)
			testConf = newConfiguration(true)
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps, testFc, testConf).Build()
		})

		It("should set remote endpoints to not ready", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			for i := range eps.Endpoints {
				Expect(eps.Endpoints[i].Conditions.Ready).To(PointTo(BeFalse()))
			}
		})
	})
})

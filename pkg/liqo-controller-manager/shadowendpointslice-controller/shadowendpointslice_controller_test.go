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
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	liqodiscovery "github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("ShadowEndpointSlice Controller", func() {
	const (
		shadowEpsNamespace string = "default"
		shadowEpsName      string = "test-shadow-eps"
		testFcName         string = "test-fc-name"
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

		newFc = func(networkReady, apiServerReady bool) *discoveryv1alpha1.ForeignCluster {
			networkStatus := discoveryv1alpha1.PeeringConditionStatusEstablished
			if !networkReady {
				networkStatus = discoveryv1alpha1.PeeringConditionStatusError
			}

			apiServerStatus := discoveryv1alpha1.PeeringConditionStatusEstablished
			if !apiServerReady {
				apiServerStatus = discoveryv1alpha1.PeeringConditionStatusError
			}

			return &discoveryv1alpha1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testFcName,
					Labels: map[string]string{
						liqodiscovery.ClusterIDLabel: testFcID,
					},
				},
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
						ClusterID:   testFcName,
						ClusterName: testFcID,
					},
				},
				Status: discoveryv1alpha1.ForeignClusterStatus{
					PeeringConditions: []discoveryv1alpha1.PeeringCondition{
						{
							Type:   discoveryv1alpha1.NetworkStatusCondition,
							Status: networkStatus,
						},
						{
							Type:   discoveryv1alpha1.APIServerStatusCondition,
							Status: apiServerStatus,
						},
					},
				},
			}
		}

		newShadowEps = func(endpointsReady bool) *vkv1alpha1.ShadowEndpointSlice {
			ready := pointer.Bool(true)
			if !endpointsReady {
				ready = pointer.Bool(false)
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
							NodeName: pointer.String(testFcName),
							Conditions: discoveryv1.EndpointConditions{
								Ready: ready,
							},
							Addresses: []string{"192.168.0.1"},
						}},
						Ports:       []discoveryv1.EndpointPort{{Name: pointer.String("HTTPS")}},
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
							Controller:         pointer.Bool(true),
							BlockOwnerDeletion: pointer.Bool(true),
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
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps, testEps, testFc).Build()
		})

		It("should output the correct log", func() {
			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("endpointslice %q found running, will update it", klog.KObj(testEps))))
			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("updated endpointslice %q with success", klog.KObj(testEps))))
		})

		It("should update endpointslice metadata to shadowendpointslice metadata", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.Labels).To(HaveKeyWithValue(liqoconsts.ManagedByLabelKey, liqoconsts.ManagedByShadowEndpointSliceValue))
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
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps, testFc).Build()
		})

		It("should output the correct log", func() {
			Expect(buffer.String()).To(ContainSubstring(
				fmt.Sprintf("created endpointslice %q for shadowendpointslice %q", klog.KObj(testShadowEps), klog.KObj(testShadowEps))))
		})

		It("should create endpointslice metadata with shadowendpointslice metadata", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.Labels).To(HaveKeyWithValue(liqoconsts.ManagedByLabelKey, liqoconsts.ManagedByShadowEndpointSliceValue))
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

	When("foreign cluster network not ready and endpoints ready", func() {
		BeforeEach(func() {
			testShadowEps = newShadowEps(true)
			testFc = newFc(false, true)
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps, testFc).Build()
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
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps, testFc).Build()
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
			fakeClient = fakeClientBuilder.WithObjects(testShadowEps, testFc).Build()
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

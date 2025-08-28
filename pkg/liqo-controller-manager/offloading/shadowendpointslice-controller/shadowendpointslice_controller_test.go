// Copyright 2019-2026 The Liqo Authors
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
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	directconnectioninfo "github.com/liqotech/liqo/pkg/utils/directconnection"
	"github.com/liqotech/liqo/pkg/utils/errors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// capturingRecorder wraps a FakeRecorder to additionally expose the object each event was
// recorded against, which FakeRecorder's own string-based Events channel cannot reliably convey
// (the fake client does not populate TypeMeta on Get, so Kind/APIVersion always come back empty).
type capturingRecorder struct {
	*record.FakeRecorder
	lastObject client.Object
}

func (r *capturingRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	r.lastObject, _ = object.(client.Object)
	r.FakeRecorder.Event(object, eventtype, reason, message)
}

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
		ctx                   context.Context
		res                   ctrl.Result
		err                   error
		buffer                *bytes.Buffer
		fakeClient            client.WithWatch
		denyDirectConnections bool
		recorder              *capturingRecorder

		testShadowEps *offloadingv1beta1.ShadowEndpointSlice
		testEps       *discoveryv1.EndpointSlice
		testFc        *liqov1beta1.ForeignCluster
		testConf      *networkingv1beta1.Configuration

		newFc = func(networkReady, apiServerReady bool) *liqov1beta1.ForeignCluster {
			networkStatus := liqov1beta1.ConditionStatusEstablished
			if !networkReady {
				networkStatus = liqov1beta1.ConditionStatusError
			}

			apiServerStatus := liqov1beta1.ConditionStatusEstablished
			if !apiServerReady {
				apiServerStatus = liqov1beta1.ConditionStatusError
			}

			return &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testFcID,
					Labels: map[string]string{
						consts.RemoteClusterID: testFcID,
					},
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: liqov1beta1.ClusterID(testFcID),
				},
				Status: liqov1beta1.ForeignClusterStatus{
					Modules: liqov1beta1.Modules{
						Networking: liqov1beta1.Module{
							Enabled: true,
							Conditions: []liqov1beta1.Condition{
								{
									Type:   liqov1beta1.NetworkConnectionStatusCondition,
									Status: networkStatus,
								},
							},
						},
					},
					Conditions: []liqov1beta1.Condition{
						{
							Type:   liqov1beta1.APIServerStatusCondition,
							Status: apiServerStatus,
						},
					},
				},
			}
		}

		newShadowEps = func(endpointsReady bool) *offloadingv1beta1.ShadowEndpointSlice {
			ready := ptr.To(true)
			if !endpointsReady {
				ready = ptr.To(false)
			}

			return &offloadingv1beta1.ShadowEndpointSlice{
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
				Spec: offloadingv1beta1.ShadowEndpointSliceSpec{
					Template: offloadingv1beta1.EndpointSliceTemplate{
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

		newConfiguration = func(remapped bool) *networkingv1beta1.Configuration {
			var remappedPodCIDRs, remappedExternalCIDRs []string
			if remapped {
				remappedPodCIDRs = []string{"10.30.0.0/16", "10.50.0.0/16"}
				remappedExternalCIDRs = []string{"10.40.0.0/16", "10.60.0.0/16"}
			} else {
				remappedPodCIDRs = []string{"10.10.0.0/16", "10.11.0.0/16"}
				remappedExternalCIDRs = []string{"10.20.0.0/16", "10.21.0.0/16"}
			}

			return &networkingv1beta1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Namespace:  "default",
					Generation: 1,
					Labels: map[string]string{
						consts.RemoteClusterID: testFcID,
					},
				},
				Spec: networkingv1beta1.ConfigurationSpec{
					Remote: networkingv1beta1.ClusterConfig{
						CIDR: networkingv1beta1.ClusterConfigCIDR{
							Pod:      cidrutils.FromStrings([]string{"10.10.0.0/16", "10.11.0.0/16"}),
							External: cidrutils.FromStrings([]string{"10.20.0.0/16", "10.21.0.0/16"}),
						},
					},
				},
				Status: networkingv1beta1.ConfigurationStatus{
					Conditions: []metav1.Condition{{
						Type:               networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured,
						Status:             metav1.ConditionTrue,
						Reason:             "NetworkCIDRsConfigured",
						Message:            "All network CIDRs are configured",
						ObservedGeneration: 1,
						LastTransitionTime: metav1.Now(),
					}},
					Remote: &networkingv1beta1.ClusterConfig{
						CIDR: networkingv1beta1.ClusterConfigCIDR{
							Pod:      cidrutils.FromStrings(remappedPodCIDRs),
							External: cidrutils.FromStrings(remappedExternalCIDRs),
						},
					},
				},
			}
		}
	)

	BeforeEach(func() {
		ctx = context.TODO()
		buffer = &bytes.Buffer{}
		denyDirectConnections = false
		recorder = &capturingRecorder{FakeRecorder: record.NewFakeRecorder(100)}
		klog.SetOutput(buffer)
	})

	JustBeforeEach(func() {
		r := &Reconciler{
			Client:                fakeClient,
			Scheme:                scheme.Scheme,
			Recorder:              recorder,
			DenyDirectConnections: denyDirectConnections,
		}
		_, err = r.Reconcile(ctx, req)
		if errors.CheckFakeClientServerSideApplyError(err) {
			Skip("Skipping test due to fake client server-side apply error")
		}
		Expect(err).NotTo(HaveOccurred())
		klog.Flush()
	})

	When("shadowendpointslice is not found", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
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
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testShadowEps, testEps, testFc, testConf).Build()
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
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testShadowEps.DeepCopy(), testFc, testConf).Build()
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
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testShadowEps.DeepCopy(), testFc, testConf).Build()
		})

		It("should remap ep ip", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			for i := range testShadowEps.Spec.Template.Endpoints {
				Expect(eps.Endpoints[i].Addresses).To(HaveLen(1))
				Expect(eps.Endpoints[i].Addresses[0]).To(HavePrefix("10.30."))
			}
		})

		When("the shadow endpoint belongs to the second remote pod CIDR", func() {
			BeforeEach(func() {
				testShadowEps = newShadowEps(true)
				testShadowEps.Spec.Template.Endpoints[0].Addresses = []string{"10.11.0.1"}
				testFc = newFc(true, true)
				testConf = newConfiguration(true)
				fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testShadowEps.DeepCopy(), testFc, testConf).Build()
			})

			It("should remap using the aligned second status pod CIDR", func() {
				eps := discoveryv1.EndpointSlice{}
				Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
				Expect(eps.Endpoints).To(HaveLen(1))
				Expect(eps.Endpoints[0].Addresses).To(Equal([]string{"10.50.0.1"}))
			})
		})
	})

	When("foreign cluster network not ready and endpoints ready", func() {
		BeforeEach(func() {
			testShadowEps = newShadowEps(true)
			testFc = newFc(false, true)
			testConf = newConfiguration(true)
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testShadowEps, testFc, testConf).Build()
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
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testShadowEps, testFc, testConf).Build()
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
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testShadowEps, testFc, testConf).Build()
		})

		It("should set remote endpoints to not ready", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			for i := range eps.Endpoints {
				Expect(eps.Endpoints[i].Conditions.Ready).To(PointTo(BeFalse()))
			}
		})
	})

	When("networking module is disabled and direct connection data is present", func() {
		const directProviderID = "direct-provider-id"

		// ForeignCluster with networking module disabled (offloading-only peering).
		newFcNetworkingDisabled := func() *liqov1beta1.ForeignCluster {
			return &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testFcID,
					Labels: map[string]string{
						consts.RemoteClusterID: testFcID,
					},
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: liqov1beta1.ClusterID(testFcID),
				},
				Status: liqov1beta1.ForeignClusterStatus{
					Modules: liqov1beta1.Modules{
						Networking: liqov1beta1.Module{Enabled: false},
					},
					Conditions: []liqov1beta1.Condition{
						{
							Type:   liqov1beta1.APIServerStatusCondition,
							Status: liqov1beta1.ConditionStatusEstablished,
						},
					},
				},
			}
		}

		// Configuration for the direct provider (provider B), with index-aligned remote (desired) and remapped pod CIDRs.
		// The direct provider is the remote cluster of this Configuration (RemoteClusterID label), so its real pod
		// addresses live in Spec.Remote, and Status.Remote holds the CIDR used to reach that cluster.
		newDirectProviderConf := func(remotePodCIDRs, remappedPodCIDRs []string) *networkingv1beta1.Configuration {
			return &networkingv1beta1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "direct-provider-conf",
					Namespace: shadowEpsNamespace,
					Labels: map[string]string{
						consts.RemoteClusterID: directProviderID,
					},
					Generation: 1,
				},
				Spec: networkingv1beta1.ConfigurationSpec{
					Remote: networkingv1beta1.ClusterConfig{
						CIDR: networkingv1beta1.ClusterConfigCIDR{
							Pod: cidrutils.FromStrings(remotePodCIDRs),
						},
					},
				},
				Status: networkingv1beta1.ConfigurationStatus{
					Remote: &networkingv1beta1.ClusterConfig{
						CIDR: networkingv1beta1.ClusterConfigCIDR{
							Pod: cidrutils.FromStrings(remappedPodCIDRs),
						},
					},
				},
			}
		}

		// directAnnotation builds a DirectConnectionDataAnnotationKey annotation value
		// that maps the given addresses to directProviderID.
		directAnnotation := func(addrs ...string) string {
			d := directconnectioninfo.ClusterAddresses{}
			d.Add(directProviderID, addrs...)
			jsonBytes, err := d.ToJSON()
			Expect(err).NotTo(HaveOccurred())
			return string(jsonBytes)
		}

		// newDirectProviderConn returns a Connected Connection towards the direct provider:
		// translation through its Configuration is only attempted once the connection is
		// confirmed up (see directConnectionUp in the reconciler), so every fixture in this
		// block that exercises the remapping needs one.
		newDirectProviderConn := func() *networkingv1beta1.Connection {
			return &networkingv1beta1.Connection{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gw-direct-provider", Namespace: shadowEpsNamespace,
					Labels: map[string]string{consts.RemoteClusterID: directProviderID},
				},
				Status: networkingv1beta1.ConnectionStatus{Value: networkingv1beta1.Connected},
			}
		}

		BeforeEach(func() {
			// Single endpoint whose address belongs to the direct provider.
			// 10.20.0.1 is in the direct-connection index and should be remapped to 10.30.0.1.
			testShadowEps = &offloadingv1beta1.ShadowEndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shadowEpsName,
					Namespace: shadowEpsNamespace,
					Labels: map[string]string{
						discoveryv1.LabelManagedBy:   forge.EndpointSliceManagedBy,
						forge.LiqoOriginClusterIDKey: testFcID,
					},
					Annotations: map[string]string{
						consts.DirectConnectionDataAnnotationKey: directAnnotation("10.20.0.1"),
					},
				},
				Spec: offloadingv1beta1.ShadowEndpointSliceSpec{
					Template: offloadingv1beta1.EndpointSliceTemplate{
						Endpoints: []discoveryv1.Endpoint{
							{Addresses: []string{"10.20.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
						},
						Ports:       []discoveryv1.EndpointPort{{Name: ptr.To("HTTPS")}},
						AddressType: discoveryv1.AddressTypeIPv4,
					},
				},
			}
			fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(
				testShadowEps.DeepCopy(), newFcNetworkingDisabled(),
				newDirectProviderConf([]string{"10.20.0.0/16"}, []string{"10.30.0.0/16"}),
				newDirectProviderConn(),
			).Build()
		})

		It("should remap direct-connection endpoint addresses using the direct provider configuration", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.Endpoints).To(HaveLen(1))
			Expect(eps.Endpoints[0].Addresses).To(Equal([]string{"10.30.0.1"}))
		})

		It("should not propagate the direct connection data annotation to the endpointslice", func() {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.Annotations).NotTo(HaveKey(consts.DirectConnectionDataAnnotationKey))
		})

		When("some endpoint addresses are not in the direct connection index", func() {
			const nonDirectAddr = "192.168.100.1"

			BeforeEach(func() {
				// Only "10.20.0.1" is registered in the direct-connection index; "192.168.100.1" is not.
				testShadowEps = &offloadingv1beta1.ShadowEndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shadowEpsName,
						Namespace: shadowEpsNamespace,
						Labels: map[string]string{
							discoveryv1.LabelManagedBy:   forge.EndpointSliceManagedBy,
							forge.LiqoOriginClusterIDKey: testFcID,
						},
						Annotations: map[string]string{
							consts.DirectConnectionDataAnnotationKey: directAnnotation("10.20.0.1"),
						},
					},
					Spec: offloadingv1beta1.ShadowEndpointSliceSpec{
						Template: offloadingv1beta1.EndpointSliceTemplate{
							Endpoints: []discoveryv1.Endpoint{
								{Addresses: []string{"10.20.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
								{Addresses: []string{nonDirectAddr}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
							},
							Ports:       []discoveryv1.EndpointPort{{Name: ptr.To("HTTPS")}},
							AddressType: discoveryv1.AddressTypeIPv4,
						},
					},
				}
				fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(
					testShadowEps.DeepCopy(), newFcNetworkingDisabled(),
					newDirectProviderConf([]string{"10.20.0.0/16"}, []string{"10.30.0.0/16"}),
					newDirectProviderConn(),
				).Build()
			})

			It("should remap only the direct-connection address and leave others unchanged", func() {
				eps := discoveryv1.EndpointSlice{}
				Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
				Expect(eps.Endpoints).To(HaveLen(2))
				Expect(eps.Endpoints[0].Addresses).To(Equal([]string{"10.30.0.1"}))
				Expect(eps.Endpoints[1].Addresses).To(Equal([]string{nonDirectAddr}))
			})
		})

		When("the direct provider has multiple pod CIDRs", func() {
			BeforeEach(func() {
				testShadowEps = &offloadingv1beta1.ShadowEndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shadowEpsName,
						Namespace: shadowEpsNamespace,
						Labels: map[string]string{
							discoveryv1.LabelManagedBy:   forge.EndpointSliceManagedBy,
							forge.LiqoOriginClusterIDKey: testFcID,
						},
						Annotations: map[string]string{
							consts.DirectConnectionDataAnnotationKey: directAnnotation("10.10.0.1", "10.20.0.1"),
						},
					},
					Spec: offloadingv1beta1.ShadowEndpointSliceSpec{
						Template: offloadingv1beta1.EndpointSliceTemplate{
							Endpoints: []discoveryv1.Endpoint{
								{Addresses: []string{"10.10.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
								{Addresses: []string{"10.20.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
							},
							Ports:       []discoveryv1.EndpointPort{{Name: ptr.To("HTTPS")}},
							AddressType: discoveryv1.AddressTypeIPv4,
						},
					},
				}
				fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(
					testShadowEps.DeepCopy(), newFcNetworkingDisabled(),
					newDirectProviderConf(
						[]string{"10.10.0.0/16", "10.20.0.0/16"},
						[]string{"10.40.0.0/16", "10.30.0.0/16"},
					),
					newDirectProviderConn(),
				).Build()
			})

			It("should remap each direct address using the matching pod CIDR index", func() {
				eps := discoveryv1.EndpointSlice{}
				Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
				Expect(eps.Endpoints).To(HaveLen(2))
				Expect(eps.Endpoints[0].Addresses).To(Equal([]string{"10.40.0.1"}))
				Expect(eps.Endpoints[1].Addresses).To(Equal([]string{"10.30.0.1"}))
			})
		})
	})

	When("the slice takes part in direct connections (readiness failover)", func() {
		const directProviderID = "direct-provider-id"

		// ForeignCluster with networking module disabled: only the direct-connection remapping
		// path runs, so the fixtures stay minimal and focused on the readiness behavior.
		newFcNetworkingDisabled := func() *liqov1beta1.ForeignCluster {
			return &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   testFcID,
					Labels: map[string]string{consts.RemoteClusterID: testFcID},
				},
				Spec: liqov1beta1.ForeignClusterSpec{ClusterID: liqov1beta1.ClusterID(testFcID)},
				Status: liqov1beta1.ForeignClusterStatus{
					Modules: liqov1beta1.Modules{Networking: liqov1beta1.Module{Enabled: false}},
					Conditions: []liqov1beta1.Condition{{
						Type:   liqov1beta1.APIServerStatusCondition,
						Status: liqov1beta1.ConditionStatusEstablished,
					}},
				},
			}
		}

		newDirectProviderConf := func() *networkingv1beta1.Configuration {
			return &networkingv1beta1.Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "direct-provider-conf", Namespace: shadowEpsNamespace,
					Labels:     map[string]string{consts.RemoteClusterID: directProviderID},
					Generation: 1,
				},
				Spec: networkingv1beta1.ConfigurationSpec{
					Remote: networkingv1beta1.ClusterConfig{CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod: cidrutils.FromStrings([]string{"10.20.0.0/16"})}},
				},
				Status: networkingv1beta1.ConfigurationStatus{
					Remote: &networkingv1beta1.ClusterConfig{CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod: cidrutils.FromStrings([]string{"10.30.0.0/16"})}},
				},
			}
		}

		directAnnotation := func(addrs ...string) string {
			d := directconnectioninfo.ClusterAddresses{}
			d.Add(directProviderID, addrs...)
			jsonBytes, errJSON := d.ToJSON()
			Expect(errJSON).NotTo(HaveOccurred())
			return string(jsonBytes)
		}

		newConnection := func(status networkingv1beta1.ConnectionStatusValue) *networkingv1beta1.Connection {
			return &networkingv1beta1.Connection{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gw-direct-provider", Namespace: shadowEpsNamespace,
					Labels: map[string]string{consts.RemoteClusterID: directProviderID},
				},
				Status: networkingv1beta1.ConnectionStatus{Value: status},
			}
		}

		// newShadowEpsWithRole forges a direct slice or an indirect companion carrying the
		// direct-connections data annotation for directProviderID.
		newShadowEpsWithRole := func(indirect bool, annotation string, addresses ...string) *offloadingv1beta1.ShadowEndpointSlice {
			shadowLabels := map[string]string{
				discoveryv1.LabelManagedBy:   forge.EndpointSliceManagedBy,
				forge.LiqoOriginClusterIDKey: testFcID,
				discoveryv1.LabelServiceName: "test-service",
			}
			if indirect {
				shadowLabels[forge.IndirectEndpointSliceLabelKey] = "true"
			}
			annotations := map[string]string{}
			if annotation != "" {
				annotations[consts.DirectConnectionDataAnnotationKey] = annotation
			}
			endpoints := make([]discoveryv1.Endpoint, 0, len(addresses))
			for _, addr := range addresses {
				endpoints = append(endpoints, discoveryv1.Endpoint{
					Addresses:  []string{addr},
					Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
				})
			}
			return &offloadingv1beta1.ShadowEndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name: shadowEpsName, Namespace: shadowEpsNamespace,
					Labels: shadowLabels, Annotations: annotations,
				},
				Spec: offloadingv1beta1.ShadowEndpointSliceSpec{
					Template: offloadingv1beta1.EndpointSliceTemplate{
						Endpoints:   endpoints,
						Ports:       []discoveryv1.EndpointPort{{Name: ptr.To("HTTPS")}},
						AddressType: discoveryv1.AddressTypeIPv4,
					},
				},
			}
		}

		expectEndpointsReady := func(ready bool) {
			eps := discoveryv1.EndpointSlice{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
			Expect(eps.Endpoints).NotTo(BeEmpty())
			for i := range eps.Endpoints {
				Expect(eps.Endpoints[i].Conditions.Ready).To(PointTo(Equal(ready)))
			}
		}

		When("the slice is direct", func() {
			// The fixture slice is mixed: 10.20.0.1 depends on the direct provider (listed in the
			// annotation, translated to 10.30.0.1 through its Configuration), while 10.99.0.9 is
			// path-independent (e.g. hosted on the consumer) and must never be affected by the
			// state of the direct connections.
			build := func(objs ...client.Object) {
				objs = append(objs,
					newShadowEpsWithRole(false, directAnnotation("10.20.0.1"), "10.20.0.1", "10.99.0.9"),
					newFcNetworkingDisabled(), newDirectProviderConf())
				fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).Build()
			}

			// expectReadiness asserts the exact set of endpoint addresses and their Ready values.
			expectReadiness := func(want map[string]bool) {
				eps := discoveryv1.EndpointSlice{}
				Expect(fakeClient.Get(ctx, req.NamespacedName, &eps)).To(Succeed())
				got := map[string]bool{}
				for i := range eps.Endpoints {
					got[eps.Endpoints[i].Addresses[0]] = *eps.Endpoints[i].Conditions.Ready
				}
				Expect(got).To(Equal(want))
			}

			When("the direct connection is established", func() {
				BeforeEach(func() { build(newConnection(networkingv1beta1.Connected)) })
				It("all endpoints should be ready (direct one translated)", func() {
					expectReadiness(map[string]bool{"10.30.0.1": true, "10.99.0.9": true})
				})
			})

			When("the direct connection is in error", func() {
				BeforeEach(func() { build(newConnection(networkingv1beta1.ConnectionError)) })
				It("only the direct endpoint should turn not ready, translated address kept (no churn)", func() {
					expectReadiness(map[string]bool{"10.30.0.1": false, "10.99.0.9": true})
				})
				It("should not raise a misconfiguration event (the connection exists, just down)", func() {
					Expect(recorder.Events).ToNot(Receive())
				})
			})

			When("the direct connection does not exist", func() {
				// Never-peered misconfiguration: the reconcile completes, materializing the slice
				// WITHOUT the endpoints of the unpeered cluster (their hub copies in the indirect
				// companion serve the traffic), and surfaces the problem with an event.
				BeforeEach(func() { build() })

				It("should exclude the unpeered endpoints and keep path-independent ones ready", func() {
					expectReadiness(map[string]bool{"10.99.0.9": true})
				})
				It("should raise a misconfiguration event naming the unpeered cluster", func() {
					Expect(recorder.Events).To(Receive(SatisfyAll(
						ContainSubstring(EventReasonDirectConnectionNotPeered),
						ContainSubstring(directProviderID),
					)))
				})
				It("should fall back to the ShadowEndpointSlice when the Service cannot be resolved", func() {
					Expect(recorder.Events).To(Receive())
					Expect(recorder.lastObject).To(BeAssignableToTypeOf(&offloadingv1beta1.ShadowEndpointSlice{}))
					Expect(recorder.lastObject.GetName()).To(Equal(shadowEpsName))
				})

				When("the reflected Service exists on this provider", func() {
					BeforeEach(func() {
						svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test-service", Namespace: shadowEpsNamespace}}
						Expect(fakeClient.Create(ctx, svc)).To(Succeed())
					})

					It("should raise the event on the Service, not the ShadowEndpointSlice", func() {
						Expect(recorder.Events).To(Receive(ContainSubstring(EventReasonDirectConnectionNotPeered)))
						Expect(recorder.lastObject).To(BeAssignableToTypeOf(&corev1.Service{}))
						Expect(recorder.lastObject.GetName()).To(Equal("test-service"))
					})
				})

				When("an endpointslice was previously materialized (peering removed afterwards)", func() {
					BeforeEach(func() {
						// Simulates un-peering: the EndpointSlice created while the peering existed
						// is still there, with the translated direct address marked ready.
						build(&discoveryv1.EndpointSlice{
							ObjectMeta: metav1.ObjectMeta{Name: shadowEpsName, Namespace: shadowEpsNamespace},
							Endpoints: []discoveryv1.Endpoint{{
								Addresses:  []string{"10.30.0.1"},
								Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
							}},
						})
					})

					It("should flush the stale direct address, keeping the path-independent endpoints", func() {
						expectReadiness(map[string]bool{"10.99.0.9": true})
					})
				})
			})

			When("direct connections are denied", func() {
				BeforeEach(func() {
					denyDirectConnections = true
					// No Configuration for the direct provider exists either: denying must not
					// attempt any remapping through it in the first place (the direct endpoint
					// stays untranslated and permanently not-ready).
					fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(
						newShadowEpsWithRole(false, directAnnotation("10.20.0.1"), "10.20.0.1", "10.99.0.9"),
						newFcNetworkingDisabled(), newConnection(networkingv1beta1.Connected)).Build()
				})

				It("should keep the direct endpoint present but not ready, path-independent ones ready", func() {
					expectReadiness(map[string]bool{"10.20.0.1": false, "10.99.0.9": true})
				})
				It("should not raise a misconfiguration event (denying is an explicit operator choice)", func() {
					Expect(recorder.Events).ToNot(Receive())
				})
			})

			When("no Configuration exists for the direct peer at all (providers never network-peered)", func() {
				BeforeEach(func() {
					// FC networking enabled between consumer and this provider (the common case for
					// direct connections; requires its own Configuration, unrelated to the direct
					// peer). No Configuration and no Connection exist at all for the direct peer:
					// P1 and P2 were simply never network-peered together.
					fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(
						newShadowEpsWithRole(false, directAnnotation("10.20.0.1"), "10.20.0.1"),
						newFc(true, true), newConfiguration(false)).Build()
				})

				It("should reconcile without error, excluding the untranslatable endpoints", func() {
					// The remap through the nonexistent Configuration would fail with an opaque
					// NotFound: the not-peered endpoints must be excluded before translation.
					expectReadiness(map[string]bool{})
				})
				It("should raise a misconfiguration event naming the unpeered cluster", func() {
					Expect(recorder.Events).To(Receive(SatisfyAll(
						ContainSubstring(EventReasonDirectConnectionNotPeered),
						ContainSubstring(directProviderID),
					)))
				})
			})
		})

		When("the slice is the indirect companion", func() {
			build := func(annotation string, objs ...client.Object) {
				// 10.80.0.1 is a hub-and-spoke address, not part of the direct-connection index.
				objs = append(objs,
					newShadowEpsWithRole(true, annotation, "10.80.0.1"),
					newFcNetworkingDisabled())
				fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).Build()
			}

			When("the direct connection is established", func() {
				BeforeEach(func() { build(directAnnotation("10.20.0.1"), newConnection(networkingv1beta1.Connected)) })
				It("endpoints should be not ready (direct path active)", func() { expectEndpointsReady(false) })
			})

			When("the direct connection is in error", func() {
				BeforeEach(func() { build(directAnnotation("10.20.0.1"), newConnection(networkingv1beta1.ConnectionError)) })
				It("endpoints should be ready (failover to the indirect path)", func() { expectEndpointsReady(true) })
			})

			When("the direct connection does not exist", func() {
				BeforeEach(func() { build(directAnnotation("10.20.0.1")) })
				It("endpoints should be ready (failover to the indirect path)", func() { expectEndpointsReady(true) })
			})

			When("direct connections are denied", func() {
				BeforeEach(func() {
					denyDirectConnections = true
					build(directAnnotation("10.20.0.1"), newConnection(networkingv1beta1.Connected))
				})
				It("endpoints should be ready even though the connection is established", func() {
					expectEndpointsReady(true)
				})
			})

			When("the companion has no direct-connections data", func() {
				BeforeEach(func() { build("", newConnection(networkingv1beta1.Connected)) })
				It("endpoints should be not ready (fully overlapping with the direct slice)", func() {
					expectEndpointsReady(false)
				})
			})
		})
	})
})

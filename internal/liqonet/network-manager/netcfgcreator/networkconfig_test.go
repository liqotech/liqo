// Copyright 2019-2021 The Liqo Authors
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

package netcfgcreator

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
)

var _ = Describe("Network config functions", func() {
	const (
		clusterID = "fake"
		namespace = "liqo"
	)

	var (
		ctx           context.Context
		clientBuilder fake.ClientBuilder
		fcw           *NetworkConfigCreator
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
	})

	JustBeforeEach(func() {
		fcw = &NetworkConfigCreator{
			Client: clientBuilder.Build(),
			Scheme: scheme.Scheme,

			PodCIDR:      "192.168.0.0/24",
			ExternalCIDR: "192.168.1.0/24",

			secretWatcher:  &SecretWatcher{wiregardPublicKey: "public-key"},
			serviceWatcher: &ServiceWatcher{endpointIP: "1.1.1.1", endpointPort: "9999"},
		}
	})

	Describe("The GetLocalNetworkConfig function", func() {
		var (
			netcfg *netv1alpha1.NetworkConfig
			err    error
		)

		JustBeforeEach(func() {
			netcfg, err = GetLocalNetworkConfig(ctx, fcw.Client, clusterID, namespace)
		})

		When("the network config with the given cluster ID does not exist", func() {
			It("should return a not found error", func() {
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})
			It("should return a nil network config", func() { Expect(netcfg).To(BeNil()) })
		})

		When("the network config with the given cluster ID does exist", func() {
			var existing *netv1alpha1.NetworkConfig

			BeforeEach(func() {
				existing = &netv1alpha1.NetworkConfig{ObjectMeta: metav1.ObjectMeta{
					Name: "foo", Namespace: namespace, Labels: map[string]string{crdreplicator.DestinationLabel: clusterID},
				}}
				clientBuilder.WithObjects(existing)
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should return the expected network config", func() { Expect(netcfg).To(Equal(existing)) })
		})

		When("two network configs with the given cluster ID do exist", func() {
			var correct, duplicate *netv1alpha1.NetworkConfig

			Context("the two network configs have different creation timestamp", func() {
				BeforeEach(func() {
					correct = &netv1alpha1.NetworkConfig{ObjectMeta: metav1.ObjectMeta{
						Name: "foo", Namespace: namespace, Labels: map[string]string{crdreplicator.DestinationLabel: clusterID},
						UID: "aeda6412-e08c-4dcd-ab7d-ac12b286010b", CreationTimestamp: metav1.NewTime(time.Now().Truncate(time.Second)),
					}}
					duplicate = &netv1alpha1.NetworkConfig{ObjectMeta: metav1.ObjectMeta{
						Name: "bar", Namespace: namespace, Labels: map[string]string{crdreplicator.DestinationLabel: clusterID},
						UID:               "8a402261-9cf4-402e-89e8-4d743fb315fb",
						CreationTimestamp: metav1.NewTime(time.Now().Truncate(time.Second).Add(10 * time.Second)),
					}}

					clientBuilder.WithObjects(correct, duplicate)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should return the expected network config", func() { Expect(netcfg).To(Equal(correct)) })
				It("should delete the duplicated network config", func() {
					ref := types.NamespacedName{Name: duplicate.Name, Namespace: duplicate.Namespace}
					Expect(kerrors.IsNotFound(fcw.Client.Get(ctx, ref, netcfg))).To(BeTrue())
				})
			})

			Context("the two network configs have the same creation timestamp", func() {
				BeforeEach(func() {
					correct = &netv1alpha1.NetworkConfig{ObjectMeta: metav1.ObjectMeta{
						Name: "foo", Namespace: namespace, Labels: map[string]string{crdreplicator.DestinationLabel: clusterID},
						UID: "8a402261-9cf4-402e-89e8-4d743fb315fb", CreationTimestamp: metav1.NewTime(time.Now().Truncate(time.Second)),
					}}
					duplicate = &netv1alpha1.NetworkConfig{ObjectMeta: metav1.ObjectMeta{
						Name: "bar", Namespace: namespace, Labels: map[string]string{crdreplicator.DestinationLabel: clusterID},
						UID: "aeda6412-e08c-4dcd-ab7d-ac12b286010b", CreationTimestamp: metav1.NewTime(time.Now().Truncate(time.Second)),
					}}

					clientBuilder.WithObjects(correct, duplicate)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should return the expected network config", func() { Expect(netcfg).To(Equal(correct)) })
				It("should delete the duplicated network config", func() {
					ref := types.NamespacedName{Name: duplicate.Name, Namespace: duplicate.Namespace}
					Expect(kerrors.IsNotFound(fcw.Client.Get(ctx, ref, netcfg))).To(BeTrue())
				})
			})
		})
	})

	Describe("The GetRemoteNetworkConfig function", func() {
		var (
			netcfg *netv1alpha1.NetworkConfig
			err    error
		)

		JustBeforeEach(func() {
			netcfg, err = GetRemoteNetworkConfig(ctx, fcw.Client, clusterID, namespace)
		})

		When("the network config with the given cluster ID does not exist", func() {
			It("should return a not found error", func() {
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})
			It("should return a nil network config", func() { Expect(netcfg).To(BeNil()) })
		})

		When("the network config with the given cluster ID does exist", func() {
			var existing *netv1alpha1.NetworkConfig

			BeforeEach(func() {
				existing = &netv1alpha1.NetworkConfig{ObjectMeta: metav1.ObjectMeta{
					Name: "foo", Namespace: namespace, Labels: map[string]string{crdreplicator.RemoteLabelSelector: clusterID},
				}}
				clientBuilder.WithObjects(existing)
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should return the expected network config", func() { Expect(netcfg).To(Equal(existing)) })
		})

		When("two network configs with the given cluster ID do exist", func() {
			BeforeEach(func() {
				clientBuilder.WithObjects(&netv1alpha1.NetworkConfig{ObjectMeta: metav1.ObjectMeta{
					Name: "foo", Namespace: namespace, Labels: map[string]string{crdreplicator.RemoteLabelSelector: clusterID},
				}}, &netv1alpha1.NetworkConfig{ObjectMeta: metav1.ObjectMeta{
					Name: "bar", Namespace: namespace, Labels: map[string]string{crdreplicator.RemoteLabelSelector: clusterID},
				}})
			})

			It("should fail with an error", func() { Expect(err).To(HaveOccurred()) })
			It("should return a nil network config", func() { Expect(netcfg).To(BeNil()) })
		})
	})

	Describe("The Enforce* functions", func() {
		var (
			fc  *discoveryv1alpha1.ForeignCluster
			err error
		)

		BeforeEach(func() {
			fc = &discoveryv1alpha1.ForeignCluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: discoveryv1alpha1.GroupVersion.String(),
					Kind:       "ForeignCluster",
				},
				ObjectMeta: metav1.ObjectMeta{Name: "whatever", UID: "8a402261-9cf4-402e-89e8-4d743fb315fb"},
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: clusterID},
				},
				Status: discoveryv1alpha1.ForeignClusterStatus{
					TenantNamespace: discoveryv1alpha1.TenantNamespaceType{Local: namespace},
				},
			}
		})

		Describe("The EnforceNetworkConfigPresence function", func() {
			JustBeforeEach(func() {
				err = fcw.EnforceNetworkConfigPresence(ctx, fc)
			})

			AssertNetworkConfigMeta := func(netcfg *netv1alpha1.NetworkConfig) {
				Expect(netcfg.Labels).To(HaveKeyWithValue(crdreplicator.LocalLabelSelector, "true"))
				Expect(netcfg.Labels).To(HaveKeyWithValue(crdreplicator.DestinationLabel, clusterID))

				Expect(metav1.GetControllerOf(netcfg).Kind).To(Equal(fc.Kind))
				Expect(metav1.GetControllerOf(netcfg).APIVersion).To(Equal(fc.APIVersion))
				Expect(metav1.GetControllerOf(netcfg).Name).To(Equal(fc.GetName()))
				Expect(metav1.GetControllerOf(netcfg).UID).To(Equal(fc.GetUID()))
			}

			AssertNetworkConfigSpec := func(netcfg *netv1alpha1.NetworkConfig) {
				Expect(netcfg.Spec.ClusterID).To(BeIdenticalTo(clusterID))
				Expect(netcfg.Spec.PodCIDR).To(BeIdenticalTo("192.168.0.0/24"))
				Expect(netcfg.Spec.ExternalCIDR).To(BeIdenticalTo("192.168.1.0/24"))
				Expect(netcfg.Spec.EndpointIP).To(BeIdenticalTo("1.1.1.1"))
				Expect(netcfg.Spec.BackendType).To(BeIdenticalTo(wireguard.DriverName))
				Expect(netcfg.Spec.BackendConfig).To(HaveKeyWithValue(wireguard.PublicKey, "public-key"))
				Expect(netcfg.Spec.BackendConfig).To(HaveKeyWithValue(wireguard.ListeningPort, "9999"))
			}

			When("the network config associated with the given foreign cluster does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the network config should be present and have the correct object meta", func() {
					netcfg, err := GetLocalNetworkConfig(ctx, fcw.Client, clusterID, namespace)
					Expect(err).ToNot(HaveOccurred())
					Expect(netcfg.Name).To(HavePrefix("net-config-"))
					AssertNetworkConfigMeta(netcfg)
				})
				It("the network config should be present and have the correct specifications", func() {
					netcfg, err := GetLocalNetworkConfig(ctx, fcw.Client, clusterID, namespace)
					Expect(err).ToNot(HaveOccurred())
					AssertNetworkConfigSpec(netcfg)
				})
			})

			When("the network config associated with the given foreign cluster does already exist", func() {
				BeforeEach(func() {
					clientBuilder.WithObjects(
						&netv1alpha1.NetworkConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: "foo", Namespace: namespace, Labels: map[string]string{
									crdreplicator.DestinationLabel: clusterID,
									"other-key":                    "other-value",
								},
							},
							Spec: netv1alpha1.NetworkConfigSpec{ClusterID: "foo", EndpointIP: "bar", BackendType: "baz"},
						},
					)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the network config should be present and have the correct object meta", func() {
					netcfg, err := GetLocalNetworkConfig(ctx, fcw.Client, clusterID, namespace)
					Expect(err).ToNot(HaveOccurred())
					Expect(netcfg.Name).To(BeIdenticalTo("foo"))
					Expect(netcfg.Labels).To(HaveKeyWithValue("other-key", "other-value"))
					AssertNetworkConfigMeta(netcfg)
				})
				It("the network config should be present and have the correct specifications", func() {
					netcfg, err := GetLocalNetworkConfig(ctx, fcw.Client, clusterID, namespace)
					Expect(err).ToNot(HaveOccurred())
					AssertNetworkConfigSpec(netcfg)
				})
			})
		})

		Describe("The EnforceNetworkConfigAbsence function", func() {
			var (
				fc  *discoveryv1alpha1.ForeignCluster
				err error
			)

			BeforeEach(func() {
				fc = &discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "whatever"},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: clusterID},
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						TenantNamespace: discoveryv1alpha1.TenantNamespaceType{Local: namespace},
					},
				}

				_ = fc
			})

			JustBeforeEach(func() { err = fcw.EnforceNetworkConfigAbsence(ctx, fc) })

			When("the network config associated with the given foreign cluster does exist", func() {
				BeforeEach(func() {
					clientBuilder.WithObjects(
						&netv1alpha1.NetworkConfig{ObjectMeta: metav1.ObjectMeta{
							Name: "foo", Namespace: namespace, Labels: map[string]string{crdreplicator.DestinationLabel: clusterID},
						}},
					)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the network config should not be present", func() {
					_, err = GetLocalNetworkConfig(ctx, fcw.Client, clusterID, namespace)
					Expect(err).To(HaveOccurred())
					Expect(kerrors.IsNotFound(err)).To(BeTrue())
				})
			})

			When("the network config associated with the given foreign cluster does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the network config should not be present", func() {
					_, err = GetLocalNetworkConfig(ctx, fcw.Client, clusterID, namespace)
					Expect(err).To(HaveOccurred())
					Expect(kerrors.IsNotFound(err)).To(BeTrue())
				})
			})
		})
	})
})

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

package netcfgcreator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
	"github.com/liqotech/liqo/pkg/utils/syncset"
)

var _ = Describe("NetworkConfigCreator Controller", func() {
	const (
		clusterID      = "fake-id"
		clusterName    = "fake-name"
		namespace      = "liqo"
		foreigncluster = "foreign-cluster"
	)

	var (
		ctx context.Context
		err error

		ns  *corev1.Namespace
		ncc *NetworkConfigCreator
		fc  *discoveryv1alpha1.ForeignCluster
	)

	AssertNetworkConfigAbsence := func() func() {
		return func() {
			var networkConfigList netv1alpha1.NetworkConfigList
			Expect(ncc.List(ctx, &networkConfigList, client.InNamespace(namespace))).To(Succeed())
			Expect(networkConfigList.Items).To(HaveLen(0))
		}
	}

	BeforeEach(func() {
		ctx = context.Background()
		cl, err := client.New(testcluster.GetCfg(), client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())

		ncc = &NetworkConfigCreator{
			Client: cl,
			Scheme: scheme.Scheme,

			PodCIDR:      "192.168.0.0/24",
			ExternalCIDR: "192.168.1.0/24",

			foreignClusters: syncset.New(),
			secretWatcher: &SecretWatcher{
				wiregardPublicKey: "public-key",
				configured:        true,
			},
			serviceWatcher: &ServiceWatcher{
				endpointIP:   "1.1.1.1",
				endpointPort: "9999",
				configured:   true,
			},
		}

		// The deletion of namespaces in the test environment does not work.
		// https://github.com/kubernetes-sigs/controller-runtime/issues/880
		ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(liqoerrors.IgnoreAlreadyExists(ncc.Create(ctx, ns))).To(Succeed())

		fc = &discoveryv1alpha1.ForeignCluster{
			ObjectMeta: metav1.ObjectMeta{Name: foreigncluster},
			Spec: discoveryv1alpha1.ForeignClusterSpec{
				ClusterIdentity:        discoveryv1alpha1.ClusterIdentity{ClusterID: clusterID, ClusterName: clusterName},
				IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
				OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
				InsecureSkipTLSVerify:  pointer.Bool(true),
				ForeignAuthURL:         "https://foo.liqo.io",
			},
			Status: discoveryv1alpha1.ForeignClusterStatus{
				TenantNamespace: discoveryv1alpha1.TenantNamespaceType{Local: namespace},
			},
		}
	})

	JustBeforeEach(func() {
		fcstatus := fc.DeepCopy()
		Expect(ncc.Create(ctx, fc)).To(Succeed())
		fcstatus.ResourceVersion = fc.ResourceVersion
		Expect(ncc.Status().Update(ctx, fcstatus)).To(Succeed())

		_, err = ncc.Reconcile(ctx, controllerruntime.Request{NamespacedName: types.NamespacedName{Name: fc.Name}})
	})

	AfterEach(func() {
		var netcfg netv1alpha1.NetworkConfig
		Expect(ncc.Delete(ctx, fc)).To(Succeed())
		Expect(client.IgnoreNotFound(ncc.DeleteAllOf(ctx, &netcfg, client.InNamespace(namespace)))).To(Succeed())
	})

	CreateNetworkConfig := func() func() {
		return func() {
			netcfg := netv1alpha1.NetworkConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo", Namespace: namespace, Labels: map[string]string{
						consts.ReplicationDestinationLabel: clusterID,
						consts.LocalResourceOwnership:      componentName,
					},
				},
				Spec: netv1alpha1.NetworkConfigSpec{
					RemoteCluster: discoveryv1alpha1.ClusterIdentity{ClusterID: "foo-id", ClusterName: "foo-name"},
					EndpointIP:    "bar",
					BackendType:   "baz",
					BackendConfig: map[string]string{},
				},
			}
			Expect(ncc.Create(ctx, &netcfg)).To(Succeed())
		}
	}

	When("the ForeignCluster has peering type OutOfBand", func() {
		BeforeEach(func() {
			fc.Spec.PeeringType = discoveryv1alpha1.PeeringTypeOutOfBand
		})

		When("the ForeignCluster has the peering enabled", func() {
			ContextBody := func(initializer func()) func() {
				return func() {
					BeforeEach(initializer)

					AssertNetworkConfigMeta := func(netcfg *netv1alpha1.NetworkConfig) {
						Expect(netcfg.Labels).To(HaveKeyWithValue(consts.ReplicationRequestedLabel, "true"))
						Expect(netcfg.Labels).To(HaveKeyWithValue(consts.ReplicationDestinationLabel, clusterID))
						Expect(netcfg.Labels).To(HaveKeyWithValue(consts.LocalResourceOwnership, componentName))

						Expect(metav1.GetControllerOf(netcfg).Kind).To(Equal("ForeignCluster"))
						Expect(metav1.GetControllerOf(netcfg).APIVersion).To(Equal("discovery.liqo.io/v1alpha1"))
						Expect(metav1.GetControllerOf(netcfg).Name).To(Equal(fc.GetName()))
						Expect(metav1.GetControllerOf(netcfg).UID).To(Equal(fc.GetUID()))
					}

					AssertNetworkConfigSpec := func(netcfg *netv1alpha1.NetworkConfig) {
						Expect(netcfg.Spec.RemoteCluster.ClusterID).To(BeIdenticalTo(clusterID))
						Expect(netcfg.Spec.RemoteCluster.ClusterName).To(BeIdenticalTo(clusterName))
						Expect(netcfg.Spec.PodCIDR).To(BeIdenticalTo("192.168.0.0/24"))
						Expect(netcfg.Spec.ExternalCIDR).To(BeIdenticalTo("192.168.1.0/24"))
						Expect(netcfg.Spec.EndpointIP).To(BeIdenticalTo("1.1.1.1"))
						Expect(netcfg.Spec.BackendType).To(BeIdenticalTo(consts.DriverName))
						Expect(netcfg.Spec.BackendConfig).To(HaveKeyWithValue(consts.PublicKey, "public-key"))
						Expect(netcfg.Spec.BackendConfig).To(HaveKeyWithValue(consts.ListeningPort, "9999"))
					}

					AssertNetworkConfigCorrectness := func() func() {
						return func() {
							var networkConfigList netv1alpha1.NetworkConfigList
							Expect(ncc.List(ctx, &networkConfigList, client.InNamespace(namespace))).To(Succeed())
							Expect(networkConfigList.Items).To(HaveLen(1))
							AssertNetworkConfigMeta(&networkConfigList.Items[0])
							AssertNetworkConfigSpec(&networkConfigList.Items[0])
						}
					}

					When("the NetworkConfig does not yet exist", func() {
						It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
						It("should create the expected network config", AssertNetworkConfigCorrectness())
					})

					When("the NetworkConfig does already exist", func() {
						BeforeEach(CreateNetworkConfig())
						It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
						It("should update the existing network config", AssertNetworkConfigCorrectness())
					})
				}
			}

			Context("incoming peering", ContextBody(func() {
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.IncomingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusEstablished, "", "")
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusNone, "", "")
			}))

			Context("outgoing peering", ContextBody(func() {
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.IncomingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusNone, "", "")
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusEstablished, "", "")
			}))

			Context("bidirectional peering", ContextBody(func() {
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.IncomingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusEstablished, "", "")
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusEstablished, "", "")
			}))
		})

		When("the ForeignCluster has not a peering enabled", func() {
			BeforeEach(func() {
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.IncomingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusNone, "", "")
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusNone, "", "")
			})

			When("the NetworkConfig does exist", func() {
				BeforeEach(CreateNetworkConfig())
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should ensure the absence of the NetworkConfig", AssertNetworkConfigAbsence())
			})

			When("the NetworkConfig does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should ensure the absence of the NetworkConfig", AssertNetworkConfigAbsence())
			})
		})
	})

	When("the ForeignCluster has peering type InBand", func() {
		BeforeEach(func() {
			fc.Spec.PeeringType = discoveryv1alpha1.PeeringTypeInBand
		})

		ContextBody := func(initializer func()) func() {
			return func() {
				BeforeEach(initializer)

				When("the NetworkConfig does exist", func() {
					BeforeEach(CreateNetworkConfig())
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should ensure the absence of the NetworkConfig", AssertNetworkConfigAbsence())
				})

				When("the NetworkConfig does not exist", func() {
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should ensure the absence of the NetworkConfig", AssertNetworkConfigAbsence())
				})
			}
		}

		Context("peering is enabled", func() {
			Context("incoming peering", ContextBody(func() {
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.IncomingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusEstablished, "", "")
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusNone, "", "")
			}))

			Context("outgoing peering", ContextBody(func() {
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.IncomingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusNone, "", "")
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusEstablished, "", "")
			}))

			Context("bidirectional peering", ContextBody(func() {
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.IncomingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusEstablished, "", "")
				peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition,
					discoveryv1alpha1.PeeringConditionStatusEstablished, "", "")
			}))
		})

		Context("peering is not enabled", ContextBody(func() {
			peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.IncomingPeeringCondition,
				discoveryv1alpha1.PeeringConditionStatusNone, "", "")
			peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition,
				discoveryv1alpha1.PeeringConditionStatusNone, "", "")
		}))
	})
})

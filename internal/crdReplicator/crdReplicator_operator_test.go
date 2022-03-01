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

package crdreplicator_test

import (
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

var _ = Describe("CRD Replicator Operator Tests", func() {

	const (
		foreignClusterName  = "foreign-cluster"
		resourceRequestName = "resource-request"
		resourceOfferName   = "resource-offer"
		networkConfigName   = "network-config"

		authURL = "https://foo.bar"
	)

	var (
		foreignCluster  discoveryv1alpha1.ForeignCluster
		resourceRequest discoveryv1alpha1.ResourceRequest
		resourceOffer   sharingv1alpha1.ResourceOffer
		networkConfig   netv1alpha1.NetworkConfig

		foreignClusterNotFound  error
		resourceRequestNotFound error
		resourceOfferNotFound   error
		networkConfigNotFound   error
	)

	SetPeeringPhases := func(conditions ...discoveryv1alpha1.PeeringConditionType) {
		Expect(retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			var fc discoveryv1alpha1.ForeignCluster
			Expect(cl.Get(ctx, types.NamespacedName{Name: foreignClusterName}, &fc)).To(Succeed())

			fc.Status.PeeringConditions = make([]discoveryv1alpha1.PeeringCondition, 0)
			for _, condition := range conditions {
				peeringconditionsutils.EnsureStatus(&fc, condition, discoveryv1alpha1.PeeringConditionStatusEstablished, "", "")
			}

			return cl.Status().Update(ctx, &fc)
		})).To(Succeed())
	}

	DisableNetworking := func() {
		foreignCluster.Spec.NetworkingEnabled = discoveryv1alpha1.NetworkingEnabledNo
	}

	RemoteRef := func(name string) types.NamespacedName {
		return types.NamespacedName{Name: name, Namespace: remoteNamespace}
	}

	GetRemoteResourceRequest := func() func() error {
		return func() error { return cl.Get(ctx, RemoteRef(resourceRequestName), &discoveryv1alpha1.ResourceRequest{}) }
	}
	GetRemoteResourceOffer := func() func() error {
		return func() error { return cl.Get(ctx, RemoteRef(resourceOfferName), &sharingv1alpha1.ResourceOffer{}) }
	}
	GetRemoteNetworkConfig := func() func() error {
		return func() error { return cl.Get(ctx, RemoteRef(networkConfigName), &netv1alpha1.NetworkConfig{}) }
	}

	GetForeignClusterFinalizer := func() func() []string {
		return func() []string {
			var fc discoveryv1alpha1.ForeignCluster
			Expect(cl.Get(ctx, types.NamespacedName{Name: foreignClusterName}, &fc)).To(Succeed())
			return fc.GetFinalizers()
		}
	}

	BeforeEach(func() {
		foreignClusterNotFound = kerrors.NewNotFound(discoveryv1alpha1.ForeignClusterGroupResource, foreignClusterName)
		resourceRequestNotFound = kerrors.NewNotFound(discoveryv1alpha1.ResourceRequestGroupResource, resourceRequestName)
		resourceOfferNotFound = kerrors.NewNotFound(sharingv1alpha1.ResourceOfferGroupResource, resourceOfferName)
		networkConfigNotFound = kerrors.NewNotFound(netv1alpha1.NetworkConfigGroupResource, networkConfigName)

		labels := func() map[string]string {
			return map[string]string{
				consts.ReplicationRequestedLabel:   strconv.FormatBool(true),
				consts.ReplicationDestinationLabel: remoteCluster.ClusterID,
			}
		}

		foreignCluster = discoveryv1alpha1.ForeignCluster{
			ObjectMeta: metav1.ObjectMeta{Name: foreignClusterName},
			Spec: discoveryv1alpha1.ForeignClusterSpec{
				ClusterIdentity: remoteCluster,
				ForeignAuthURL:  authURL, OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
				IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto, InsecureSkipTLSVerify: pointer.Bool(true),
				NetworkingEnabled: discoveryv1alpha1.NetworkingEnabledYes,
			},
			Status: discoveryv1alpha1.ForeignClusterStatus{
				TenantNamespace: discoveryv1alpha1.TenantNamespaceType{Local: localNamespace, Remote: remoteNamespace}},
		}

		resourceRequest = discoveryv1alpha1.ResourceRequest{
			ObjectMeta: metav1.ObjectMeta{Name: resourceRequestName, Namespace: localNamespace, Labels: labels()},
			Spec: discoveryv1alpha1.ResourceRequestSpec{
				ClusterIdentity: remoteCluster,
				AuthURL:         authURL,
			},
		}

		resourceOffer = sharingv1alpha1.ResourceOffer{
			ObjectMeta: metav1.ObjectMeta{Name: resourceOfferName, Namespace: localNamespace, Labels: labels()},
			Spec:       sharingv1alpha1.ResourceOfferSpec{ClusterId: remoteCluster.ClusterID},
		}

		networkConfig = netv1alpha1.NetworkConfig{
			ObjectMeta: metav1.ObjectMeta{Name: networkConfigName, Namespace: localNamespace, Labels: labels()},
			Spec: netv1alpha1.NetworkConfigSpec{
				RemoteCluster: remoteCluster, PodCIDR: "1.1.1.0/24", ExternalCIDR: "1.1.2.0/24",
				EndpointIP: "1.1.1.1", BackendType: consts.DriverName, BackendConfig: map[string]string{},
			},
		}
	})

	JustBeforeEach(func() {
		statusCopy := foreignCluster.Status.DeepCopy()
		Expect(cl.Create(ctx, &foreignCluster)).To(Succeed())
		foreignCluster.Status = *statusCopy
		Expect(cl.Status().Update(ctx, &foreignCluster)).To(Succeed())
		Expect(cl.Create(ctx, &resourceRequest)).To(Succeed())
		Expect(cl.Create(ctx, &resourceOffer)).To(Succeed())
		Expect(cl.Create(ctx, &networkConfig)).To(Succeed())
	})

	JustAfterEach(func() {
		Expect(cl.DeleteAllOf(ctx, &discoveryv1alpha1.ResourceRequest{}, client.InNamespace(localNamespace))).To(Succeed())
		Expect(cl.DeleteAllOf(ctx, &sharingv1alpha1.ResourceOffer{}, client.InNamespace(localNamespace))).To(Succeed())
		Expect(cl.DeleteAllOf(ctx, &netv1alpha1.NetworkConfig{}, client.InNamespace(localNamespace))).To(Succeed())
		Expect(cl.DeleteAllOf(ctx, &discoveryv1alpha1.ForeignCluster{})).To(Succeed())

		// Ensure the finalizer has been removed correctly
		Eventually(func() error { return cl.Get(ctx, types.NamespacedName{Name: foreignClusterName}, &foreignCluster) }).
			Should(MatchError(foreignClusterNotFound))

		// Ensure all remote resources have been recollected
		Expect(GetRemoteResourceRequest()()).To(MatchError(resourceRequestNotFound))
		Expect(GetRemoteResourceOffer()()).To(MatchError(resourceOfferNotFound))
		Expect(GetRemoteNetworkConfig()()).To(MatchError(networkConfigNotFound))
	})

	Context("replication tests by phase with networking", func() {
		When("the peering phase is none", func() {
			It("Should replicate no resources", func() {
				Consistently(GetForeignClusterFinalizer()).ShouldNot(ContainElement("crdReplicator.liqo.io"))
				Consistently(GetRemoteResourceRequest()).Should(MatchError(resourceRequestNotFound))
				Consistently(GetRemoteResourceOffer()).Should(MatchError(resourceOfferNotFound))
				Consistently(GetRemoteNetworkConfig()).Should(MatchError(networkConfigNotFound))
			})
		})

		When("the peering phase is authenticated", func() {
			JustBeforeEach(func() { SetPeeringPhases(discoveryv1alpha1.AuthenticationStatusCondition) })
			It("Should replicate only the resource request", func() {
				Eventually(GetForeignClusterFinalizer()).Should(ContainElement("crdReplicator.liqo.io"))
				Eventually(GetRemoteResourceRequest()).Should(Succeed())
				Consistently(GetRemoteResourceOffer()).Should(MatchError(resourceOfferNotFound))
				Consistently(GetRemoteNetworkConfig()).Should(MatchError(networkConfigNotFound))
			})
		})

		When("the peering phase is outgoing", func() {
			JustBeforeEach(func() {
				SetPeeringPhases(discoveryv1alpha1.AuthenticationStatusCondition, discoveryv1alpha1.OutgoingPeeringCondition)
			})
			It("Should replicate the resource request and the network config", func() {
				Eventually(GetForeignClusterFinalizer()).Should(ContainElement("crdReplicator.liqo.io"))
				Eventually(GetRemoteResourceRequest()).Should(Succeed())
				Eventually(GetRemoteNetworkConfig()).Should(Succeed())
				Consistently(GetRemoteResourceOffer()).Should(MatchError(resourceOfferNotFound))
			})
		})

		When("the peering phase is incoming", func() {
			JustBeforeEach(func() {
				SetPeeringPhases(discoveryv1alpha1.AuthenticationStatusCondition, discoveryv1alpha1.IncomingPeeringCondition)
			})
			It("Should replicate all resources", func() {
				Eventually(GetForeignClusterFinalizer()).Should(ContainElement("crdReplicator.liqo.io"))
				Eventually(GetRemoteResourceRequest()).Should(Succeed())
				Eventually(GetRemoteResourceOffer()).Should(Succeed())
				Eventually(GetRemoteNetworkConfig()).Should(Succeed())
			})
		})
	})

	Context("replication tests by phase without networking", func() {
		BeforeEach(func() { DisableNetworking() })

		When("the peering phase is none", func() {

			It("Should replicate no resources", func() {
				Consistently(GetForeignClusterFinalizer()).ShouldNot(ContainElement("crdReplicator.liqo.io"))
				Consistently(GetRemoteResourceRequest()).Should(MatchError(resourceRequestNotFound))
				Consistently(GetRemoteResourceOffer()).Should(MatchError(resourceOfferNotFound))
				Consistently(GetRemoteNetworkConfig()).Should(MatchError(networkConfigNotFound))
			})
		})

		When("the peering phase is authenticated", func() {
			JustBeforeEach(func() {
				SetPeeringPhases(discoveryv1alpha1.AuthenticationStatusCondition)
			})
			It("Should replicate only the resource request", func() {
				Eventually(GetForeignClusterFinalizer()).Should(ContainElement("crdReplicator.liqo.io"))
				Eventually(GetRemoteResourceRequest()).Should(Succeed())
				Consistently(GetRemoteResourceOffer()).Should(MatchError(resourceOfferNotFound))
				Consistently(GetRemoteNetworkConfig()).Should(MatchError(networkConfigNotFound))
			})
		})

		When("the peering phase is outgoing", func() {
			JustBeforeEach(func() {
				SetPeeringPhases(discoveryv1alpha1.AuthenticationStatusCondition, discoveryv1alpha1.OutgoingPeeringCondition)
			})
			It("Should replicate the resource request", func() {
				Eventually(GetForeignClusterFinalizer()).Should(ContainElement("crdReplicator.liqo.io"))
				Eventually(GetRemoteResourceRequest()).Should(Succeed())
				Consistently(GetRemoteNetworkConfig()).Should(MatchError(networkConfigNotFound))
				Consistently(GetRemoteResourceOffer()).Should(MatchError(resourceOfferNotFound))
			})
		})

		When("the peering phase is incoming", func() {
			JustBeforeEach(func() {
				SetPeeringPhases(discoveryv1alpha1.AuthenticationStatusCondition, discoveryv1alpha1.IncomingPeeringCondition)
			})
			It("Should replicate all resources but not the network config", func() {
				Eventually(GetForeignClusterFinalizer()).Should(ContainElement("crdReplicator.liqo.io"))
				Eventually(GetRemoteResourceRequest()).Should(Succeed())
				Eventually(GetRemoteResourceOffer()).Should(Succeed())
				Consistently(GetRemoteNetworkConfig()).Should(MatchError(networkConfigNotFound))
			})
		})
	})
})

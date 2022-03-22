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

package status

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	netv1 "github.com/liqotech/liqo/apis/net/v1alpha1"
)

var _ = Describe("Remoteinfo", func() {
	const (
		rootTitle    = "Remote Clusters Information"
		namespace    = "liqo"
		clusterID1   = "e126183a-5445-404f-8802-fdd5ed64b4ec"
		clusterName1 = "cluster1"
		clusterID2   = "d945afca-3a15-45ef-a472-41f1803592d1"
		clusterName2 = "cluster2"
	)
	var (
		clientBuilder fake.ClientBuilder
		rootNode      = newRootInfoNode(rootTitle)
		ric           *RemoteInfoChecker
	)

	BeforeEach(func() {
		ctx = context.Background()
		_ = netv1.AddToScheme(scheme.Scheme)
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
	})

	Context("Creating a new remoteInfoChecker", func() {
		JustBeforeEach(func() {
			ric = newRemoteInfoChecker(namespace, &[]string{}, &[]string{}, clientBuilder.Build())
		})
		It("should return a valid remoteInfoChecker", func() {
			ricTest := &RemoteInfoChecker{
				client:             clientBuilder.Build(),
				namespace:          namespace,
				clusterNameFilter:  &[]string{},
				clusterIDFilter:    &[]string{},
				errors:             false,
				collectionErrors:   nil,
				rootRemoteInfoNode: rootNode,
			}
			Expect(*ric).To(Equal(*ricTest))
		})
	})

	Context("Checking if a cluster ID or a cluster name are included between filters", func() {
		Context("Checking cluster ID", func() {
			BeforeEach(func() {
				ric.clusterIDFilter = &[]string{"cluster-id-1", "cluster-id-2"}
			})
			When("Cluster ID is included", func() {
				It("should return true", func() {
					Expect(ric.argsFilterCheck("cluster-id-1", "")).To(BeTrue())
				})
			})
			When("Cluster ID is not included", func() {
				It("should return false", func() {
					Expect(ric.argsFilterCheck("cluster-id-3", "")).To(BeFalse())
				})

			})
		})
		Context("Checking cluster name", func() {
			BeforeEach(func() {
				ric.clusterNameFilter = &[]string{"cluster-name-1", "cluster-name-2"}
			})
			When("Cluster name is included", func() {
				It("should return true", func() {
					Expect(ric.argsFilterCheck("", "cluster-name-1")).To(BeTrue())
				})
			})
			When("Cluster name is not included", func() {
				It("should return false", func() {
					Expect(ric.argsFilterCheck("", "cluster-name-3")).To(BeFalse())
				})

			})
		})

	})

	Context("Getting Collect() result", func() {
		var infoNode InfoNode
		BeforeEach(func() {
			clientBuilder.WithObjects(
				&v1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind: "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name": "clusterid-configmap",
						},
						Namespace: namespace,
					},
					Immutable: new(bool),
					Data: map[string]string{
						"CLUSTER_ID":   clusterID1,
						"CLUSTER_NAME": clusterName1,
					},
					BinaryData: map[string][]byte{},
				},
				&netv1.NetworkConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: "NetworkConfig",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterName1,
						Labels: map[string]string{
							"liqo.io/originID":    clusterID2,
							"liqo.io/remoteID":    clusterID1,
							"liqo.io/replication": "false",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: clusterName2,
							},
						},
					},
					Spec: netv1.NetworkConfigSpec{
						PodCIDR:      "10.200.0.0/16",
						ExternalCIDR: "10.201.0.0/16",
					},
					Status: netv1.NetworkConfigStatus{
						PodCIDRNAT:      "10.202.0.0/16",
						ExternalCIDRNAT: "10.203.0.0/16",
					},
				},
				&netv1.NetworkConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: "NetworkConfig",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterName2,
						Labels: map[string]string{
							"liqo.io/remoteID":    clusterID2,
							"liqo.io/replication": "true",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: clusterName2,
							},
						},
					},
					Spec: netv1.NetworkConfigSpec{
						PodCIDR:      "10.200.0.0/16",
						ExternalCIDR: "10.201.0.0/16",
					},
					Status: netv1.NetworkConfigStatus{
						PodCIDRNAT:      "10.202.0.0/16",
						ExternalCIDRNAT: "10.203.0.0/16",
					},
				},
				&netv1.TunnelEndpoint{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"clusterID": clusterID2,
						},
					},
					Status: netv1.TunnelEndpointStatus{
						GatewayIP: "172.18.0.2",
						Connection: netv1.Connection{
							PeerConfiguration: map[string]string{
								"endpointIP": "172.18.0.3",
							},
						},
					},
				},
			)
			infoNode = InfoNode{}
			infoNode.title = rootTitle
			clusterSection := infoNode.addSectionToNode(clusterName2, "")
			local := clusterSection.addSectionToNode("Local Network Configuration", "")
			remote := clusterSection.addSectionToNode("Remote Network Configuration", "")
			originalLocal := local.addSectionToNode("Original Network Configuration", "Spec")
			remappedLocal := local.addSectionToNode(
				"Remapped Network Configuration", fmt.Sprintf("Status: how %s's CIDRs has been remapped by %s", clusterName1, clusterName2),
			)
			originalRemote := remote.addSectionToNode("Original Network Configuration", "Spec")
			remappedRemote := remote.addSectionToNode(
				"Remapped Network Configuration", fmt.Sprintf("Status: how %s remapped %s's CIDRs", clusterName1, clusterName2),
			)
			originalLocal.addDataToNode("Pod CIDR", "10.200.0.0/16")
			originalLocal.addDataToNode("External CIDR", "10.201.0.0/16")
			remappedLocal.addDataToNode("Pod CIDR", "10.202.0.0/16")
			remappedLocal.addDataToNode("External CIDR", "10.203.0.0/16")
			originalRemote.addDataToNode("Pod CIDR", "10.200.0.0/16")
			originalRemote.addDataToNode("External CIDR", "10.201.0.0/16")
			remappedRemote.addDataToNode("Pod CIDR", "10.202.0.0/16")
			remappedRemote.addDataToNode("External CIDR", "10.203.0.0/16")
			tunnelEndpoint := clusterSection.addSectionToNode("Tunnel Endpoint", "")
			tunnelEndpoint.addDataToNode("Gateway IP", "172.18.0.2")
			tunnelEndpoint.addDataToNode("Endpoint IP", "172.18.0.3")
		})

		When("There aren't filters", func() {
			JustBeforeEach(func() {
				ric = newRemoteInfoChecker(namespace, &[]string{}, &[]string{}, clientBuilder.Build())
			})
			It("should return a valid tree", func() {
				_ = ric.Collect(ctx)
				Expect(ric.rootRemoteInfoNode).To(Equal(infoNode))
			})
		})
		When("There is a filter on cluster ID", func() {
			When("Filtered ID is contained in filtered ID list", func() {
				JustBeforeEach(func() {
					ric = newRemoteInfoChecker(namespace, &[]string{}, &[]string{clusterID2}, clientBuilder.Build())
				})
				It("should return a valid tree", func() {
					_ = ric.Collect(ctx)
					Expect(ric.rootRemoteInfoNode).To(Equal(infoNode))
				})
			})
			When("Filtered ID is not contained in filtered ID list", func() {
				JustBeforeEach(func() {
					ric = newRemoteInfoChecker(namespace, &[]string{}, &[]string{"invalid filter"}, clientBuilder.Build())
				})
				It("should return a void tree", func() {
					_ = ric.Collect(ctx)
					Expect(ric.rootRemoteInfoNode.nextNodes).To(BeEmpty())
				})
			})

		})
		When("There is a filter on cluster name", func() {
			When("Filtered name is contained in filtered name list", func() {
				JustBeforeEach(func() {
					ric = newRemoteInfoChecker(namespace, &[]string{clusterName2}, &[]string{}, clientBuilder.Build())
				})
				It("should return a valid tree", func() {
					_ = ric.Collect(ctx)
					Expect(ric.rootRemoteInfoNode).To(Equal(infoNode))
				})
			})
			When("Filtered name is not contained in filtered name list", func() {
				JustBeforeEach(func() {
					ric = newRemoteInfoChecker(namespace, &[]string{"invalid filter"}, &[]string{}, clientBuilder.Build())
				})
				It("should return a void tree", func() {
					_ = ric.Collect(ctx)
					Expect(ric.rootRemoteInfoNode.nextNodes).To(BeEmpty())
				})
			})

		})
	})
})

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

package multiplevirtualnodes

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 3
	// testName is the name of this E2E test.
	testName = "MULTIPLE_VIRTUALNODES"
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var (
	ctx         = context.Background()
	testContext = tester.GetTester(ctx)
	interval    = config.Interval
	timeout     = config.Timeout

	testResSliceLabel = "liqo.io/test-resourceslice"

	testResourceRequirement = func() labels.Requirement {
		req, err := labels.NewRequirement(testResSliceLabel, selection.Exists, []string{})
		Expect(err).To(Not(HaveOccurred()))
		return *req
	}

	createTestResourceSlice = func(cl client.Client, name, tenantNs string, providerClusterID liqov1beta1.ClusterID, createVirtualNode bool) {
		resSlice := forge.ResourceSlice(name, tenantNs)
		if resSlice.Labels == nil {
			resSlice.Labels = map[string]string{}
		}
		resSlice.Labels[testResSliceLabel] = "true"
		Expect(forge.MutateResourceSlice(resSlice, providerClusterID, &forge.ResourceSliceOptions{
			Class: authv1beta1.ResourceSliceClassDefault,
		}, createVirtualNode)).To(Succeed())
		Expect(cl.Create(ctx, resSlice)).To(Succeed())
	}

	deleteTestResourceSlices = func(cl client.Client) {
		resSlices, err := getters.ListResourceSlicesByLabel(ctx, cl,
			corev1.NamespaceAll, liqolabels.LocalLabelSelector().Add(testResourceRequirement()))
		Expect(err).To(Not(HaveOccurred()))
		for j := range resSlices {
			Expect(client.IgnoreNotFound(cl.Delete(ctx, &resSlices[j]))).To(Succeed())
		}
	}

	checkCleanUp = func(cl client.Client, role liqov1beta1.RoleType, numPeeredProviders int) error {
		resSlices, err := getters.ListResourceSlicesByLabel(ctx, cl,
			corev1.NamespaceAll, labels.NewSelector().Add(testResourceRequirement()))
		if err != nil {
			return err
		}
		if len(resSlices) != 0 {
			return fmt.Errorf("ResourceSlices not deleted yet")
		}

		// Number of expected VirtualNodes/Nodes after cleanup.
		var expectedNum int
		switch role {
		case liqov1beta1.ConsumerRole:
			expectedNum = numPeeredProviders
		default:
			expectedNum = 0
		}

		// Number of virtual nodes should equal the number of peered providers.
		vNodes, err := getters.ListVirtualNodesByLabels(ctx, cl, labels.Everything())
		if err != nil {
			return err
		}
		if len(vNodes.Items) != expectedNum {
			return fmt.Errorf("Found %d VirtualNodes after cleanup, expected %d", len(vNodes.Items), expectedNum)
		}

		nodes, err := getters.ListLiqoNodes(ctx, cl)
		if err != nil {
			return err
		}
		if len(nodes.Items) != expectedNum {
			return fmt.Errorf("Found %d Liqo nodes after cleanup, expected %d", len(nodes.Items), expectedNum)
		}

		return nil
	}
)

var _ = Describe("Liqo E2E", func() {
	Context("Multiple VirtualNodes and ResourceSlices", func() {
		AfterEach(func() {
			for i := range testContext.Clusters {
				deleteTestResourceSlices(testContext.Clusters[i].ControllerClient)
			}

			for i := range testContext.Clusters {
				Eventually(func() error {
					return checkCleanUp(testContext.Clusters[i].ControllerClient,
						testContext.Clusters[i].Role, testContext.Clusters[i].NumPeeredProviders)
				}, timeout, interval).Should(Succeed())
			}
		})

		When("Peerings are established", func() {
			It("Should have the right number of resourceslices", func() {
				for i := range testContext.Clusters {
					cluster := testContext.Clusters[i]
					switch cluster.Role {
					case liqov1beta1.ConsumerRole:
						resSlices, err := getters.ListResourceSlicesByLabel(ctx, cluster.ControllerClient,
							corev1.NamespaceAll, liqolabels.LocalLabelSelector())
						Expect(err).To(Not(HaveOccurred()))
						Expect(len(resSlices)).To(Equal(cluster.NumPeeredProviders))
					case liqov1beta1.ProviderRole:
						resSlices, err := getters.ListResourceSlicesByLabel(ctx, cluster.ControllerClient,
							corev1.NamespaceAll, liqolabels.RemoteLabelSelector())
						Expect(err).To(Not(HaveOccurred()))
						Expect(len(resSlices)).To(Equal(cluster.NumPeeredConsumers))
					default:
						resSlices, err := getters.ListResourceSlicesByLabel(ctx, cluster.ControllerClient, corev1.NamespaceAll, labels.Everything())
						Expect(err).To(Not(HaveOccurred()))
						Expect(len(resSlices)).To(BeZero())
					}
				}
			})
		})

		When("Adding multiple ResourceSlices for each providers on multiple providers", func() {

			It("Should replicate the ResourceSlices to the provider, and create VirtualNodes and Nodes", func() {
				// On every consumer cluster, create a ResourceSlice for each provider cluster.
				for i := range testContext.Clusters {
					if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
						consumer := testContext.Clusters[i]
						nsManager := tenantnamespace.NewManager(consumer.NativeClient, consumer.ControllerClient.Scheme())
						for j := range testContext.Clusters {
							if testContext.Clusters[j].Role == liqov1beta1.ProviderRole {
								provider := testContext.Clusters[j]
								tenantNs, err := nsManager.GetNamespace(ctx, provider.Cluster)
								Expect(err).To(Not(HaveOccurred()))
								createTestResourceSlice(consumer.ControllerClient,
									fmt.Sprintf("rs-test-%s", provider.Cluster), tenantNs.Name, provider.Cluster, true)
							}
						}
					}
				}

				// Test that every ResourceSlices, VirtualNodes and Nodes are created.
				for i := range testContext.Clusters {
					switch testContext.Clusters[i].Role {
					// CONSUMERS
					case liqov1beta1.ConsumerRole:
						consumer := testContext.Clusters[i]

						var resSlices []authv1beta1.ResourceSlice

						// List all ResourceSlices created by the consumer
						// (filtering out the resourceslices from the original peering)
						// and test if the number of ResourceSlices is correct.
						Eventually(func() error {
							var err error
							resSlices, err = getters.ListResourceSlicesByLabel(ctx, consumer.ControllerClient,
								corev1.NamespaceAll, liqolabels.LocalLabelSelector().Add(testResourceRequirement()))
							if err != nil {
								return err
							}
							if len(resSlices) != consumer.NumPeeredProviders {
								return fmt.Errorf("Found %d ResourceSlices, expected %d", len(resSlices), consumer.NumPeeredProviders)
							}
							return nil
						}, timeout, interval).Should(Succeed())

						for j := range resSlices {
							var vNodes *offloadingv1beta1.VirtualNodeList

							// Test if every resourceSlice has the associated VirtualNode.
							Eventually(func() error {
								var err error
								vNodes, err = getters.ListVirtualNodesByLabels(ctx, consumer.ControllerClient,
									labels.Set{consts.ResourceSliceNameLabelKey: resSlices[j].Name}.AsSelector())
								if err != nil {
									return err
								}
								if len(vNodes.Items) != 1 {
									return fmt.Errorf("Found %d VirtualNodes for ResourceSlice %s, expected 1", len(vNodes.Items), resSlices[j].Name)
								}
								return nil
							}, timeout, interval).Should(Succeed())

							// Test if every VirtualNode has the associated Node and that it is Ready.
							Eventually(func() error {
								node, err := getters.GetNodeFromVirtualNode(ctx, consumer.ControllerClient, &vNodes.Items[0])
								if err != nil {
									return fmt.Errorf("Unable to get Node for VirtualNode %s: %w", vNodes.Items[0].Name, err)
								}
								if !utils.IsNodeReady(node) {
									return fmt.Errorf("Node %s is not ready", node.Name)
								}
								return nil
							}, timeout, interval).Should(Succeed())
						}

					// PROVIDERS
					case liqov1beta1.ProviderRole:
						provider := testContext.Clusters[i]

						var resSlices []authv1beta1.ResourceSlice

						// List all ResourceSlices replicated on the provider
						// (filtering out the resourceslices from the original peering)
						// and test if the number of ResourceSlices is correct.
						Eventually(func() error {
							var err error
							resSlices, err = getters.ListResourceSlicesByLabel(ctx, provider.ControllerClient,
								corev1.NamespaceAll, liqolabels.RemoteLabelSelector().Add(testResourceRequirement()))
							if err != nil {
								return err
							}
							if len(resSlices) != provider.NumPeeredConsumers {
								return fmt.Errorf("Found %d ResourceSlices, expected %d", len(resSlices), provider.NumPeeredConsumers)
							}
							return nil
						}, timeout, interval).Should(Succeed())

						// Test that a VirtualNode has not been created for the remote ResourceSlice.
						for j := range resSlices {
							Eventually(func() error {
								vNodes, err := getters.ListVirtualNodesByLabels(ctx, provider.ControllerClient,
									labels.Set{consts.ResourceSliceNameLabelKey: resSlices[j].Name}.AsSelector())
								if err != nil {
									return err
								}
								if len(vNodes.Items) != 0 {
									return fmt.Errorf("Found %d VirtualNodes for ResourceSlice %s, expected 0", len(vNodes.Items), resSlices[j].Name)
								}
								return nil
							}, timeout, interval).Should(Succeed())
						}

					default:
						// Do nothing.
					}
				}
			})
		})
	})
})

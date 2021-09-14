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

package liqodeploymentctrl

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	testutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/liqo-deployment-controller/testutils"
)

const (
	// Some tests do not have the necessity to create real namespaces and resources, they are not really deployed
	// in the cluster.
	mockName      = "test"
	mockNamespace = "ns-test"
	timeout       = time.Second * 10
	interval      = time.Millisecond * 250
)

var _ = Describe("Test for LiqoDeployment reconciler", func() {

	Context("1 - Testing the core functions of the controller", func() {

		Context("1.1 - Testing against the getClusterFilter() function", func() {

			It("Checking that the cluster Selector is equal to the NamespaceOffloading Selector when the "+
				"LiqoDeployment has no Selector specified", func() {
				namespaceOff := testutils.GetNamespaceOffloading(testutils.GetNodeSelector(testutils.NoProviderC), mockNamespace)
				ldp := testutils.GetLiqoDeployment(testutils.GetNodeSelector(testutils.EmptySelector),
					testutils.GetGenerationLabels(testutils.EmptyGenerationLabels), mockName, mockNamespace, false)
				Expect(getClusterFilter(namespaceOff, ldp)).To(Equal(testutils.GetNodeSelector(testutils.NoProviderC)))
			})

			It("Checking that the cluster Selector is equal to the NamespaceOffloading Selector merged "+
				"with the LiqoDeployment Selector when it is specified", func() {
				namespaceOff := testutils.GetNamespaceOffloading(testutils.GetNodeSelector(testutils.NoProviderC), mockNamespace)
				ldp := testutils.GetLiqoDeployment(testutils.GetNodeSelector(testutils.NoRegionB),
					testutils.GetGenerationLabels(testutils.EmptyGenerationLabels), mockName, mockNamespace, false)
				Expect(getClusterFilter(namespaceOff, ldp)).To(Equal(testutils.GetNodeSelector(testutils.MergedSelector)))
			})

		})

		Context("1.2 - Testing against the checkCompatibleVirtualNodes() function", func() {
			When("Checking the selected cluster maps provide by the checkCompatibleVirtualNodes() function", func() {
				// These are the already merged clusterSelector, NamespaceOffloading Selector + LiqoDeployment Selector.
				// In this case the mergedSelector selects all the virtual nodes.
				selector1 := testutils.GetNodeSelector(testutils.DefaultSelector)
				// In this case the mergedSelector is equal to the NoProvideC Selector.
				selector2 := testutils.GetNodeSelector(testutils.NoProviderC)

				// This LiqoDeployment will replicate deployment with provider granularity.
				ldp1 := testutils.GetLiqoDeployment(testutils.GetNodeSelector(testutils.EmptySelector),
					testutils.GetGenerationLabels(testutils.ProviderGenerationLabels), mockName, mockNamespace, false)

				// This LiqoDeployment will replicate deployment with provider and region granularity.
				ldp2 := testutils.GetLiqoDeployment(testutils.GetNodeSelector(testutils.EmptySelector),
					testutils.GetGenerationLabels(testutils.RegionAndProviderGenerationLabels), mockName, mockNamespace, false)

				DescribeTable("Deployment replicated with different granularity",
					func(ldp *offv1alpha1.LiqoDeployment, ns *corev1.NodeSelector, expectedMap map[string]struct{}) {
						availableCombinationsMap := map[string]struct{}{}
						Expect(controller.checkCompatibleVirtualNodes(ctx, ns, ldp, availableCombinationsMap)).ToNot(HaveOccurred())
						Expect(availableCombinationsMap).To(Equal(expectedMap))
					},
					Entry("Provider granularity with all virtual nodes", ldp1, &selector1,
						testutils.GetAvailableCombinations(testutils.ProviderSelectionWithoutSelector)),
					Entry("Provider granularity without the virtual node 3", ldp1, &selector2,
						testutils.GetAvailableCombinations(testutils.ProviderSelectionWithSelector)),
					Entry("Region and Provider granularity with all virtual nodes", ldp2, &selector1,
						testutils.GetAvailableCombinations(testutils.RegionAndProviderSelectionWithoutSelector)),
					Entry("Region and Provider granularity without the virtual node 3", ldp2, &selector2,
						testutils.GetAvailableCombinations(testutils.RegionAndProviderSelectionWithSelector)),
				)
			})

			It("Checking the LDP annotation in case of wrong selector", func() {
				selector := testutils.GetNodeSelector(testutils.WrongSelector)
				ldpWrongSelectorName := "wrong-node-selector"
				// This ldp must be created because the controller will use it
				ldpWrongSelector, err := testutils.CreateLiqoDeployment(ctx, k8sClient,
					selector, testutils.GetGenerationLabels(testutils.EmptyGenerationLabels),
					ldpWrongSelectorName, namespaceContext1, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(ldpWrongSelector).NotTo(BeNil())

				availableCombinationsMap := map[string]struct{}{}
				Expect(controller.checkCompatibleVirtualNodes(ctx, &selector, ldpWrongSelector, availableCombinationsMap)).ToNot(Succeed())
				_, ok := ldpWrongSelector.Annotations[errorAnnotationKey]
				Expect(ok).To(BeTrue())
			})

		})

		Context("1.3 - Testing against the enforceDeploymentReplicas() function", func() {
			It("Checking the number of replicated deployment, their labels, and their NodeSelector", func() {
				// This ldp must be created in the cluster to set the ownerReference on the deployment
				ldpName := "nginx-replicator"
				ldp, err := testutils.CreateLiqoDeployment(ctx, k8sClient, testutils.GetNodeSelector(testutils.EmptySelector),
					testutils.GetGenerationLabels(testutils.ProviderGenerationLabels), ldpName, namespaceContext1, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(ldp).NotTo(BeNil())
				availableCombinationsMap := testutils.GetAvailableCombinations(testutils.ProviderSelectionWithoutSelector)
				controller.enforceDeploymentReplicas(ctx, ldp, availableCombinationsMap)

				// Checking the deploymet presence for the various provider labels
				for labelsString := range availableCombinationsMap {
					generationLabelsString := strings.Split(labelsString, labelSeparator)
					generationLabelsString = generationLabelsString[1:]
					generationLabels := map[string]string{}

					tmp := strings.Split(generationLabelsString[0], keyValueSeparator)
					generationLabels[tmp[0]] = tmp[1]
					By(fmt.Sprintf("Checking the deployment for the '%s' label with value '%s'", tmp[0], tmp[1]))

					deploymentList := &appsv1.DeploymentList{}
					Eventually(func() bool {
						err = k8sClient.List(ctx, deploymentList, client.MatchingLabels{replicatorLabel: ldp.Name}, &client.ListOptions{
							LabelSelector: labels.SelectorFromSet(generationLabels),
							Namespace:     ldp.Namespace,
						})
						if err != nil {
							return false
						}
						return len(deploymentList.Items) == 1
					}, timeout, interval).Should(BeTrue())

					By("Checking that the MockAffinity is mutated")
					nodeSelector := corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      testutils.ProviderLabel,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{generationLabels[testutils.ProviderLabel]},
								},
								{
									Key:      testutils.ProviderLabel,
									Operator: corev1.NodeSelectorOpNotIn,
									Values:   []string{testutils.ProviderC},
								},
							},
						},
					}}

					Expect(*deploymentList.Items[0].Spec.Template.Spec.Affinity.
						NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).To(Equal(nodeSelector))

					By("Checking that the PodAntiAffinity is preserved")
					Expect(deploymentList.Items[0].Spec.Template.Spec.Affinity.
						PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).To(Equal(testutils.GetPodAntiAffinity(testutils.MockAffinity)))

					By("Checking the generation labels presence")
					generationLabels[replicatorLabel] = ldp.Name
					for key, value := range generationLabels {
						Expect(deploymentList.Items[0].Labels).To(HaveKeyWithValue(key, value))
						Expect(deploymentList.Items[0].Spec.Selector.MatchLabels).To(HaveKeyWithValue(replicatorLabel, ldp.Name))
						Expect(deploymentList.Items[0].Spec.Template.Labels).To(HaveKeyWithValue(key, value))
					}

				}

			})
		})

		Context("1.4 - Testing against the searchUnnecessaryDeploymentReplicas() function", func() {
			It("Checking if the not necessary deployment are really deleted", func() {
				ldp := testutils.GetLiqoDeployment(testutils.GetNodeSelector(testutils.EmptySelector),
					testutils.GetGenerationLabels(testutils.ProviderGenerationLabels), mockName, namespaceContext1, false)
				mockDeploymentName1 := "deployment-1"
				mockDeploymentName2 := "deployment-2"
				notRequiredDeploymentName := "deployment-not-desired"
				requiredDeploymentName := "deployment-desired"

				ldp.Status.CurrentDeployment = map[string]offv1alpha1.GeneratedDeploymentStatus{}
				// Mock entries, the are no real deployments that correspond to these entries, so they
				// must be removed.
				ldp.Status.CurrentDeployment[mockDeploymentName1] = offv1alpha1.GeneratedDeploymentStatus{}
				ldp.Status.CurrentDeployment[mockDeploymentName2] = offv1alpha1.GeneratedDeploymentStatus{}

				// This is a real existing deployment, but it is not required, so it must be deleted.
				ldp.Status.CurrentDeployment[notRequiredDeploymentName] = offv1alpha1.GeneratedDeploymentStatus{
					GenerationLabelsValues: map[string]string{
						testutils.RegionLabel: testutils.RegionB,
					},
				}

				// This is a real existing deployment and it is required.
				ldp.Status.CurrentDeployment[requiredDeploymentName] = offv1alpha1.GeneratedDeploymentStatus{
					GenerationLabelsValues: map[string]string{
						testutils.ProviderLabel: testutils.ProviderB,
					},
				}

				_, err := testutils.CreateNewDeployment(ctx, k8sClient, nil, namespaceContext1, notRequiredDeploymentName)
				Expect(err).NotTo(HaveOccurred())

				_, err = testutils.CreateNewDeployment(ctx, k8sClient, nil, namespaceContext1, requiredDeploymentName)
				Expect(err).NotTo(HaveOccurred())

				By("Checking the deployments existence")
				Eventually(func() error {
					deployment := &appsv1.Deployment{}
					if err = k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceContext1,
						Name:      notRequiredDeploymentName,
					}, deployment); err != nil {
						return err
					}
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceContext1,
						Name:      requiredDeploymentName,
					}, deployment)
				}, timeout, interval).Should(BeNil())

				availableCombinationsMap := testutils.GetAvailableCombinations(testutils.ProviderSelectionWithoutSelector)
				controller.searchUnnecessaryDeploymentReplicas(ctx, ldp, availableCombinationsMap)

				// The deployment with provider label equal to B is required, so the entry must be deleted.
				key := fmt.Sprintf("%s%s%s%s", testutils.LabelSeparator,
					testutils.ProviderLabel, testutils.KeyValueSeparator, testutils.ProviderB)
				Expect(availableCombinationsMap).ToNot(HaveKey(key))

				Expect(ldp.Status.CurrentDeployment).ToNot(HaveKey(mockDeploymentName1))
				Expect(ldp.Status.CurrentDeployment).ToNot(HaveKey(mockDeploymentName1))
				Expect(ldp.Status.CurrentDeployment).To(HaveKey(notRequiredDeploymentName))

				// This must have the deletion timestamp set
				Eventually(func() bool {
					deploymentNotRequired := &appsv1.Deployment{}
					err = k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceContext1,
						Name:      notRequiredDeploymentName,
					}, deploymentNotRequired)
					if apierrors.IsNotFound(err) {
						return true
					}
					return err == nil && !deploymentNotRequired.DeletionTimestamp.IsZero()
				}, timeout, interval).Should(BeTrue())

				// This deployment should not have the deletion timestamp set
				deploymentRequired := &appsv1.Deployment{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceContext1,
					Name:      requiredDeploymentName,
				}, deploymentRequired)
				Expect(err).ToNot(HaveOccurred())
				Expect(deploymentRequired.DeletionTimestamp.IsZero()).To(BeTrue())
			})

		})

	})

	Context("2 - Test some reconciliation workflows", func() {

		It("2.1 - Generating deployment replicas with cluster granularity and checking the LiqoDeployment "+
			"enforcement when a deployment replica is changed by users", func() {
			ldpName := "ldp-enforcement"
			ldp, err := testutils.CreateLiqoDeployment(ctx, k8sClient, testutils.GetNodeSelector(testutils.EmptySelector),
				testutils.GetGenerationLabels(testutils.EmptyGenerationLabels), ldpName, namespaceContext2, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(ldp).NotTo(BeNil())

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceContext2,
					Name:      ldpName,
				}, ldp)
			}, timeout, interval).Should(BeNil())

			By("Reconciliation on the right Liqo Deployment")
			_, err = controller.Reconcile(ctx, reconcile.Request{NamespacedName: struct {
				Namespace string
				Name      string
			}{Namespace: namespaceContext2, Name: ldpName}})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that the 3 required deployments are created")
			deploymentList := &appsv1.DeploymentList{}
			Eventually(func() bool {
				err = k8sClient.List(ctx, deploymentList, client.MatchingLabels{replicatorLabel: ldp.Name}, &client.ListOptions{
					Namespace: namespaceContext2,
				})
				return err == nil && len(deploymentList.Items) == 3
			}, timeout, interval).Should(BeTrue())

			By("Changing the first deployment")
			deploymentName := deploymentList.Items[0].Name
			mockKey := "mock-key"
			mockValue := "mock-value"
			deploymentList.Items[0].Spec.Template.Labels[mockKey] = mockValue
			Eventually(func() error {
				return k8sClient.Update(ctx, &deploymentList.Items[0])
			}, timeout, interval).Should(BeNil())

			By("Reconciliation on the right Liqo Deployment")
			_, err = controller.Reconcile(ctx, reconcile.Request{NamespacedName: struct {
				Namespace string
				Name      string
			}{Namespace: namespaceContext2, Name: ldpName}})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that the LiqoDeployment enforces template values")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespaceContext2}, deployment)
				if err != nil {
					return false
				}
				_, ok := deployment.Spec.Template.Labels[mockKey]
				return !ok
			}, timeout, interval).Should(BeTrue())
		})

		It("2.2 - Changing the LiqoDeployment granularity and observing how deployment replicas are updated", func() {
			ldpName := "ldp-granularity"
			ldp, err := testutils.CreateLiqoDeployment(ctx, k8sClient, testutils.GetNodeSelector(testutils.EmptySelector),
				testutils.GetGenerationLabels(testutils.RegionAndProviderSelectionWithSelector), ldpName, namespaceContext2, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(ldp).NotTo(BeNil())

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceContext2,
					Name:      ldpName,
				}, ldp)
			}, timeout, interval).Should(BeNil())

			By("Reconciliation on the right Liqo Deployment")
			_, err = controller.Reconcile(ctx, reconcile.Request{NamespacedName: struct {
				Namespace string
				Name      string
			}{Namespace: namespaceContext2, Name: ldpName}})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the name of the 3 deployments with region A, region B and region C")
			deploymentNames := map[string]string{}
			deploymentList := &appsv1.DeploymentList{}
			Eventually(func() bool {
				err = k8sClient.List(ctx, deploymentList, client.MatchingLabels{replicatorLabel: ldp.Name}, &client.ListOptions{
					Namespace: ldp.Namespace,
				})
				return err == nil && len(deploymentList.Items) == 3
			}, timeout, interval).Should(BeTrue())

			for _, deployment := range deploymentList.Items {
				deploymentNames[deployment.Labels[testutils.RegionLabel]] = deployment.Name
			}

			By("Changing the replication granularity to 'Region' and filtering out clusters with 'Provider=C'")
			Eventually(func() error {
				if err = k8sClient.Get(ctx, types.NamespacedName{Name: ldp.Name, Namespace: namespaceContext2}, ldp); err != nil {
					return err
				}
				original := ldp.DeepCopy()
				ldp.Spec.SelectedClusters = testutils.GetNodeSelector(testutils.NoProviderC)
				ldp.Spec.GenerationLabels = testutils.GetGenerationLabels(testutils.RegionGenerationLabels)
				return k8sClient.Patch(ctx, ldp, client.MergeFrom(original))
			}, timeout, interval).Should(BeNil())

			By("Checking the LiqoDeployment update")
			Eventually(func() bool {
				if err = k8sClient.Get(ctx, types.NamespacedName{Name: ldp.Name, Namespace: namespaceContext2}, ldp); err != nil {
					return false
				}
				return len(ldp.Spec.GenerationLabels) == 1
			}, timeout, interval).Should(BeTrue())

			By("Reconciliation on the right Liqo Deployment")
			Eventually(func() error {
				_, err = controller.Reconcile(ctx, reconcile.Request{NamespacedName: struct {
					Namespace string
					Name      string
				}{Namespace: namespaceContext2, Name: ldpName}})
				return err
			}, timeout, interval).Should(BeNil())

			By("Checking if the deployment replica with region A is correctly updated")
			deployment := &appsv1.Deployment{}
			Eventually(func() bool {
				if err = k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceContext2,
					Name:      deploymentNames[testutils.RegionA],
				}, deployment); err != nil {
					return false
				}
				value, region := deployment.Labels[testutils.RegionLabel]
				_, provider := deployment.Labels[testutils.ProviderLabel]
				return region && value == testutils.RegionA && !provider
			}, timeout*3, interval).Should(BeTrue())

			By("Checking if the deployment replica with region B is correctly updated")
			Eventually(func() bool {
				if err = k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceContext2,
					Name:      deploymentNames[testutils.RegionB],
				}, deployment); err != nil {
					return false
				}
				value, region := deployment.Labels[testutils.RegionLabel]
				_, provider := deployment.Labels[testutils.ProviderLabel]
				return region && value == testutils.RegionB && !provider
			}, timeout, interval).Should(BeTrue())

			By("Checking if the deployment replica with region C is correctly deleted")
			Eventually(func() bool {
				err = k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceContext2,
					Name:      deploymentNames[testutils.RegionC],
				}, deployment)
				if apierrors.IsNotFound(err) {
					return true
				}
				return err == nil && !deployment.DeletionTimestamp.IsZero()
			}, timeout, interval).Should(BeTrue())

		})

	})

})

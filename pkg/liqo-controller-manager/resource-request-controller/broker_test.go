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

package resourcerequestoperator

/*
import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils"
)

const BrokerNamespace = "broker-test"

var _ = Describe("ResourceRequest Operator", func() {
	Context("Broker", func() {
		var (
			createdResourceRequest *discoveryv1alpha1.ResourceRequest
			node1                  *corev1.Node
			node2                  *corev1.Node
			podWithoutLabel        *corev1.Pod
		)
		BeforeEach(func() {
			createdResourceRequest = CreateResourceRequest(ctx, ResourceRequestName, BrokerNamespace, cluster1, k8sClient)
			var err error
			node1, err = createNewNode(ctx, "test-node1", false, clientset)
			Expect(err).ToNot(HaveOccurred())
			node2, err = createNewNode(ctx, "test-node2", false, clientset)
			Expect(err).ToNot(HaveOccurred())
			podWithoutLabel, err = createNewPod(ctx, "test-pod-2", "", false, clientset)
			Expect(err).ToNot(HaveOccurred())

			resourceRequest := discoveryv1alpha1.ResourceRequest{}
			Eventually(func() []string {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      createdResourceRequest.Name,
					Namespace: createdResourceRequest.Namespace,
				}, &resourceRequest)
				if err != nil {
					return []string{}
				}
				return resourceRequest.Finalizers
			}, timeout, interval).Should(ContainElement(tenantFinalizer))
		})
		AfterEach(func() {
			err := k8sClient.DeleteAllOf(ctx, &discoveryv1alpha1.ResourceRequest{}, client.InNamespace(BrokerNamespace))
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.DeleteAllOf(ctx, &sharingv1alpha1.ResourceOffer{}, client.InNamespace(BrokerNamespace))
			Expect(err).ToNot(HaveOccurred())
			err = clientset.CoreV1().Pods("default").DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			err = clientset.CoreV1().Nodes().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			err = clientset.StorageV1().StorageClasses().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				var resourceRequest discoveryv1alpha1.ResourceRequest
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      createdResourceRequest.Name,
					Namespace: createdResourceRequest.Namespace,
				}, &resourceRequest)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
		When("a new virtual node is created", func() {
			It("is recognized", func() {
				virtualNode, err := createNewNode(ctx, "test-virtual-node", true, clientset)
				Expect(err).ToNot(HaveOccurred())
				Expect(utils.IsVirtualNode(virtualNode)).To(BeTrue())
			})
			It("does not change the resources", func() {
				podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)
				Eventually(func() bool {
					nodeList := []corev1.ResourceList{
						node2.Status.Allocatable,
						node1.Status.Allocatable,
					}
					podList := []corev1.ResourceList{
						podReq,
					}
					return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("a new virtual pod is created", func() {
			It("is recognized", func() {
				pod, err := createNewPod(ctx, "shadow-test", "", true, clientset)
				Expect(err).ToNot(HaveOccurred())
				Expect(isShadowPod(pod)).Should(BeTrue())
			})
			It("does not change the resources", func() {
				podReq1, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)
				By("Expected just normal pod resources")
				Eventually(func() bool {
					nodeList := []corev1.ResourceList{
						node2.Status.Allocatable,
						node1.Status.Allocatable,
					}
					podList := []corev1.ResourceList{
						podReq1,
					}
					return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("a node is not ready", func() {
			It("computes the resources correctly", func() {
				podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)
				var err error
				node1, err = setNodeReadyStatus(ctx, node1, false, clientset)
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() bool {
					resourcesRead := scaledMonitor.ReadResources(cluster1.ClusterID)
					for resourceName, quantity := range resourcesRead {
						toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
						toCheck.Sub(podReq[resourceName])
						ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
						if quantity.Cmp(toCheck) != 0 {
							return false
						}
					}
					return true
				}, timeout, interval).Should(BeTrue())

				Eventually(func() bool {
					nodeList := []corev1.ResourceList{
						node2.Status.Allocatable,
					}
					podList := []corev1.ResourceList{
						podReq,
					}
					return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("a node becomes ready", func() {
			It("updates the resources correctly", func() {
				var err error
				node1, err = setNodeReadyStatus(ctx, node1, true, clientset)
				Expect(err).ToNot(HaveOccurred())

				podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)
				Eventually(func() bool {
					nodeList := []corev1.ResourceList{
						node1.Status.Allocatable,
						node2.Status.Allocatable,
					}
					podList := []corev1.ResourceList{
						podReq,
					}
					return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					resourcesRead := scaledMonitor.ReadResources(cluster1.ClusterID)
					for resourceName, quantity := range resourcesRead {
						toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
						toCheck.Add(node1.Status.Allocatable[resourceName])
						toCheck.Sub(podReq[resourceName])
						ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
						if quantity.Cmp(toCheck) != 0 {
							return false
						}
					}
					return true
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("a node's resources change", func() {
			It("updates the resources advertised", func() {
				var err error
				podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)

				toUpdate := node1.Status.Allocatable.DeepCopy()
				for _, quantity := range toUpdate {
					quantity.Sub(*resource.NewQuantity(1, quantity.Format))
				}
				node1.Status.Allocatable = toUpdate.DeepCopy()
				node1, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node1, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() bool {
					resourcesRead := scaledMonitor.ReadResources(cluster1.ClusterID)
					for resourceName, quantity := range resourcesRead {
						toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
						toCheck.Add(node1.Status.Allocatable[resourceName])
						toCheck.Sub(podReq[resourceName])
						ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
						if quantity.Cmp(toCheck) != 0 {
							return false
						}
					}
					return true
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					nodeList := []corev1.ResourceList{
						node2.Status.Allocatable,
						node1.Status.Allocatable,
					}
					podList := []corev1.ResourceList{
						podReq,
					}
					return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("a node is deleted", func() {
			It("updates the resources advertised", func() {
				var err error
				podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)

				err = clientset.CoreV1().Nodes().Delete(ctx, node1.Name, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() bool {
					resourcesRead := scaledMonitor.ReadResources(cluster1.ClusterID)
					for resourceName, quantity := range resourcesRead {
						toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
						toCheck.Sub(podReq[resourceName])
						ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
						if quantity.Cmp(toCheck) != 0 {
							return false
						}
					}
					return true
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					nodeList := []corev1.ResourceList{
						node2.Status.Allocatable,
					}
					podList := []corev1.ResourceList{
						podReq,
					}
					return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
				}, timeout, interval).Should(BeTrue())
			})
		})
		When("a pod changes status", func() {
			When("it becomes not ready", func() {
				It("updates the resources advertised", func() {
					var err error
					podWithoutLabel, err = setPodReadyStatus(ctx, podWithoutLabel, false, clientset)
					Expect(err).ToNot(HaveOccurred())
					Eventually(func() bool {
						nodeList := []corev1.ResourceList{
							node2.Status.Allocatable,
							node1.Status.Allocatable,
						}
						var podList []corev1.ResourceList
						return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
					}, timeout, interval).Should(BeTrue())
				})
			})
			When("it becomes ready", func() {
				It("updates the resources advertised", func() {
					var err error
					podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)

					podWithoutLabel, err = setPodReadyStatus(ctx, podWithoutLabel, true, clientset)
					Expect(err).ToNot(HaveOccurred())
					Eventually(func() bool {
						nodeList := []corev1.ResourceList{
							node2.Status.Allocatable,
							node1.Status.Allocatable,
						}
						podList := []corev1.ResourceList{
							podReq,
						}
						return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
					}, timeout, interval).Should(BeTrue())
				})
			})
		})

		When("a pod is offloaded", func() {
			When("by the same cluster", func() {
				It("does not change the resources", func() {
					var err error
					podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)

					_, err = createNewPod(ctx, "pod-offloaded-"+cluster1.ClusterID, cluster1.ClusterID, false, clientset)
					Expect(err).ToNot(HaveOccurred())
					Eventually(func() bool {
						nodeList := []corev1.ResourceList{
							node2.Status.Allocatable,
							node1.Status.Allocatable,
						}
						podList := []corev1.ResourceList{
							podReq,
						}
						return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
					}, timeout, interval).Should(BeTrue())
				})
			})
			When("by different clusters", func() {
				It("updates the resources advertised", func() {
					var err error
					podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)

					podOffloaded, err := createNewPod(ctx, "pod-offloaded-"+cluster2.ClusterID, cluster2.ClusterID, false, clientset)
					Expect(err).ToNot(HaveOccurred())
					podOffloaded2, err := createNewPod(ctx, "pod-offloaded-"+cluster3.ClusterID, cluster3.ClusterID, false, clientset)
					Expect(err).ToNot(HaveOccurred())
					podReq2, _ := resourcehelper.PodRequestsAndLimits(podOffloaded)
					podReq3, _ := resourcehelper.PodRequestsAndLimits(podOffloaded2)
					Eventually(func() bool {
						nodeList := []corev1.ResourceList{
							node2.Status.Allocatable,
							node1.Status.Allocatable,
						}
						podList := []corev1.ResourceList{
							podReq,
							podReq2,
							podReq3,
						}
						return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
					}, timeout, interval).Should(BeTrue())
					By("Checking change ready status for offloaded pod. Expected no change in offer.")
					_, err = setPodReadyStatus(ctx, podOffloaded, false, clientset)
					Expect(err).ToNot(HaveOccurred())
					Eventually(func() bool {
						nodeList := []corev1.ResourceList{
							node2.Status.Allocatable,
							node1.Status.Allocatable,
						}
						podList := []corev1.ResourceList{
							podReq,
							podReq2,
							podReq3,
						}
						return checkResourceOfferUpdate(ctx, BrokerNamespace, nodeList, podList, k8sClient)
					}, timeout, interval).Should(BeTrue())

					By("Update threshold with huge amount to test isAboveThreshold function")
					updater.SetThreshold(80)
					cpu := node2.Status.Allocatable[corev1.ResourceCPU]
					cpu.Add(*resource.NewQuantity(2, resource.DecimalSI))
					node2.Status.Allocatable[corev1.ResourceCPU] = cpu
					node2, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node2, metav1.UpdateOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(updater.isAboveThreshold(cluster1.ClusterID)).ShouldNot(BeTrue())
					updater.SetThreshold(4)
				})
			})
		})
	})
})
*/

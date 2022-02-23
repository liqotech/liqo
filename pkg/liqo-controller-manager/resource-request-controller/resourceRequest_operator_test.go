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

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	resourcemonitors "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller/resource-monitors"
	"github.com/liqotech/liqo/pkg/utils"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

const (
	ResourceRequestName = "test-resource"
	ResourcesNamespace  = "default"
	ResourcesNamespace2 = "new-namespace"
	timeout             = time.Second * 5
	interval            = time.Millisecond * 100
)

var (
	now      = metav1.Now()
	cluster1 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "test-cluster1-id",
		ClusterName: "test-cluster1-name",
	}
	cluster1Copy = discoveryv1alpha1.ClusterIdentity{ // A copy of cluster1 with different ID and same name
		ClusterID:   "test-cluster1-copy-id",
		ClusterName: "test-cluster1-name",
	}
	cluster2 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "test-cluster2-id",
		ClusterName: "test-cluster2-name",
	}
	cluster3 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "test-cluster3-id",
		ClusterName: "test-cluster3-name",
	}

	offerName types.NamespacedName
)

func CreateResourceRequest(ctx context.Context, resourceRequestName, resourcesNamespace string,
	cluster discoveryv1alpha1.ClusterIdentity, k8sClient client.Client) *discoveryv1alpha1.ResourceRequest {
	By("By creating a new ResourceRequest")
	resourceRequest := &discoveryv1alpha1.ResourceRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceRequestName + cluster.ClusterID,
			Namespace: resourcesNamespace,
			Labels: map[string]string{
				consts.ReplicationOriginLabel: cluster.ClusterID,
				consts.ReplicationStatusLabel: "true",
			},
		},
		Spec: discoveryv1alpha1.ResourceRequestSpec{
			AuthURL:         "https://127.0.0.1:39087",
			ClusterIdentity: cluster,
		},
	}
	Expect(k8sClient.Create(ctx, resourceRequest)).Should(Succeed())
	requestLookupKey := types.NamespacedName{
		Name:      resourceRequestName + cluster.ClusterID,
		Namespace: resourcesNamespace,
	}
	createdResourceRequest := &discoveryv1alpha1.ResourceRequest{}
	Eventually(func() bool {
		err := k8sClient.Get(ctx, requestLookupKey, createdResourceRequest)
		return err == nil
	}, timeout, interval).Should(BeTrue())
	return createdResourceRequest
}

func isShadowPod(podToCheck *corev1.Pod) bool {
	if shadowLabel, exists := podToCheck.Labels[consts.LocalPodLabelKey]; exists {
		if shadowLabel == consts.LocalPodLabelValue {
			return true
		}
	}
	return false
}

var _ = Describe("ResourceRequest Operator", func() {

	type storageClassTemplate struct {
		name        string
		provisioner string
		isDefault   bool
	}

	var (
		createdResourceRequest *discoveryv1alpha1.ResourceRequest
		podWithoutLabel        *corev1.Pod
		node1                  *corev1.Node
		node2                  *corev1.Node
		storageClasses         = []storageClassTemplate{
			{
				name:        "test-storage-class-1",
				provisioner: "prov-1",
				isDefault:   false,
			},
			{
				name:        "test-storage-class-2",
				provisioner: "prov-2",
				isDefault:   true,
			},
		}
	)

	BeforeEach(func() {
		createdResourceRequest = CreateResourceRequest(ctx, ResourceRequestName, ResourcesNamespace, cluster1, k8sClient)
		var err error
		node1, err = createNewNode(ctx, "test-node1", false, clientset)
		Expect(err).ToNot(HaveOccurred())
		node2, err = createNewNode(ctx, "test-node2", false, clientset)
		Expect(err).ToNot(HaveOccurred())
		podWithoutLabel, err = createNewPod(ctx, "test-pod-2", "", false, clientset)
		Expect(err).ToNot(HaveOccurred())

		for _, storageClass := range storageClasses {
			_, err = createNewStorageClass(ctx, clientset, storageClass.name, storageClass.provisioner, storageClass.isDefault)
			Expect(err).ToNot(HaveOccurred())
		}
	})
	AfterEach(func() {
		Expect(k8sClient.DeleteAllOf(ctx, &discoveryv1alpha1.ResourceRequest{}, client.InNamespace(ResourcesNamespace))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &discoveryv1alpha1.ResourceRequest{}, client.InNamespace(ResourcesNamespace2))).To(Succeed())
		Eventually(func() []discoveryv1alpha1.ResourceRequest {
			var rrl discoveryv1alpha1.ResourceRequestList
			Expect(k8sClient.List(ctx, &rrl)).To(Succeed())
			return rrl.Items
		}, timeout, interval).Should(HaveLen(0))

		Expect(k8sClient.DeleteAllOf(ctx, &sharingv1alpha1.ResourceOffer{}, client.InNamespace(ResourcesNamespace))).To(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &sharingv1alpha1.ResourceOffer{}, client.InNamespace(ResourcesNamespace2))).To(Succeed())
		Eventually(func() []sharingv1alpha1.ResourceOffer {
			var rro sharingv1alpha1.ResourceOfferList
			Expect(k8sClient.List(ctx, &rro)).To(Succeed())
			return rro.Items
		}, timeout, interval).Should(HaveLen(0))

		Expect(clientset.CoreV1().Pods("default").DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		Expect(clientset.CoreV1().Nodes().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		Expect(clientset.StorageV1().StorageClasses().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())

		Expect(k8sClient.DeleteAllOf(ctx, &discoveryv1alpha1.ForeignCluster{})).To(Succeed())
	})

	When("Creating a new ResourceRequest", func() {

		It("Should create a ForeignCluster", func() {
			Eventually(func() error {
				var fc discoveryv1alpha1.ForeignCluster
				return k8sClient.Get(ctx, types.NamespacedName{
					Name: cluster1.ClusterName,
				}, &fc)
			}, timeout, interval).Should(Succeed())
		})

		It("Should create a new ForeignCluster in presence of cluster name conflicts", func() {
			// Make sure the first ForeignCluster has already been created before creating the new resource request, to ensure ordering.
			Eventually(func() error {
				var fc discoveryv1alpha1.ForeignCluster
				return k8sClient.Get(ctx, types.NamespacedName{Name: cluster1.ClusterName}, &fc)
			}, timeout, interval).Should(Succeed())

			CreateResourceRequest(ctx, ResourceRequestName, ResourcesNamespace2, cluster1Copy, k8sClient)
			Eventually(func() error {
				var fc discoveryv1alpha1.ForeignCluster
				return k8sClient.Get(ctx, types.NamespacedName{Name: foreignclusterutils.UniqueName(&cluster1Copy)}, &fc)
			}, timeout, interval).Should(Succeed())
		})

		It("Should create ClusterRole and ClusterRoleBinding", func() {
			var clusterRole rbacv1.ClusterRole
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name: fmt.Sprintf("liqo-tenant-remote-%s", GetTenantName(cluster1)),
				}, &clusterRole)
			}, timeout, interval).ShouldNot(HaveOccurred())

			Expect(clusterRole.Rules).To(HaveLen(1))
			Expect(clusterRole.Rules[0]).To(Equal(rbacv1.PolicyRule{
				APIGroups:     []string{"metrics.liqo.io"},
				Resources:     []string{"scrape", "scrape/metrics"},
				Verbs:         []string{"get"},
				ResourceNames: []string{cluster1.ClusterID},
			}))

			var clusterRoleBinding rbacv1.ClusterRoleBinding
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name: fmt.Sprintf("liqo-tenant-remote-%s", GetTenantName(cluster1)),
				}, &clusterRoleBinding)
			}, timeout, interval).ShouldNot(HaveOccurred())

			Expect(clusterRoleBinding.Subjects).To(HaveLen(1))
			Expect(clusterRoleBinding.Subjects[0]).To(Equal(rbacv1.Subject{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     rbacv1.UserKind,
				Name:     cluster1.ClusterID,
			}))
			Expect(clusterRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     fmt.Sprintf("liqo-tenant-remote-%s", GetTenantName(cluster1)),
			}))
		})

		It("Should create a new ResourceOffer", func() {
			By("Checking Offer creation")
			var offers sharingv1alpha1.ResourceOfferList
			Eventually(func() []sharingv1alpha1.ResourceOffer {
				Expect(k8sClient.List(ctx, &offers, client.InNamespace(ResourcesNamespace))).To(Succeed())
				return offers.Items
			}, timeout, interval).Should(HaveLen(1))
			createdResourceOffer := &offers.Items[0]
			By("Checking all ResourceOffer parameters")

			offerName = types.NamespacedName{
				Name:      createdResourceOffer.Name,
				Namespace: ResourcesNamespace,
			}
			Expect(createdResourceOffer.Name).Should(ContainSubstring(homeCluster.ClusterName))
			Expect(createdResourceOffer.Labels[discovery.ClusterIDLabel]).Should(Equal(createdResourceRequest.Spec.ClusterIdentity.ClusterID))
			Expect(createdResourceOffer.Labels[consts.ReplicationRequestedLabel]).Should(Equal("true"))
			Expect(createdResourceOffer.Labels[consts.ReplicationDestinationLabel]).Should(Equal(createdResourceRequest.Spec.ClusterIdentity.ClusterID))
			By("Checking OwnerReference for Garbage Collector")
			Expect(createdResourceOffer.GetOwnerReferences()).ShouldNot(HaveLen(0))
			Expect(createdResourceOffer.GetOwnerReferences()).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name": Equal(createdResourceRequest.Name),
			})))

			Eventually(func() []sharingv1alpha1.StorageType {
				Expect(k8sClient.Get(ctx, offerName, createdResourceOffer)).To(Succeed())
				return createdResourceOffer.Spec.StorageClasses
			}, timeout, interval).ShouldNot(BeEmpty())
			for _, storageClass := range storageClasses {
				item := sharingv1alpha1.StorageType{
					StorageClassName: storageClass.name,
					Default:          storageClass.isDefault,
				}
				Expect(createdResourceOffer.Spec.StorageClasses).To(ContainElement(item))
			}

			By("Checking resources at offer creation")
			podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node1.Status.Allocatable,
					1: node2.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())

			By("Checking ResourceOffer invalidation on request set deleting phase")
			var resourceRequest discoveryv1alpha1.ResourceRequest
			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				// make sure to be working on the last ForeignCluster version
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      ResourceRequestName + cluster1.ClusterID,
					Namespace: ResourcesNamespace,
				}, &resourceRequest)
				if err != nil {
					return err
				}
				resourceRequest.Spec.WithdrawalTimestamp = &now

				return k8sClient.Update(ctx, &resourceRequest)
			})
			Expect(err).ToNot(HaveOccurred())

			// set the vk status in the ResourceOffer to created
			err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      createdResourceOffer.Name,
					Namespace: createdResourceOffer.Namespace,
				}, createdResourceOffer)
				if err != nil {
					return err
				}
				createdResourceOffer.Status.VirtualKubeletStatus = sharingv1alpha1.VirtualKubeletStatusCreated
				createdResourceOffer.Status.Phase = sharingv1alpha1.ResourceOfferAccepted
				return k8sClient.Status().Update(ctx, createdResourceOffer)
			})
			Expect(err).ToNot(HaveOccurred())
			var resourceOffer sharingv1alpha1.ResourceOffer
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, offerName, &resourceOffer); err != nil {
					return false
				}
				return !resourceOffer.Spec.WithdrawalTimestamp.IsZero()
			}, timeout, interval).Should(BeTrue())

			By("Checking ResourceOffer deletion")
			// the ResourceOffer should be deleted when the remote VirtualKubelet will be down
			Expect(k8sClient.Get(ctx, offerName, createdResourceOffer)).ToNot(HaveOccurred())
			createdResourceOffer.Status.VirtualKubeletStatus = sharingv1alpha1.VirtualKubeletStatusNone
			Expect(k8sClient.Status().Update(ctx, createdResourceOffer)).ToNot(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, offerName, &resourceOffer)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Checking ResourceRequest Deletion")
			err = k8sClient.Delete(ctx, &resourceRequest)
			Expect(err).ToNot(HaveOccurred())

			// check the resource request deletion
			Eventually(func() bool {
				var rr discoveryv1alpha1.ResourceRequest
				return apierrors.IsNotFound(k8sClient.Get(ctx,
					types.NamespacedName{Name: resourceRequest.Name, Namespace: resourceRequest.Namespace},
					&rr))
			}, timeout, interval).Should(BeTrue())
		})
	})

	When("Creating a new ResourceRequest", func() {
		It("LocalResourceMonitor should update resources in correct way", func() {
			podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)

			By("Checking update node ready condition")
			var err error
			node1, err = setNodeReadyStatus(ctx, node1, false, clientset)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				resourcesRead := scaledMonitor.ReadResources(cluster1.ClusterID)
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
					toCheck.Sub(podReq[resourceName])
					resourcemonitors.ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Checking if ResourceOffer has been update and has correct amount of resources")
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
			node1, err = setNodeReadyStatus(ctx, node1, true, clientset)
			Expect(err).ToNot(HaveOccurred())

			By("Checking inserting of node1 again in ResourceOffer")
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node1.Status.Allocatable,
					1: node2.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				resourcesRead := scaledMonitor.ReadResources(cluster1.ClusterID)
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
					toCheck.Add(node1.Status.Allocatable[resourceName])
					toCheck.Sub(podReq[resourceName])
					resourcemonitors.ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Decreasing node resources")
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
					resourcemonitors.ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Creating new resources on node1")
			toUpdate = node1.Status.Allocatable.DeepCopy()
			toUpdate["liqo.io/fake-resource"] = *resource.NewQuantity(10, resource.DecimalSI)
			node1.Status.Allocatable = toUpdate.DeepCopy()
			node1, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node1, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				resourcesRead := scaledMonitor.ReadResources(cluster1.ClusterID)
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
					toCheck.Add(node1.Status.Allocatable[resourceName])
					toCheck.Sub(podReq[resourceName])
					resourcemonitors.ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// Reproduces issue #1052.
			By("Creating new resources on node2")
			toUpdate = node2.Status.Allocatable.DeepCopy()
			toUpdate["liqo.io/fake-resource"] = *resource.NewQuantity(10, resource.DecimalSI)
			node2.Status.Allocatable = toUpdate.DeepCopy()
			node2, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node2, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				resourcesRead := scaledMonitor.ReadResources(cluster1.ClusterID)
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
					toCheck.Add(node1.Status.Allocatable[resourceName])
					toCheck.Sub(podReq[resourceName])
					resourcemonitors.ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Checking if ResourceOffer has been updated correctly")
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
					1: node1.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
			By("Checking Node Delete")
			err = clientset.CoreV1().Nodes().Delete(ctx, node1.Name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				resourcesRead := scaledMonitor.ReadResources(cluster1.ClusterID)
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
					toCheck.Sub(podReq[resourceName])
					resourcemonitors.ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
			By("Checking if ResourceOffer has been updated correctly")
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
			By("Checking correct update of resource after pod changing Status")
			podWithoutLabel, err = setPodPhase(ctx, podWithoutLabel, corev1.PodFailed, clientset)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
				}
				var podList []corev1.ResourceList
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
			// set the pod ready again
			podWithoutLabel, err = setPodPhase(ctx, podWithoutLabel, corev1.PodRunning, clientset)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
			By("Adding pod offloaded by cluster which refers the ResourceOffer. Expected no change in resources")
			_, err = createNewPod(ctx, "pod-offloaded-"+cluster1.ClusterID, cluster1.ClusterID, false, clientset)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
			By("Adding pods offloaded by a different clusters. Expected change in resources.")
			podOffloaded, err := createNewPod(ctx, "pod-offloaded-"+cluster2.ClusterID, cluster2.ClusterID, false, clientset)
			Expect(err).ToNot(HaveOccurred())
			podOffloaded2, err := createNewPod(ctx, "pod-offloaded-"+cluster3.ClusterID, cluster3.ClusterID, false, clientset)
			Expect(err).ToNot(HaveOccurred())
			podReq2, _ := resourcehelper.PodRequestsAndLimits(podOffloaded)
			podReq3, _ := resourcehelper.PodRequestsAndLimits(podOffloaded2)
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
					1: podReq2,
					2: podReq3,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
			By("Checking change ready status for offloaded pod. Expected no change in offer.")
			_, err = setPodPhase(ctx, podOffloaded, corev1.PodFailed, clientset)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
					1: podReq2,
					2: podReq3,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
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
	Context("Testing virtual nodes and shadow pods", func() {
		It("Test virtual node creation", func() {
			By("Testing check function returning false")
			Expect(utils.IsVirtualNode(node2)).ShouldNot(BeTrue())
			podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)
			virtualNode, err := createNewNode(ctx, "test-virtual-node", true, clientset)
			Expect(err).ToNot(HaveOccurred())
			Expect(utils.IsVirtualNode(virtualNode)).Should(BeTrue())
			By("Expected no change on resources")
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
					1: node1.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
		})
		It("Testing shadow pod creation", func() {
			pod, err := createNewPod(ctx, "shadow-test", "", true, clientset)
			Expect(err).ToNot(HaveOccurred())
			Expect(isShadowPod(pod)).Should(BeTrue())
			podReq1, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)
			By("Expected just normal pod resources")
			Eventually(func() bool {
				nodeList := []corev1.ResourceList{
					0: node2.Status.Allocatable,
					1: node1.Status.Allocatable,
				}
				podList := []corev1.ResourceList{
					0: podReq1,
				}
				return checkResourceOfferUpdate(ctx, homeCluster, nodeList, podList, k8sClient)
			}, timeout, interval).Should(BeTrue())
		})
	})
	Context("Setting zero node resources", func() {
		It("Testing negative resources", func() {
			cpu1 := node1.Status.Allocatable[corev1.ResourceCPU]
			cpu2 := node2.Status.Allocatable[corev1.ResourceCPU]
			cpu2.Set(0)
			cpu1.Set(0)
			node2.Status.Allocatable[corev1.ResourceCPU] = cpu2
			_, err := clientset.CoreV1().Nodes().UpdateStatus(ctx, node2, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			node1.Status.Allocatable[corev1.ResourceCPU] = cpu1
			_, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node1, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			By("Checking resources become all zero")
			Eventually(func() bool {
				offer := &sharingv1alpha1.ResourceOffer{}
				if err := k8sClient.Get(ctx, offerName, offer); err != nil {
					if !apierrors.IsNotFound(err) {
						Expect(err).ToNot(HaveOccurred())
					}
					return false
				}
				return isAllZero(&offer.Spec.ResourceQuota.Hard)
			}, timeout, interval).Should(BeTrue())
		})
	})
})

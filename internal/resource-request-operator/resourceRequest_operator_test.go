package resourcerequestoperator

import (
	"fmt"
	"time"

	capsulev1alpha1 "github.com/clastix/capsule/api/v1alpha1"
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
	"k8s.io/klog/v2"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

const (
	ResourceRequestName = "test-resource"
	ResourcesNamespace  = "default"
	timeout             = time.Second * 10
	interval            = time.Millisecond * 250
	homeClusterID       = "2468825c-0f62-44d7-bed1-9a7bc331c0b0"
)

var (
	now = metav1.Now()
)

func createTestNodes() (node1, node2 *corev1.Node) {
	resources := corev1.ResourceList{}
	resources[corev1.ResourceCPU] = *resource.NewQuantity(2, resource.DecimalSI)
	resources[corev1.ResourceMemory] = *resource.NewQuantity(1000000, resource.DecimalSI)
	first := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node1",
		},
	}
	second := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node2",
		},
	}
	first, err := clientset.CoreV1().Nodes().Create(ctx, first, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	first.Status = corev1.NodeStatus{
		Capacity:    resources,
		Allocatable: resources,
		Conditions: []corev1.NodeCondition{
			0: {
				Type:   corev1.NodeReady,
				Status: corev1.ConditionTrue,
			},
		},
	}
	first, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, first, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())
	second, err = clientset.CoreV1().Nodes().Create(ctx, second, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	second.Status = corev1.NodeStatus{
		Capacity:    resources,
		Allocatable: resources,
		Conditions: []corev1.NodeCondition{
			0: {
				Type:   corev1.NodeReady,
				Status: corev1.ConditionTrue,
			},
		},
	}
	second, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, second, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())
	return first, second
}

func createTestPods() (podWithLabels, podWithoutLabels *corev1.Pod) {
	resources := corev1.ResourceList{}
	resources[corev1.ResourceCPU] = *resource.NewQuantity(1, resource.DecimalSI)
	resources[corev1.ResourceMemory] = *resource.NewQuantity(50000, resource.DecimalSI)
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod1",
			Namespace: "default",
			Labels: map[string]string{
				forge.LiqoOutgoingKey:     "test",
				forge.LiqoOriginClusterID: homeClusterID,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				0: {
					Name: "test-container1",
					Resources: corev1.ResourceRequirements{
						Limits:   resources,
						Requests: resources,
					},
					Image: "nginx",
				},
			},
		},
	}

	wrongPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-wrong-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				0: {
					Name: "test-container1",
					Resources: corev1.ResourceRequirements{
						Limits:   resources,
						Requests: resources,
					},
					Image: "nginx",
				},
			},
		},
	}

	pod1, err := clientset.CoreV1().Pods("default").Create(ctx, pod1, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	wrongPod, err = clientset.CoreV1().Pods("default").Create(ctx, wrongPod, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	return pod1, wrongPod
}

func checkResourceOfferUpdate(nodeResources, podResources []corev1.ResourceList) bool {
	offer := &sharingv1alpha1.ResourceOffer{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      offerPrefix + clusterID,
		Namespace: ResourcesNamespace,
	}, offer)
	if err != nil {
		return false
	}
	offerResources := offer.Spec.ResourceQuota.Hard
	testList := corev1.ResourceList{}
	for _, nodeResource := range nodeResources {
		for resourceName, quantity := range nodeResource {
			toAdd := testList[resourceName].DeepCopy()
			toAdd.Add(quantity)
			testList[resourceName] = toAdd.DeepCopy()
		}
	}

	for _, podResource := range podResources {
		for resourceName, quantity := range podResource {
			toSub := testList[resourceName].DeepCopy()
			toSub.Sub(quantity)
			testList[resourceName] = toSub.DeepCopy()
		}
	}

	for resourceName, quantity := range offerResources {
		toCheck := testList[resourceName].DeepCopy()
		scale(resourceName, &toCheck)
		if quantity.Cmp(toCheck) != 0 {
			return false
		}
	}
	return true
}

func scale(resourceName corev1.ResourceName, quantity *resource.Quantity) {
	percentage := int64(50)
	switch resourceName {
	case corev1.ResourceCPU:
		// use millis
		quantity.SetScaled(quantity.MilliValue()*percentage/100, resource.Milli)
	case corev1.ResourceMemory:
		// use mega
		quantity.SetScaled(quantity.ScaledValue(resource.Mega)*percentage/100, resource.Mega)
	default:
		quantity.Set(quantity.Value() * percentage / 100)
	}
}

func createResourceRequest() *discoveryv1alpha1.ResourceRequest {
	By("By creating a new ResourceRequest")
	resourceRequest := &discoveryv1alpha1.ResourceRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ResourceRequestName,
			Namespace: ResourcesNamespace,
			Labels: map[string]string{
				crdreplicator.RemoteLabelSelector:    homeClusterID,
				crdreplicator.ReplicationStatuslabel: "true",
			},
		},
		Spec: discoveryv1alpha1.ResourceRequestSpec{
			AuthURL: "https://127.0.0.1:39087",
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID: homeClusterID,
			},
		},
	}
	Expect(k8sClient.Create(ctx, resourceRequest)).Should(Succeed())
	requestLookupKey := types.NamespacedName{Name: ResourceRequestName, Namespace: ResourcesNamespace}
	createdResourceRequest := &discoveryv1alpha1.ResourceRequest{}
	Eventually(func() bool {
		err := k8sClient.Get(ctx, requestLookupKey, createdResourceRequest)
		return err == nil
	}, timeout, interval).Should(BeTrue())
	return createdResourceRequest
}

var _ = Describe("ResourceRequest Operator", func() {
	var (
		createdResourceRequest *discoveryv1alpha1.ResourceRequest
		podWithoutLabel        *corev1.Pod
		node1                  *corev1.Node
		node2                  *corev1.Node
	)
	BeforeEach(func() {
		createdResourceRequest = createResourceRequest()
		node1, node2 = createTestNodes()
		_, podWithoutLabel = createTestPods()
	})
	AfterEach(func() {
		err := clientset.CoreV1().Nodes().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		err = clientset.CoreV1().Pods("default").DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &discoveryv1alpha1.ResourceRequest{}, client.InNamespace(ResourcesNamespace))
		Expect(err).ToNot(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &sharingv1alpha1.ResourceOffer{}, client.InNamespace(ResourcesNamespace))
		Expect(err).ToNot(HaveOccurred())
	})

	Context("Testing ResourceRequest Controller when creating a new ResourceRequest", func() {
		It("Should create new ResourceOffer ", func() {
			By("Checking Request status")
			var resourceRequest discoveryv1alpha1.ResourceRequest
			Eventually(func() []string {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      ResourceRequestName,
					Namespace: ResourcesNamespace,
				}, &resourceRequest)
				if err != nil {
					return []string{}
				}
				return resourceRequest.Finalizers
			}, timeout, interval).Should(ContainElement(tenantFinalizer))

			Expect(resourceRequest.Status.OfferWithdrawalTimestamp.IsZero()).To(BeTrue())

			By("Checking Tenant creation")
			var tenant capsulev1alpha1.Tenant
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name: fmt.Sprintf("tenant-%v", resourceRequest.Spec.ClusterIdentity.ClusterID),
				}, &tenant)
			}, timeout, interval).ShouldNot(HaveOccurred())

			Expect(string(tenant.Spec.Owner.Kind)).To(Equal(rbacv1.UserKind))
			Expect(tenant.Spec.Owner.Name).To(Equal(resourceRequest.Spec.ClusterIdentity.ClusterID))
			Expect(tenant.Spec.AdditionalRoleBindings).To(ContainElement(
				capsulev1alpha1.AdditionalRoleBindings{
					ClusterRoleName: "liqo-virtual-kubelet-remote",
					Subjects: []rbacv1.Subject{
						{
							Kind: rbacv1.UserKind,
							Name: resourceRequest.Spec.ClusterIdentity.ClusterID,
						},
					},
				},
			))

			By("Checking Offer creation")
			createdResourceOffer := &sharingv1alpha1.ResourceOffer{}
			offerName := types.NamespacedName{
				Name:      offerPrefix + clusterID,
				Namespace: ResourcesNamespace,
			}
			klog.Info(offerName)
			Eventually(func() error {
				return k8sClient.Get(ctx, offerName, createdResourceOffer)
			}, timeout, interval).ShouldNot(HaveOccurred())
			By("Checking all ResourceOffer parameters")

			Expect(createdResourceOffer.Name).Should(ContainSubstring(clusterID))
			Expect(createdResourceOffer.Labels[discovery.ClusterIDLabel]).Should(Equal(createdResourceRequest.Spec.ClusterIdentity.ClusterID))
			Expect(createdResourceOffer.Labels[crdreplicator.LocalLabelSelector]).Should(Equal("true"))
			Expect(createdResourceOffer.Labels[crdreplicator.DestinationLabel]).Should(Equal(createdResourceRequest.Spec.ClusterIdentity.ClusterID))
			By("Checking OwnerReference for Garbage Collector")
			Expect(createdResourceOffer.GetOwnerReferences()).ShouldNot(HaveLen(0))
			Expect(createdResourceOffer.GetOwnerReferences()).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name": Equal(createdResourceRequest.Name),
			})))

			By("Checking resources at offer creation")
			podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)
			Eventually(func() bool {
				offer := &sharingv1alpha1.ResourceOffer{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      offerPrefix + clusterID,
					Namespace: ResourcesNamespace,
				}, offer)
				if err != nil {
					return false
				}
				offerResources := offer.Spec.ResourceQuota.Hard
				for resourceName, quantity := range offerResources {
					testValue := node2.Status.Allocatable[resourceName].DeepCopy()
					testValue.Add(node1.Status.Allocatable[resourceName])
					testValue.Sub(podReq[resourceName])
					scale(resourceName, &testValue)
					if quantity.Cmp(testValue) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Checking ResourceOffer invalidation on request set deleting phase")
			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				// make sure to be working on the last ForeignCluster version
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      resourceRequest.Name,
					Namespace: resourceRequest.Namespace,
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

			By("Checking Tenant Deletion")
			err = k8sClient.Delete(ctx, &resourceRequest)
			Expect(err).ToNot(HaveOccurred())

			// check the tenant deletion
			Eventually(func() int {
				var tenantList capsulev1alpha1.TenantList
				err := k8sClient.List(ctx, &tenantList)
				if err != nil {
					return -1
				}
				return len(tenantList.Items)
			}, timeout, interval).Should(BeNumerically("==", 0))

			// check the resource request deletion and that the finalizer has been removed
			Eventually(func() int {
				var resourceRequestList discoveryv1alpha1.ResourceRequestList
				err := k8sClient.List(ctx, &resourceRequestList)
				if err != nil {
					return -1
				}
				return len(resourceRequestList.Items)
			}, timeout, interval).Should(BeNumerically("==", 0))
		})
	})

	Context("Testing broadcaster", func() {
		It("Broadcaster should update resources in correct way", func() {
			var resourceRequest discoveryv1alpha1.ResourceRequest
			Eventually(func() []string {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      ResourceRequestName,
					Namespace: ResourcesNamespace,
				}, &resourceRequest)
				if err != nil {
					return []string{}
				}
				return resourceRequest.Finalizers
			}, timeout, interval).Should(ContainElement(tenantFinalizer))
			podReq, _ := resourcehelper.PodRequestsAndLimits(podWithoutLabel)
			By("Checking update node ready condition")
			node1.Status.Conditions[0] = corev1.NodeCondition{
				Type:   corev1.NodeReady,
				Status: corev1.ConditionFalse,
			}
			var err error
			node1, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node1, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				resourcesRead := newBroadcaster.ReadResources(homeClusterID)
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
					toCheck.Sub(podReq[resourceName])
					scale(resourceName, &toCheck)
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
				return checkResourceOfferUpdate(nodeList, podList)
			}, timeout, interval).Should(BeTrue())
			node1.Status.Conditions[0] = corev1.NodeCondition{
				Type:   corev1.NodeReady,
				Status: corev1.ConditionTrue,
			}
			node1, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node1, metav1.UpdateOptions{})
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
				return checkResourceOfferUpdate(nodeList, podList)
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				resourcesRead := newBroadcaster.ReadResources(homeClusterID)
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
					toCheck.Add(node1.Status.Allocatable[resourceName])
					toCheck.Sub(podReq[resourceName])
					scale(resourceName, &toCheck)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Checking update node resources")
			toUpdate := node1.Status.Allocatable.DeepCopy()
			for _, quantity := range toUpdate {
				quantity.Sub(*resource.NewQuantity(1, quantity.Format))
			}
			node1.Status.Allocatable = toUpdate.DeepCopy()
			node1, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node1, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				resourcesRead := newBroadcaster.ReadResources(homeClusterID)
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
					toCheck.Add(node1.Status.Allocatable[resourceName])
					toCheck.Sub(podReq[resourceName])
					scale(resourceName, &toCheck)
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
				return checkResourceOfferUpdate(nodeList, podList)
			}, timeout, interval).Should(BeTrue())
			By("Checking Node Delete")
			err = clientset.CoreV1().Nodes().Delete(ctx, node1.Name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				resourcesRead := newBroadcaster.ReadResources(homeClusterID)
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName].DeepCopy()
					toCheck.Sub(podReq[resourceName])
					scale(resourceName, &toCheck)
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
				return checkResourceOfferUpdate(nodeList, podList)
			}, timeout, interval).Should(BeTrue())
		})
	})
})

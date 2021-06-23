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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

func createTestNodes() (*corev1.Node, *corev1.Node) {
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

func createTestPods() (*corev1.Pod, *corev1.Pod) {
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
				crdreplicator.RemoteLabelSelector:    clusterId,
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
		node1, node2 = createTestNodes()
		_, podWithoutLabel = createTestPods()
		createdResourceRequest = createResourceRequest()
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
				Name:      offerPrefix + clusterId,
				Namespace: ResourcesNamespace,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, offerName, createdResourceOffer)
			}, timeout, interval).ShouldNot(HaveOccurred())
			By("Checking all ResourceOffer parameters")

			Expect(createdResourceOffer.Name).Should(ContainSubstring(clusterId))
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
			offerResources := createdResourceOffer.Spec.ResourceQuota.Hard
			for resourceName, quantity := range offerResources {
				testValue := node1.Status.Allocatable[resourceName].DeepCopy()
				testValue.Add(node2.Status.Allocatable[resourceName])
				testValue.Sub(podReq[resourceName])
				scale(resourceName, &testValue)
				Expect(quantity.Cmp(testValue)).Should(BeZero())
			}

			By("Checking Tenant Deletion")
			err := k8sClient.Delete(ctx, &resourceRequest)
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
			node1.Status.Conditions[0] = corev1.NodeCondition{
				Type:   corev1.NodeReady,
				Status: corev1.ConditionTrue,
			}
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
			By("Checking update resources")
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
		})
	})
})

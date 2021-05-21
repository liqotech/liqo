package resourcerequestoperator

import (
	"context"
	"time"

	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/discovery"
)

var _ = Describe("ResourceRequest controller", func() {

	const (
		ResourceRequestName = "test-resource"
		OfferPrefix         = "resourceoffer-"
		ResourcesNamespace  = "default"
		timeout             = time.Second * 10
		interval            = time.Millisecond * 250
	)

	Context("Testing ResourceRequest Controller when creating a new ResourceRequest", func() {
		It("Should create new ResourceRequest and related ResourceOffer ", func() {
			ctx := context.Background()
			By("Creating mock nodes")
			resources := corev1.ResourceList{}
			resources[corev1.ResourceCPU] = *resource.NewQuantity(2, resource.DecimalSI)
			resources[corev1.ResourceMemory] = *resource.NewQuantity(1, resource.BinarySI)
			resources[corev1.ResourceLimitsCPU] = *resource.NewQuantity(3, resource.DecimalSI)
			node1 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node1",
				},
				Status: corev1.NodeStatus{
					Capacity:    resources,
					Allocatable: resources,
					Phase:       corev1.NodeRunning,
				},
			}
			_, err := clientset.CoreV1().Nodes().Create(ctx, node1, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			node2 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node2",
				},
				Status: corev1.NodeStatus{
					Capacity:    resources,
					Allocatable: resources,
					Phase:       corev1.NodeRunning,
				},
			}
			_, err = clientset.CoreV1().Nodes().Create(ctx, node2, metav1.CreateOptions{})
			Expect(err).To(BeNil())
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
						ClusterID: "2468825c-0f62-44d7-bed1-9a7bc331c0b0",
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

			By("Checking Offer creation")
			createdResourceOffer := &sharingv1alpha1.ResourceOffer{}
			offerName := types.NamespacedName{
				Name:      offerPrefix + clusterId,
				Namespace: ResourcesNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, offerName, createdResourceOffer)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdResourceOffer.Name).Should(ContainSubstring(clusterId))
			Expect(createdResourceOffer.Labels[discovery.ClusterIDLabel]).Should(Equal(createdResourceRequest.Spec.ClusterIdentity.ClusterID))
			Expect(createdResourceOffer.Labels[crdreplicator.LocalLabelSelector]).Should(Equal("true"))
			Expect(createdResourceOffer.Labels[crdreplicator.DestinationLabel]).Should(Equal(createdResourceRequest.Spec.ClusterIdentity.ClusterID))
			By("Checking OwnerReference for Garbage Collector")
			Expect(createdResourceOffer.GetOwnerReferences()).ShouldNot(HaveLen(0))
			Expect(createdResourceOffer.GetOwnerReferences()).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name": Equal(createdResourceRequest.Name),
			})))
			By("Checking resources at creation")
			offerResources := createdResourceOffer.Spec.ResourceQuota.Hard
			for resourceName, quantity := range offerResources {
				testValue := node1.Status.Allocatable[resourceName]
				testValue.Add(node2.Status.Allocatable[resourceName])
				testValue.Set(testValue.Value() * 50 / 100)
				Expect(quantity.Cmp(testValue)).Should(BeZero())
			}
			By("Checking update node phase")
			node1.Status.Phase = corev1.NodePending
			_, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node1, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			Eventually(func() bool {
				resourcesRead, err := newBroadcaster.ReadResources()
				if err != nil {
					return false
				}
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName]
					toCheck.Set(toCheck.Value() * 50 / 100)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
			node1.Status.Phase = corev1.NodeRunning
			_, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node1, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			Eventually(func() bool {
				resourcesRead, err := newBroadcaster.ReadResources()
				if err != nil {
					return false
				}
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName]
					toCheck.Add(node1.Status.Allocatable[resourceName])
					toCheck.Set(toCheck.Value() * 50 / 100)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
			By("Checking update resources")
			toUpdate := node1.Status.Allocatable
			for _, quantity := range toUpdate {
				quantity.Sub(*resource.NewQuantity(1, quantity.Format))
			}
			node1.Status.Allocatable = toUpdate
			_, err = clientset.CoreV1().Nodes().UpdateStatus(ctx, node1, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			Eventually(func() bool {
				resourcesRead, err := newBroadcaster.ReadResources()
				if err != nil {
					return false
				}
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName]
					toCheck.Add(node1.Status.Allocatable[resourceName])
					toCheck.Set(toCheck.Value() * 50 / 100)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
			By("Checking Node Delete")
			err = clientset.CoreV1().Nodes().Delete(ctx, node1.Name, metav1.DeleteOptions{})
			Expect(err).To(BeNil())
			Eventually(func() bool {
				resourcesRead, err := newBroadcaster.ReadResources()
				if err != nil {
					return false
				}
				for resourceName, quantity := range resourcesRead {
					toCheck := node2.Status.Allocatable[resourceName]
					toCheck.Set(toCheck.Value() * 50 / 100)
					if quantity.Cmp(toCheck) != 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

		})

	})
})

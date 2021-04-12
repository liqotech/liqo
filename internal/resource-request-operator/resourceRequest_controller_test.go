package resourceRequestOperator

import (
	"context"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
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
			By("By creating a new ResourceRequest")
			ctx := context.Background()
			resourceRequest := &discoveryv1alpha1.ResourceRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ResourceRequestName,
					Namespace: ResourcesNamespace,
				},
				Spec: discoveryv1alpha1.ResourceRequestSpec{
					AuthUrl: "https://127.0.0.1:39087",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
						ClusterID: "2468825c-0f62-44d7-bed1-9a7bc331c0b0",
					},
					Namespace: ResourcesNamespace,
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
			Expect(createdResourceOffer.Labels[discovery.ClusterIdLabel]).Should(Equal(createdResourceRequest.Spec.ClusterIdentity.ClusterID))

			By("Checking OwnerReference for Garbage Collector")
			Expect(createdResourceOffer.GetOwnerReferences()).ShouldNot(HaveLen(0))
			Expect(createdResourceOffer.GetOwnerReferences()).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name": Equal(createdResourceRequest.Name),
			})))
		})

	})
})

package authenticationtoken

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

var _ = Context("eventHandler test", func() {

	Context("getReconcileRequestFromSecret", func() {

		const (
			foreignClusterName = "fc"
			foreignClusterID   = "fc-id-1"

			secretName = "token-secret"
			token      = "token"
		)

		var (
			foreignCluster *discoveryv1alpha1.ForeignCluster
		)

		BeforeEach(func() {
			foreignCluster = &discoveryv1alpha1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: foreignClusterName,
					Labels: map[string]string{
						discovery.ClusterIDLabel: foreignClusterID,
					},
				},
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
						ClusterID: foreignClusterID,
					},
					OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					ForeignAuthURL:         "https://example.com",
					InsecureSkipTLSVerify:  pointer.BoolPtr(true),
				},
			}

			Expect(k8sClient.Create(ctx, foreignCluster)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, foreignCluster))
		})

		It("getReconcileRequestFromSecret", func() {

			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: v1.NamespaceDefault,
					Labels: map[string]string{
						discovery.ClusterIDLabel: foreignClusterID,
					},
				},
			}

			reconcileRequest := getReconcileRequestFromSecret(ctx, k8sClient, secret)
			Expect(reconcileRequest).ToNot(BeNil())
			Expect(reconcileRequest.Namespace).To(BeEmpty())
			Expect(reconcileRequest.Name).To(Equal(foreignClusterName))

		})

	})

})

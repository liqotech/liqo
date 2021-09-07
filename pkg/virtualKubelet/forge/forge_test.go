package forge

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping/test"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
)

var _ = Describe("Virtual Kubelet labels test", func() {
	var (
		namespaceNattingTable *test.MockNamespaceMapper
		foreignClusterID      options.Option
	)

	Context("Testing Labels attached to offloaded pods", func() {
		BeforeEach(
			func() {
				namespaceNattingTable = &test.MockNamespaceMapper{Cache: map[string]string{}}
				namespaceNattingTable.Cache["homeNamespace"] = "homeNamespace-natted"
				foreignClusterID = types.NewNetworkingOption(types.RemoteClusterID, "foreign-id")
				InitForger(namespaceNattingTable, foreignClusterID)
			},
		)

		It("Creating new pod to offload", func() {
			foreignObj, err := HomeToForeign(nil, nil, LiqoOutgoingKey)
			Expect(err).NotTo(HaveOccurred())
			foreignPod := foreignObj.(*corev1.Pod)
			Expect(foreignPod.Labels[LiqoOutgoingKey]).ShouldNot(BeNil())
			Expect(foreignPod.Labels[LiqoOriginClusterID]).ShouldNot(BeNil())
			Expect(foreignPod.Labels[LiqoOriginClusterID]).Should(Equal("foreign-id"))

		})

	})

})

var _ = Describe("Forge toleration test", func() {
	var (
		tol1 corev1.Toleration
		tol2 corev1.Toleration
	)

	Context("Tolerations", func() {
		BeforeEach(func() {
			tol1 = corev1.Toleration{
				Key:      "virtual-node.liqo.io/not-allowed",
				Operator: "Exist",
				Effect:   "NoExecute",
			}
			tol2 = corev1.Toleration{
				Key:      "node.kubernetes.io/not-ready",
				Operator: "Exist",
				Effect:   "NoExecute",
			}

		})

		It("Filtering tolerations", func() {
			input := []corev1.Toleration{tol1, tol2}
			expected := []corev1.Toleration{tol2}
			output := forgeTolerations(input)
			Expect(output).To(Equal(expected))
		})

	})
})

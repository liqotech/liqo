package utils

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestAddress(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Address Suite")
}

var _ = Describe("Address", func() {

	var (
		cluster testutil.Cluster
		ctx     context.Context
		cancel  context.CancelFunc
	)

	BeforeSuite(func() {
		ctx, cancel = context.WithCancel(context.Background())

		var err error
		cluster, _, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	AfterSuite(func() {
		cancel()

		err := cluster.GetEnv().Stop()
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	type addressTestcase struct {
		node            *v1.Node
		expectedAddress string
	}

	DescribeTable("Address table",

		func(c addressTestcase) {
			client := kubernetes.NewForConfigOrDie(cluster.GetCfg())

			node, err := client.CoreV1().Nodes().Create(ctx, c.node, metav1.CreateOptions{})
			Expect(err).To(Succeed())
			node.Status = *c.node.Status.DeepCopy()
			node, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			Expect(err).To(Succeed())

			address, err := GetAPIServerAddressFromMasterNode(ctx, client)
			Expect(err).To(Succeed())

			Expect(address).To(Equal(c.expectedAddress))

			Expect(client.CoreV1().Nodes().Delete(ctx, node.Name, metav1.DeleteOptions{})).To(Succeed())
		},

		Entry("master node", addressTestcase{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/master": "",
					},
				},
				Spec: v1.NodeSpec{},
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{
							Type:    v1.NodeExternalIP,
							Address: "1.2.3.4",
						},
					},
				},
			},
			expectedAddress: "https://1.2.3.4:6443",
		}),

		Entry("control plane node", addressTestcase{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
				Spec: v1.NodeSpec{},
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{
							Type:    v1.NodeExternalIP,
							Address: "1.2.3.4",
						},
					},
				},
			},
			expectedAddress: "https://1.2.3.4:6443",
		}),
	)

})

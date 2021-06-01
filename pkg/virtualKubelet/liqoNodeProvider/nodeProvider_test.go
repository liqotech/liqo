package liqonodeprovider

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	testUtils "github.com/liqotech/liqo/pkg/utils/testUtils"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250

	nodeName         = "node-name"
	advName          = "adv-name"
	foreignClusterID = "foreign-id"
	kubeletNamespace = "default"
)

func TestNodeProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeProvider Suite")
}

var _ = Describe("NodeProvider", func() {

	var (
		cluster        testUtils.Cluster
		nodeProvider   *LiqoNodeProvider
		podStopper     chan struct{}
		networkStopper chan struct{}
		err            error
		nodeChan       chan *v1.Node
		ctx            context.Context
		cancel         context.CancelFunc

		stop chan struct{}
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		cluster, _, err = testUtils.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
		Expect(err).To(BeNil())

		client := kubernetes.NewForConfigOrDie(cluster.GetCfg())
		node := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
		}
		_, err = client.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
		Expect(err).To(BeNil())

		podStopper = make(chan struct{}, 1)
		networkStopper = make(chan struct{}, 1)
		nodeChan = make(chan *v1.Node, 10)

		nodeProvider, err = NewLiqoNodeProvider(nodeName, advName, foreignClusterID, kubeletNamespace, podStopper, networkStopper, cluster.GetCfg(), 0, false)
		Expect(err).To(BeNil())

		nodeProvider.NotifyNodeStatus(ctx, func(node *v1.Node) {
			nodeChan <- node
			client.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{})
		})

		var ready chan struct{}
		ready, stop = nodeProvider.StartProvider()
		close(ready)
	})

	AfterEach(func() {
		cancel()

		err := cluster.GetEnv().Stop()
		Expect(err).To(BeNil())

		close(stop)
		close(podStopper)
		close(nodeChan)
	})

	type nodeProviderTestcase struct {
		advertisement      *sharingv1alpha1.Advertisement
		tunnelEndpoint     *netv1alpha1.TunnelEndpoint
		expectedConditions []v1.NodeCondition
	}

	DescribeTable("NodeProvider table",

		func(c nodeProviderTestcase) {
			dynClient := dynamic.NewForConfigOrDie(cluster.GetCfg())

			if c.advertisement != nil {
				unstructAdv, err := runtime.DefaultUnstructuredConverter.ToUnstructured(c.advertisement)
				Expect(err).To(BeNil())
				_, err = dynClient.Resource(sharingv1alpha1.GroupVersion.WithResource("advertisements")).Create(ctx, &unstructured.Unstructured{
					Object: unstructAdv,
				}, metav1.CreateOptions{})
				Expect(err).To(BeNil())
			}

			if c.tunnelEndpoint != nil {
				Expect(isChanOpen(networkStopper)).To(BeTrue())

				c.tunnelEndpoint.Labels = map[string]string{
					consts.ClusterIDLabelName: nodeProvider.foreignClusterID,
				}
				unstructTep, err := runtime.DefaultUnstructuredConverter.ToUnstructured(c.tunnelEndpoint)
				Expect(err).To(BeNil())
				unstruct, err := dynClient.Resource(netv1alpha1.TunnelEndpointGroupVersionResource).Create(ctx, &unstructured.Unstructured{
					Object: unstructTep,
				}, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				unstruct.Object["status"] = unstructTep["status"]

				_, err = dynClient.Resource(netv1alpha1.TunnelEndpointGroupVersionResource).UpdateStatus(ctx, unstruct, metav1.UpdateOptions{})
				Expect(err).To(BeNil())

				Eventually(func() bool {
					return isChanOpen(networkStopper)
				}, timeout, interval).Should(BeFalse())
			}

			Eventually(func() []v1.NodeCondition {
				select {
				case node := <-nodeChan:
					return node.Status.Conditions
				default:
					return []v1.NodeCondition{}
				}
			}, timeout, interval).Should(ContainElements(c.expectedConditions))
		},

		Entry("update from Advertisement", nodeProviderTestcase{
			advertisement: &sharingv1alpha1.Advertisement{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Advertisement",
					APIVersion: sharingv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: advName,
				},
				Spec: sharingv1alpha1.AdvertisementSpec{
					ClusterId:     "remote-id",
					KubeConfigRef: v1.SecretReference{},
					LimitRange: v1.LimitRangeSpec{
						Limits: []v1.LimitRangeItem{},
					},
					Timestamp:  metav1.NewTime(time.Now()),
					TimeToLive: metav1.NewTime(time.Now().Add(1 * time.Hour)),
					ResourceQuota: v1.ResourceQuotaSpec{
						Hard: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(3, resource.DecimalSI),
						},
					},
				},
			},
			tunnelEndpoint: nil,
			expectedConditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodeMemoryPressure,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodeDiskPressure,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodePIDPressure,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodeNetworkUnavailable,
					Status: v1.ConditionTrue,
				},
			},
		}),

		Entry("update from TunnelEndpoint", nodeProviderTestcase{
			advertisement: nil,
			tunnelEndpoint: &netv1alpha1.TunnelEndpoint{
				TypeMeta: metav1.TypeMeta{
					Kind:       "TunnelEndpoint",
					APIVersion: netv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "tep",
				},
				Spec: netv1alpha1.TunnelEndpointSpec{
					BackendConfig: map[string]string{},
				},
				Status: netv1alpha1.TunnelEndpointStatus{
					Connection: netv1alpha1.Connection{
						Status: netv1alpha1.Connected,
					},
				},
			},
			expectedConditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodeMemoryPressure,
					Status: v1.ConditionTrue,
				},
				{
					Type:   v1.NodeDiskPressure,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodePIDPressure,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodeNetworkUnavailable,
					Status: v1.ConditionFalse,
				},
			},
		}),

		Entry("update from both", nodeProviderTestcase{
			advertisement: &sharingv1alpha1.Advertisement{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Advertisement",
					APIVersion: sharingv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: advName,
				},
				Spec: sharingv1alpha1.AdvertisementSpec{
					ClusterId:     "remote-id",
					KubeConfigRef: v1.SecretReference{},
					LimitRange: v1.LimitRangeSpec{
						Limits: []v1.LimitRangeItem{},
					},
					Timestamp:  metav1.NewTime(time.Now()),
					TimeToLive: metav1.NewTime(time.Now().Add(1 * time.Hour)),
					ResourceQuota: v1.ResourceQuotaSpec{
						Hard: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(3, resource.DecimalSI),
						},
					},
				},
			},
			tunnelEndpoint: &netv1alpha1.TunnelEndpoint{
				TypeMeta: metav1.TypeMeta{
					Kind:       "TunnelEndpoint",
					APIVersion: netv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "tep",
				},
				Spec: netv1alpha1.TunnelEndpointSpec{
					BackendConfig: map[string]string{},
				},
				Status: netv1alpha1.TunnelEndpointStatus{
					Connection: netv1alpha1.Connection{
						Status: netv1alpha1.Connected,
					},
				},
			},
			expectedConditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
				{
					Type:   v1.NodeMemoryPressure,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodeDiskPressure,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodePIDPressure,
					Status: v1.ConditionFalse,
				},
				{
					Type:   v1.NodeNetworkUnavailable,
					Status: v1.ConditionFalse,
				},
			},
		}),
	)

})

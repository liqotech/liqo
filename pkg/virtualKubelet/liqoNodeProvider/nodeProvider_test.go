// Copyright 2019-2024 The Liqo Authors
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

package liqonodeprovider

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250

	nodeName         = "node-name"
	foreignClusterID = "foreign-id"
	kubeletNamespace = "default"
)

func TestNodeProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeProvider Suite")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
})

var _ = Describe("NodeProvider", func() {

	var (
		cluster      testutil.Cluster
		nodeProvider *LiqoNodeProvider
		podStopper   chan struct{}
		err          error
		nodeChan     chan *v1.Node
		ctx          context.Context
		cancel       context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		cluster, _, err = testutil.NewTestCluster([]string{
			filepath.Join("..", "..", "..", "deployments", "liqo", "charts", "liqo-crds", "crds"),
		})
		Expect(err).To(BeNil())

		client := kubernetes.NewForConfigOrDie(cluster.GetCfg())

		podStopper = make(chan struct{}, 1)
		nodeChan = make(chan *v1.Node, 10)

		nodeProvider = NewLiqoNodeProvider(&InitConfig{
			NodeName:  nodeName,
			Namespace: kubeletNamespace,

			HomeConfig:      cluster.GetCfg(),
			RemoteConfig:    cluster.GetCfg(), /* not actually used in tests */
			RemoteClusterID: foreignClusterID,

			PodProviderStopper: podStopper,

			CheckNetworkStatus: true,
		})

		_, err = client.CoreV1().Nodes().Create(ctx, nodeProvider.GetNode(), metav1.CreateOptions{})
		Expect(err).To(BeNil())

		nodeProvider.NotifyNodeStatus(ctx, func(node *v1.Node) {
			nodeChan <- node
			_, _ = client.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{})
		})

		ready := nodeProvider.StartProvider(ctx)
		close(ready)
	})

	AfterEach(func() {
		cancel()

		err := cluster.GetEnv().Stop()
		Expect(err).To(BeNil())

		close(podStopper)
		close(nodeChan)
	})

	type nodeProviderTestcase struct {
		virtualNode        *offloadingv1alpha1.VirtualNode
		connection         *networkingv1alpha1.Connection
		expectedConditions []types.GomegaMatcher
	}

	ConditionMatcher := func(key v1.NodeConditionType, status v1.ConditionStatus) types.GomegaMatcher {
		return MatchFields(IgnoreExtras, Fields{
			"Type":   BeIdenticalTo(key),
			"Status": BeIdenticalTo(status),
		})
	}

	DescribeTable("NodeProvider table",
		func(c nodeProviderTestcase) {
			dynClient := dynamic.NewForConfigOrDie(cluster.GetCfg())

			if c.virtualNode != nil {
				unstructVirtualNode, err := runtime.DefaultUnstructuredConverter.ToUnstructured(c.virtualNode)
				Expect(err).To(BeNil())
				_, err = dynClient.Resource(offloadingv1alpha1.VirtualNodeGroupVersionResource).
					Namespace(kubeletNamespace).Create(ctx, &unstructured.Unstructured{
					Object: unstructVirtualNode,
				}, metav1.CreateOptions{})
				Expect(err).To(BeNil())
			}

			if c.connection != nil {
				c.connection.Labels = map[string]string{
					consts.RemoteClusterID: string(nodeProvider.foreignClusterID),
				}
				unstructConn, err := runtime.DefaultUnstructuredConverter.ToUnstructured(c.connection)
				Expect(err).To(BeNil())
				unstruct, err := dynClient.Resource(networkingv1alpha1.ConnectionGroupVersionResource).
					Namespace(kubeletNamespace).Create(ctx, &unstructured.Unstructured{
					Object: unstructConn,
				}, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				unstruct.Object["status"] = unstructConn["status"]

				_, err = dynClient.Resource(networkingv1alpha1.ConnectionGroupVersionResource).
					Namespace(kubeletNamespace).UpdateStatus(ctx, unstruct, metav1.UpdateOptions{})
				Expect(err).To(BeNil())
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

		Entry("update from VirtualNode", nodeProviderTestcase{
			virtualNode: &offloadingv1alpha1.VirtualNode{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VirtualNode",
					APIVersion: offloadingv1alpha1.VirtualNodeGroupVersionResource.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: kubeletNamespace,
				},
				Spec: offloadingv1alpha1.VirtualNodeSpec{
					ClusterID: "remote-id",
					ResourceQuota: v1.ResourceQuotaSpec{
						Hard: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(3, resource.DecimalSI),
						},
					},
				},
			},
			connection: nil,
			expectedConditions: []types.GomegaMatcher{
				ConditionMatcher(v1.NodeReady, v1.ConditionFalse),
				ConditionMatcher(v1.NodeMemoryPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodeDiskPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodePIDPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodeNetworkUnavailable, v1.ConditionTrue),
			},
		}),

		Entry("update from Connection", nodeProviderTestcase{
			virtualNode: nil,
			connection: &networkingv1alpha1.Connection{
				TypeMeta: metav1.TypeMeta{
					Kind:       networkingv1alpha1.ConnectionKind,
					APIVersion: networkingv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conn",
					Namespace: kubeletNamespace,
				},
				Spec: networkingv1alpha1.ConnectionSpec{
					Type: networkingv1alpha1.ConnectionTypeServer,
				},
				Status: networkingv1alpha1.ConnectionStatus{
					Value: networkingv1alpha1.Connected,
				},
			},
			expectedConditions: []types.GomegaMatcher{
				ConditionMatcher(v1.NodeReady, v1.ConditionFalse),
				ConditionMatcher(v1.NodeMemoryPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodeDiskPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodePIDPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodeNetworkUnavailable, v1.ConditionFalse),
			},
		}),

		Entry("update from both", nodeProviderTestcase{
			virtualNode: &offloadingv1alpha1.VirtualNode{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VirtualNode",
					APIVersion: offloadingv1alpha1.VirtualNodeGroupVersionResource.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: kubeletNamespace,
				},
				Spec: offloadingv1alpha1.VirtualNodeSpec{
					ClusterID: "remote-id",
					ResourceQuota: v1.ResourceQuotaSpec{
						Hard: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(3, resource.DecimalSI),
						},
					},
				},
			},
			connection: &networkingv1alpha1.Connection{
				TypeMeta: metav1.TypeMeta{
					Kind:       networkingv1alpha1.ConnectionKind,
					APIVersion: networkingv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conn",
					Namespace: kubeletNamespace,
				},
				Spec: networkingv1alpha1.ConnectionSpec{
					Type: networkingv1alpha1.ConnectionTypeServer,
				},
				Status: networkingv1alpha1.ConnectionStatus{
					Value: networkingv1alpha1.Connected,
				},
			},
			expectedConditions: []types.GomegaMatcher{
				ConditionMatcher(v1.NodeReady, v1.ConditionTrue),
				ConditionMatcher(v1.NodeMemoryPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodeDiskPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodePIDPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodeNetworkUnavailable, v1.ConditionFalse),
			},
		}),
	)

	It("Labels patch", func() {

		By("Add labels")

		labels := map[string]string{
			"test1": "value1",
			"test2": "value2",
		}

		err := nodeProvider.patchLabels(ctx, labels)
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeProvider.lastAppliedLabels).To(Equal(labels))

		client := kubernetes.NewForConfigOrDie(cluster.GetCfg())
		node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		nodeLabels := node.GetLabels()
		v, ok := nodeLabels["test1"]
		Expect(ok).To(BeTrue())
		Expect(v).To(Equal("value1"))
		v, ok = nodeLabels["test2"]
		Expect(ok).To(BeTrue())
		Expect(v).To(Equal("value2"))

		By("Update labels")

		labels = map[string]string{
			"test1": "value3",
			"test2": "value4",
		}

		err = nodeProvider.patchLabels(ctx, labels)
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeProvider.lastAppliedLabels).To(Equal(labels))

		node, err = client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		nodeLabels = node.GetLabels()
		v, ok = nodeLabels["test1"]
		Expect(ok).To(BeTrue())
		Expect(v).To(Equal("value3"))
		v, ok = nodeLabels["test2"]
		Expect(ok).To(BeTrue())
		Expect(v).To(Equal("value4"))

		By("Delete labels")

		labels = map[string]string{
			"test1": "value3",
		}

		err = nodeProvider.patchLabels(ctx, labels)
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeProvider.lastAppliedLabels).To(Equal(labels))

		node, err = client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		nodeLabels = node.GetLabels()
		v, ok = nodeLabels["test1"]
		Expect(ok).To(BeTrue())
		Expect(v).To(Equal("value3"))
		_, ok = nodeLabels["test2"]
		Expect(ok).To(BeFalse())
	})

})

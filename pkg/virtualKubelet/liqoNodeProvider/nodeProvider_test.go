// Copyright 2019-2026 The Liqo Authors
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

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250

	nodeName         = "node-name"
	foreignClusterID = "foreign-id"
	kubeletNamespace = "default"
	taintKey         = "virtual-taint"
	taintValue       = "taint-value"
	trueValue        = "true"
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
		foreignCluster     *liqov1beta1.ForeignCluster
		virtualNode        *offloadingv1beta1.VirtualNode
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
				_, err = dynClient.Resource(offloadingv1beta1.VirtualNodeGroupVersionResource).
					Namespace(kubeletNamespace).Create(ctx, &unstructured.Unstructured{
					Object: unstructVirtualNode,
				}, metav1.CreateOptions{})
				Expect(err).To(BeNil())
			}

			if c.foreignCluster != nil {
				unstructConn, err := runtime.DefaultUnstructuredConverter.ToUnstructured(c.foreignCluster)
				Expect(err).To(BeNil())
				unstruct, err := dynClient.Resource(liqov1beta1.ForeignClusterGroupVersionResource).
					Create(ctx, &unstructured.Unstructured{
						Object: unstructConn,
					}, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				unstruct.Object["status"] = unstructConn["status"]

				_, err = dynClient.Resource(liqov1beta1.ForeignClusterGroupVersionResource).
					UpdateStatus(ctx, unstruct, metav1.UpdateOptions{})
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

		Entry("Networking disabled, Offloading Enabled", nodeProviderTestcase{
			foreignCluster: testutil.FakeForeignCluster(foreignClusterID, &liqov1beta1.Modules{
				Offloading: liqov1beta1.Module{
					Enabled: true,
				},
				Networking: liqov1beta1.Module{
					Enabled: false,
				},
				Authentication: liqov1beta1.Module{
					Enabled: true,
				},
			}),
			virtualNode: &offloadingv1beta1.VirtualNode{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VirtualNode",
					APIVersion: offloadingv1beta1.VirtualNodeGroupVersionResource.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: kubeletNamespace,
				},
				Spec: offloadingv1beta1.VirtualNodeSpec{
					ClusterID: "remote-id",
					ResourceQuota: v1.ResourceQuotaSpec{
						Hard: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(3, resource.DecimalSI),
						},
					},
				},
			},
			expectedConditions: []types.GomegaMatcher{
				ConditionMatcher(v1.NodeReady, v1.ConditionTrue),
				ConditionMatcher(v1.NodeMemoryPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodeDiskPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodePIDPressure, v1.ConditionFalse),
			},
		}),

		Entry("Networking enabled (disconnected), Offloading disabled ", nodeProviderTestcase{
			foreignCluster: testutil.FakeForeignCluster(foreignClusterID, &liqov1beta1.Modules{
				Offloading: liqov1beta1.Module{
					Enabled: false,
				},
				Networking: liqov1beta1.Module{
					Enabled: true,
				},
				Authentication: liqov1beta1.Module{
					Enabled: false,
				},
			}),
			virtualNode: nil,
			expectedConditions: []types.GomegaMatcher{
				ConditionMatcher(v1.NodeReady, v1.ConditionFalse),
				ConditionMatcher(v1.NodeMemoryPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodeDiskPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodePIDPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodeNetworkUnavailable, v1.ConditionTrue),
			},
		}),

		Entry("Networking enabled (connected), Offloading disabled ", nodeProviderTestcase{
			foreignCluster: testutil.FakeForeignCluster(foreignClusterID, &liqov1beta1.Modules{
				Offloading: liqov1beta1.Module{
					Enabled: false,
				},
				Networking: liqov1beta1.Module{
					Enabled: true,
					Conditions: []liqov1beta1.Condition{{
						Type:   liqov1beta1.NetworkConnectionStatusCondition,
						Status: liqov1beta1.ConditionStatusEstablished,
					}},
				},
				Authentication: liqov1beta1.Module{
					Enabled: false,
				},
			}),
			virtualNode: nil,
			expectedConditions: []types.GomegaMatcher{
				ConditionMatcher(v1.NodeReady, v1.ConditionFalse),
				ConditionMatcher(v1.NodeMemoryPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodeDiskPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodePIDPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodeNetworkUnavailable, v1.ConditionFalse),
			},
		}),

		Entry("Networking enabled (connected), Offloading enabled", nodeProviderTestcase{
			foreignCluster: testutil.FakeForeignCluster(foreignClusterID, &liqov1beta1.Modules{
				Offloading: liqov1beta1.Module{
					Enabled: true,
				},
				Networking: liqov1beta1.Module{
					Enabled: true,
					Conditions: []liqov1beta1.Condition{{
						Type:   liqov1beta1.NetworkConnectionStatusCondition,
						Status: liqov1beta1.ConditionStatusEstablished,
					}},
				},
				Authentication: liqov1beta1.Module{
					Enabled: true,
				},
			}),
			virtualNode: &offloadingv1beta1.VirtualNode{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VirtualNode",
					APIVersion: offloadingv1beta1.VirtualNodeGroupVersionResource.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: kubeletNamespace,
				},
				Spec: offloadingv1beta1.VirtualNodeSpec{
					ClusterID: "remote-id",
					ResourceQuota: v1.ResourceQuotaSpec{
						Hard: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(3, resource.DecimalSI),
						},
					},
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

		Entry("ForeignCluster not yet reconciled (ObservedGeneration=0), Offloading enabled", nodeProviderTestcase{
			foreignCluster: func() *liqov1beta1.ForeignCluster {
				fc := testutil.FakeForeignCluster(foreignClusterID, &liqov1beta1.Modules{
					Offloading: liqov1beta1.Module{
						Enabled: true,
					},
					Networking: liqov1beta1.Module{
						Enabled: false,
					},
					Authentication: liqov1beta1.Module{
						Enabled: true,
					},
				})
				fc.Status.ObservedGeneration = 0
				return fc
			}(),
			virtualNode: &offloadingv1beta1.VirtualNode{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VirtualNode",
					APIVersion: offloadingv1beta1.VirtualNodeGroupVersionResource.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: kubeletNamespace,
				},
				Spec: offloadingv1beta1.VirtualNodeSpec{
					ClusterID: "remote-id",
					ResourceQuota: v1.ResourceQuotaSpec{
						Hard: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(3, resource.DecimalSI),
						},
					},
				},
			},
			expectedConditions: []types.GomegaMatcher{
				ConditionMatcher(v1.NodeReady, v1.ConditionFalse),
				ConditionMatcher(v1.NodeNetworkUnavailable, v1.ConditionTrue),
			},
		}),
	)

	It("hydrates the node from virtual node, foreign cluster and remote node", func() {
		virtualNode := &offloadingv1beta1.VirtualNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeName,
				Namespace: kubeletNamespace,
			},
			Spec: offloadingv1beta1.VirtualNodeSpec{
				Labels: map[string]string{
					"virtual-label": "virtual-value",
				},
				Annotations: map[string]string{
					"virtual-annotation": "annotation-value",
				},
				Taints: []v1.Taint{{
					Key:    taintKey,
					Value:  taintValue,
					Effect: v1.TaintEffectNoSchedule,
				}},
				StorageClasses: []liqov1beta1.StorageType{{StorageClassName: "fast"}},
				ResourceQuota: v1.ResourceQuotaSpec{
					Hard: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("2"),
						v1.ResourceMemory: resource.MustParse("3Gi"),
					},
				},
				Images: []v1.ContainerImage{{
					Names:     []string{"nginx:latest"},
					SizeBytes: 1234,
				}},
			},
		}

		foreignCluster := testutil.FakeForeignCluster(foreignClusterID, &liqov1beta1.Modules{
			Offloading: liqov1beta1.Module{
				Enabled: true,
			},
			Networking: liqov1beta1.Module{
				Enabled: true,
				Conditions: []liqov1beta1.Condition{{
					Type:   liqov1beta1.NetworkConnectionStatusCondition,
					Status: liqov1beta1.ConditionStatusEstablished,
				}},
			},
		})
		foreignCluster.Status.ObservedGeneration = 1

		remoteNode := &v1.Node{
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{Type: v1.NodeReady, Status: v1.ConditionTrue},
					{Type: v1.NodeMemoryPressure, Status: v1.ConditionFalse},
					{Type: v1.NodeDiskPressure, Status: v1.ConditionTrue},
					{Type: v1.NodePIDPressure, Status: v1.ConditionFalse},
				},
				NodeInfo: v1.NodeSystemInfo{
					Architecture:            "arm64",
					OperatingSystem:         "linux",
					KernelVersion:           "6.8.0",
					KubeletVersion:          "v1.31.0",
					ContainerRuntimeVersion: "containerd://2.0.0",
					OSImage:                 "Talos Linux",
				},
			},
		}

		hydratedProvider := NewLiqoNodeProvider(&InitConfig{
			NodeName:           nodeName,
			Namespace:          kubeletNamespace,
			InternalIP:         "10.2.0.1",
			HomeConfig:         cluster.GetCfg(),
			RemoteConfig:       cluster.GetCfg(),
			RemoteClusterID:    foreignClusterID,
			CheckNetworkStatus: true,
			VirtualNode:        virtualNode,
			ForeignCluster:     foreignCluster,
			RemoteNode:         remoteNode,
		})

		hydratedNode := hydratedProvider.GetNode()

		Expect(hydratedNode.Labels).To(HaveKeyWithValue("virtual-label", "virtual-value"))
		Expect(hydratedNode.Labels).To(HaveKeyWithValue(consts.StorageAvailableLabel, trueValue))
		Expect(hydratedNode.Annotations).To(HaveKeyWithValue("virtual-annotation", "annotation-value"))
		Expect(hydratedNode.Spec.Taints).To(ContainElement(v1.Taint{
			Key:    taintKey,
			Value:  taintValue,
			Effect: v1.TaintEffectNoSchedule,
		}))
		Expect(hydratedNode.Spec.Taints).To(ContainElement(v1.Taint{
			Key:    consts.VirtualNodeTolerationKey,
			Value:  trueValue,
			Effect: v1.TaintEffectNoExecute,
		}))
		Expect(hydratedNode.Status.Capacity.Cpu().Cmp(resource.MustParse("2"))).To(Equal(0))
		Expect(hydratedNode.Status.Capacity.Memory().Cmp(resource.MustParse("3Gi"))).To(Equal(0))
		Expect(hydratedNode.Status.Allocatable.Cpu().Cmp(resource.MustParse("2"))).To(Equal(0))
		Expect(hydratedNode.Status.Allocatable.Memory().Cmp(resource.MustParse("3Gi"))).To(Equal(0))
		Expect(hydratedNode.Status.Images).To(Equal(virtualNode.Spec.Images))
		Expect(hydratedNode.Status.NodeInfo).To(Equal(remoteNode.Status.NodeInfo))

		Expect(lookupCondition(hydratedNode.Status.Conditions, v1.NodeReady).Status).To(Equal(v1.ConditionTrue))
		Expect(lookupCondition(hydratedNode.Status.Conditions, v1.NodeMemoryPressure).Status).To(Equal(v1.ConditionFalse))
		Expect(lookupCondition(hydratedNode.Status.Conditions, v1.NodeDiskPressure).Status).To(Equal(v1.ConditionTrue))
		Expect(lookupCondition(hydratedNode.Status.Conditions, v1.NodePIDPressure).Status).To(Equal(v1.ConditionFalse))
		Expect(lookupCondition(hydratedNode.Status.Conditions, v1.NodeNetworkUnavailable).Status).To(Equal(v1.ConditionFalse))
	})

	It("hydrates the node when virtual node labels and annotations are nil", func() {
		virtualNode := &offloadingv1beta1.VirtualNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeName,
				Namespace: kubeletNamespace,
			},
			Spec: offloadingv1beta1.VirtualNodeSpec{
				ResourceQuota: v1.ResourceQuotaSpec{
					Hard: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("2"),
						v1.ResourceMemory: resource.MustParse("3Gi"),
					},
				},
			},
		}

		hydratedProvider := NewLiqoNodeProvider(&InitConfig{
			NodeName:           nodeName,
			Namespace:          kubeletNamespace,
			InternalIP:         "10.0.0.1",
			HomeConfig:         cluster.GetCfg(),
			RemoteConfig:       cluster.GetCfg(),
			RemoteClusterID:    foreignClusterID,
			CheckNetworkStatus: true,
			VirtualNode:        virtualNode,
			ForeignCluster:     nil,
			RemoteNode:         nil,
		})

		hydratedNode := hydratedProvider.GetNode()

		Expect(hydratedNode.Labels).To(HaveKeyWithValue(consts.StorageAvailableLabel, "false"))
		Expect(hydratedNode.Annotations).ToNot(BeNil())
		Expect(hydratedNode.Annotations).ToNot(HaveKey("virtual-annotation"))
	})

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

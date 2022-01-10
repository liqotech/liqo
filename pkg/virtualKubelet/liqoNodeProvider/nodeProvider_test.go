// Copyright 2019-2022 The Liqo Authors
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
	"fmt"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250

	nodeName            = "node-name"
	resourceRequestName = "resource-request-name"
	foreignClusterID    = "foreign-id"
	kubeletNamespace    = "default"
)

func TestNodeProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeProvider Suite")
}

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

		cluster, _, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
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
		})

		_, err = client.CoreV1().Nodes().Create(ctx, nodeProvider.GetNode(), metav1.CreateOptions{})
		Expect(err).To(BeNil())

		nodeProvider.NotifyNodeStatus(ctx, func(node *v1.Node) {
			nodeChan <- node
			client.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{})
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
		resourceOffer      *sharingv1alpha1.ResourceOffer
		tunnelEndpoint     *netv1alpha1.TunnelEndpoint
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

			if c.resourceOffer != nil {
				unstructResourceOffer, err := runtime.DefaultUnstructuredConverter.ToUnstructured(c.resourceOffer)
				Expect(err).To(BeNil())
				_, err = dynClient.Resource(sharingv1alpha1.ResourceOfferGroupVersionResource).
					Namespace(kubeletNamespace).Create(ctx, &unstructured.Unstructured{
					Object: unstructResourceOffer,
				}, metav1.CreateOptions{})
				Expect(err).To(BeNil())
			}

			if c.tunnelEndpoint != nil {
				c.tunnelEndpoint.Labels = map[string]string{
					consts.ClusterIDLabelName: nodeProvider.foreignClusterID,
				}
				unstructTep, err := runtime.DefaultUnstructuredConverter.ToUnstructured(c.tunnelEndpoint)
				Expect(err).To(BeNil())
				unstruct, err := dynClient.Resource(netv1alpha1.TunnelEndpointGroupVersionResource).
					Namespace(kubeletNamespace).Create(ctx, &unstructured.Unstructured{
					Object: unstructTep,
				}, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				unstruct.Object["status"] = unstructTep["status"]

				_, err = dynClient.Resource(netv1alpha1.TunnelEndpointGroupVersionResource).
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

		Entry("update from ResourceOffer", nodeProviderTestcase{
			resourceOffer: &sharingv1alpha1.ResourceOffer{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ResourceOffer",
					APIVersion: sharingv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceRequestName,
					Namespace: kubeletNamespace,
					Labels: map[string]string{
						consts.ReplicationOriginLabel: foreignClusterID,
					},
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterId: "remote-id",
					ResourceQuota: v1.ResourceQuotaSpec{
						Hard: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(3, resource.DecimalSI),
						},
					},
				},
			},
			tunnelEndpoint: nil,
			expectedConditions: []types.GomegaMatcher{
				ConditionMatcher(v1.NodeReady, v1.ConditionFalse),
				ConditionMatcher(v1.NodeMemoryPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodeDiskPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodePIDPressure, v1.ConditionFalse),
				ConditionMatcher(v1.NodeNetworkUnavailable, v1.ConditionTrue),
			},
		}),

		Entry("update from TunnelEndpoint", nodeProviderTestcase{
			resourceOffer: nil,
			tunnelEndpoint: &netv1alpha1.TunnelEndpoint{
				TypeMeta: metav1.TypeMeta{
					Kind:       "TunnelEndpoint",
					APIVersion: netv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tep",
					Namespace: kubeletNamespace,
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
			expectedConditions: []types.GomegaMatcher{
				ConditionMatcher(v1.NodeReady, v1.ConditionFalse),
				ConditionMatcher(v1.NodeMemoryPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodeDiskPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodePIDPressure, v1.ConditionTrue),
				ConditionMatcher(v1.NodeNetworkUnavailable, v1.ConditionFalse),
			},
		}),

		Entry("update from both", nodeProviderTestcase{
			resourceOffer: &sharingv1alpha1.ResourceOffer{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ResourceOffer",
					APIVersion: sharingv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceRequestName,
					Namespace: kubeletNamespace,
					Labels: map[string]string{
						consts.ReplicationOriginLabel: foreignClusterID,
					},
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterId: "remote-id",
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
					Name:      "tep",
					Namespace: kubeletNamespace,
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

		err := nodeProvider.patchLabels(labels)
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

		err = nodeProvider.patchLabels(labels)
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

		err = nodeProvider.patchLabels(labels)
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

	Context("Node Cleanup", func() {

		It("Cordon Node", func() {

			err = nodeProvider.cordonNode(ctx)
			Expect(err).ToNot(HaveOccurred())

			client := kubernetes.NewForConfigOrDie(cluster.GetCfg())
			Eventually(func() bool {
				node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return node.Spec.Unschedulable
			}, timeout, interval).Should(BeTrue())

		})

		It("Drain Node", func() {

			client := kubernetes.NewForConfigOrDie(cluster.GetCfg())

			By("creating pods on our virtual node")

			nPods := 10
			for i := 0; i < nPods; i++ {
				// put some pods to our node, some other in other nodes
				var nodeName string
				if i%2 == 0 {
					nodeName = nodeProvider.nodeName
				} else {
					nodeName = "other-node"
				}

				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("pod-%v", i),
						Namespace: v1.NamespaceDefault,
					},
					Spec: v1.PodSpec{
						NodeName: nodeName,
						Containers: []v1.Container{
							{
								Name:  "nginx",
								Image: "nginx",
							},
						},
					},
				}
				_, err = client.CoreV1().Pods(v1.NamespaceDefault).Create(ctx, pod, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
			}

			By("Draining node")

			// set a deadline for the draining
			drainCtx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
			defer cancel()

			// the drain function needs to be launched in a different goroutine since
			// it is blocking until the pods deletion
			completed := false
			go func() {
				err := nodeProvider.drainNode(drainCtx)
				if err == nil {
					completed = true
				}
			}()

			Eventually(func() bool {
				podList, err := client.CoreV1().Pods(v1.NamespaceDefault).List(ctx, metav1.ListOptions{
					FieldSelector: fields.SelectorFromSet(fields.Set{
						"spec.nodeName": nodeProvider.nodeName,
					}).String(),
				})
				if err != nil {
					return true
				}

				// check if every pod has a deletion timestamp set, if it is, the eviction has been created
				for i := range podList.Items {
					if podList.Items[i].GetDeletionTimestamp().IsZero() {
						return true
					}
					// delete the evicted pods to make the drain function to terminate,
					// we have to do it manually since no API server is running
					Expect(client.CoreV1().Pods(v1.NamespaceDefault).Delete(ctx, podList.Items[i].Name, metav1.DeleteOptions{
						GracePeriodSeconds: pointer.Int64Ptr(0),
					})).ToNot(HaveOccurred())
				}
				return false
			}, timeout, interval).Should(BeFalse())

			// the drain function has completed successfully
			Eventually(func() bool {
				return completed
			}, timeout, interval).Should(BeTrue())

			By("Checking that the pods on other nodes are still alive")

			podList, err := client.CoreV1().Pods(v1.NamespaceDefault).List(ctx, metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(podList.Items)).To(BeNumerically("==", nPods/2))
			for _, pod := range podList.Items {
				Expect(pod.Spec.NodeName).ToNot(Equal(nodeProvider.nodeName))
				Expect(pod.GetDeletionTimestamp().IsZero()).To(BeTrue())
			}

		})

	})

})

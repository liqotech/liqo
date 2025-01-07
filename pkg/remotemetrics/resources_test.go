// Copyright 2019-2025 The Liqo Authors
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

package remotemetrics

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Context("Resources", func() {

	var cl client.Client
	var getter ResourceGetter
	var ctx context.Context

	var getNode = func(name string, conditionReady corev1.ConditionStatus) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: conditionReady,
					},
				},
			},
		}
	}

	var getNamespace = func(name, originalName, clusterID string) *corev1.Namespace {
		return &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					consts.RemoteNamespaceOriginalNameAnnotationKey: originalName,
				},
				Labels: map[string]string{
					consts.RemoteClusterID: clusterID,
				},
			},
		}
	}

	var getPod = func(name, namespace, clusterID, node string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					forge.LiqoOriginClusterIDKey: clusterID,
				},
			},
			Spec: corev1.PodSpec{
				NodeName: node,
			},
		}
	}

	BeforeEach(func() {
		ctx = context.Background()

		cl = fake.NewClientBuilder().WithObjects(
			getNode("node1", corev1.ConditionTrue),
			getNode("node2", corev1.ConditionTrue),
			getNode("node3", corev1.ConditionFalse),
			getNamespace("ns1", "origNs1", "cluster1"),
			getNamespace("ns2", "origNs2", "cluster2"),
			getPod("pod1", "ns1", "cluster1", "node1"),
			getPod("pod2", "ns1", "cluster1", "node2"),
			getPod("pod3", "ns2", "cluster2", "node1"),
			getPod("pod4", "ns2", "cluster2", "node2"),
		).Build()

		getter = NewResourceGetter(cl)
	})

	It("should retrieve nodes", func() {
		nodes := getter.GetNodeNames(ctx)
		Expect(nodes).To(HaveLen(2))
		Expect(nodes).To(ContainElements("node1", "node2"))
	})

	It("should retrieve namespaces", func() {
		namespaces := getter.GetNamespaces(ctx, "cluster1")
		Expect(namespaces).To(HaveLen(1))
		Expect(namespaces).To(ContainElements(MappedNamespace{
			Namespace:    "ns1",
			OriginalName: "origNs1",
		}))
		Expect(namespaces).ToNot(ContainElements(MappedNamespace{
			Namespace:    "ns2",
			OriginalName: "origNs2",
		}))
	})

	It("should retrieve pods", func() {
		pods := getter.GetPodNames(ctx, "cluster1", "node1")
		Expect(pods).To(HaveLen(1))
		Expect(pods).To(ContainElements("pod1"))
		Expect(pods).ToNot(ContainElements("pod2", "pod3", "pod4"))

		pods = getter.GetPodNames(ctx, "cluster1", "node2")
		Expect(pods).To(HaveLen(1))
		Expect(pods).To(ContainElements("pod2"))
		Expect(pods).ToNot(ContainElements("pod1", "pod3", "pod4"))

		pods = getter.GetPodNames(ctx, "cluster2", "node1")
		Expect(pods).To(HaveLen(1))
		Expect(pods).To(ContainElements("pod3"))
		Expect(pods).ToNot(ContainElements("pod1", "pod2", "pod4"))

		pods = getter.GetPodNames(ctx, "cluster2", "node2")
		Expect(pods).To(HaveLen(1))
		Expect(pods).To(ContainElements("pod4"))
		Expect(pods).ToNot(ContainElements("pod1", "pod2", "pod3"))
	})

})

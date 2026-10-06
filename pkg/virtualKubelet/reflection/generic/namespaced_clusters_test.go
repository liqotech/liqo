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

package generic

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Per-cluster reflection annotations", func() {
	const (
		allowClusters = consts.AllowReflectionClustersAnnotationKey
		skipClusters  = consts.SkipReflectionClustersAnnotationKey
		allow         = consts.AllowReflectionAnnotationKey
		skip          = consts.SkipReflectionAnnotationKey
	)

	BeforeEach(func() {
		local, remote, node, ip := forge.LocalCluster, forge.RemoteCluster, forge.LiqoNodeName, forge.LiqoNodeIP
		forge.Init("local-cluster", "cluster-a", "liqo-cluster-a", "1.1.1.1")
		DeferCleanup(func() { forge.Init(local, remote, node, ip) })
	})

	object := func(annotations map[string]string) metav1.Object {
		return &metav1.ObjectMeta{Name: "object", Namespace: "namespace", Annotations: annotations}
	}

	DescribeTable("the ShouldSkipReflection function",
		func(annotations map[string]string, skipWithAllowList, skipWithDenyList bool) {
			for policy, expected := range map[offloadingv1beta1.ReflectionType]bool{
				offloadingv1beta1.AllowList: skipWithAllowList, offloadingv1beta1.DenyList: skipWithDenyList,
			} {
				reflector := NamespacedReflector{reflectionType: policy}
				skipped, err := reflector.ShouldSkipReflection(object(annotations))
				Expect(err).ToNot(HaveOccurred())
				Expect(skipped).To(Equal(expected), "policy %s", policy)
			}
		},
		Entry("no annotations: the reflection policy applies", nil, true, false),
		Entry("allowed towards the remote cluster", map[string]string{allowClusters: "cluster-a,cluster-b"}, false, false),
		Entry("allowed towards other clusters only", map[string]string{allowClusters: "cluster-b"}, true, true),
		Entry("allowed towards no cluster", map[string]string{allowClusters: ""}, true, true),
		Entry("skipped towards the remote cluster", map[string]string{skipClusters: "cluster-b, cluster-a"}, true, true),
		Entry("skipped towards other clusters only: the reflection policy applies",
			map[string]string{skipClusters: "cluster-b"}, true, false),
		Entry("both allowed and skipped towards the remote cluster: skip prevails",
			map[string]string{allowClusters: "cluster-a", skipClusters: "cluster-a"}, true, true),
		Entry("skipped towards other clusters, and allowed towards the remote one",
			map[string]string{allowClusters: "cluster-a", skipClusters: "cluster-b"}, false, false),
		Entry("allowed towards the remote cluster, although globally skipped: the per-cluster annotation prevails",
			map[string]string{allowClusters: "cluster-a", skip: "true"}, false, false),
		Entry("allowed towards other clusters only, although globally allowed: the per-cluster annotation prevails",
			map[string]string{allowClusters: "cluster-b", allow: "true"}, true, true),
		Entry("lists with spaces and empty entries", map[string]string{allowClusters: " , cluster-a ,"}, false, false),
		Entry("cluster IDs are matched exactly", map[string]string{allowClusters: "cluster-a-1"}, true, true),
	)

	DescribeTable("the ForcedAllowOrSkip function",
		func(annotations map[string]string, expected *bool, expectError bool) {
			reflector := NamespacedReflector{reflectionType: offloadingv1beta1.DenyList}
			skipped, err := reflector.ForcedAllowOrSkip(object(annotations))
			if expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(skipped).To(Equal(expected))
		},
		Entry("no annotations", nil, nil, false),
		Entry("allowed towards the remote cluster", map[string]string{allowClusters: "cluster-a"}, ptr.To(false), false),
		Entry("allowed towards other clusters only", map[string]string{allowClusters: "cluster-b"}, ptr.To(true), false),
		Entry("skipped towards the remote cluster", map[string]string{skipClusters: "cluster-a"}, ptr.To(true), false),
		Entry("skipped towards other clusters only", map[string]string{skipClusters: "cluster-b"}, nil, false),
		Entry("skipped towards other clusters only, and globally allowed", map[string]string{skipClusters: "cluster-b", allow: "true"},
			ptr.To(false), false),
		Entry("both global annotations, without per-cluster ones", map[string]string{allow: "true", skip: "true"}, nil, true),
		Entry("both global annotations, but allowed towards the remote cluster",
			map[string]string{allow: "true", skip: "true", allowClusters: "cluster-a"}, ptr.To(false), false),
	)
})

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

package mutate

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Pod labels mutations", func() {
	Describe("shadow pod label", func() {
		const nodeName = "node"

		var (
			ctx           context.Context
			clientBuilder fake.ClientBuilder

			original, mutated corev1.Pod
			node              corev1.Node

			err error
		)

		BeforeEach(func() {
			ctx = context.Background()
			clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
			original = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar", Labels: map[string]string{"other-key": "other-value"}}}
			node = corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
		})

		JustBeforeEach(func() {
			mutated = *original.DeepCopy()
			c := clientBuilder.WithObjects(&node).Build()
			err = mutateShadowPodLabel(ctx, c, &mutated)
		})

		Context("the pod has not yet been assigned to a node", func() {
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should not mutate the pod, including the labels", func() { Expect(mutated).To(Equal(original)) })
		})

		Context("the pod has been assigned to a non-existing node", func() {
			BeforeEach(func() { original.Spec.NodeName = "not-existing" })
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should not mutate the pod, including the labels", func() { Expect(mutated).To(Equal(original)) })
		})

		Context("the pod has been assigned to an existing node", func() {
			BeforeEach(func() { original.Spec.NodeName = nodeName })

			When("the node is a standard worker", func() {
				When("the pod does not have the shadow pod label", func() {
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should not mutate the pod, including the labels", func() { Expect(mutated).To(Equal(original)) })
				})
				When("the pod has the shadow pod label", func() {
					BeforeEach(func() { original.Labels[consts.LocalPodLabelKey] = consts.LocalPodLabelValue })
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should remove that label", func() { Expect(mutated.Labels).ToNot(HaveKey(consts.LocalPodLabelKey)) })
				})
			})

			When("the node is a liqo node", func() {
				BeforeEach(func() { node.Labels = map[string]string{consts.TypeLabel: consts.TypeNode} })
				When("the pod does not have the shadow pod label", func() {
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should add the shadow pod label", func() {
						Expect(mutated.Labels).To(HaveKeyWithValue(consts.LocalPodLabelKey, consts.LocalPodLabelValue))
					})
					It("should not mutate other existing pod label", func() {
						Expect(mutated.Labels).To(HaveKeyWithValue("other-key", "other-value"))
					})
				})
				When("the pod has the shadow pod label", func() {
					BeforeEach(func() { original.Labels[consts.LocalPodLabelKey] = consts.LocalPodLabelValue })
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should not mutate the pod, including the labels", func() { Expect(mutated).To(Equal(original)) })
				})
			})
		})
	})
})

// Copyright 2019-2021 The Liqo Authors
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

package shadowpodctrl_test

import (
	"bytes"
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	shadowpodctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/shadowpod-controller"
)

var _ = Describe("Reconcile", func() {
	const (
		shadowPodNamespace string = "default"
		shadowPodName      string = "test-shadow-pod"
	)

	var (
		req = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      shadowPodName,
				Namespace: shadowPodNamespace,
			},
		}
		ctx    context.Context
		res    ctrl.Result
		err    error
		buffer *bytes.Buffer

		testShadowPod vkv1alpha1.ShadowPod
		testPod       corev1.Pod
	)

	BeforeEach(func() {
		ctx = context.TODO()
		buffer = &bytes.Buffer{}
		klog.SetOutput(buffer)

		testShadowPod = vkv1alpha1.ShadowPod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shadowPodName,
				Namespace: shadowPodNamespace,
				Labels: map[string]string{
					"label1-key": "label1-value",
					"label2-key": "label2-value",
				},
				Annotations: map[string]string{
					"annotation1-key": "annotation1-value",
					"annotation2-key": "annotation2-value",
				},
			},
			Spec: vkv1alpha1.ShadowPodSpec{
				Pod: corev1.PodSpec{Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx",
					},
				}},
			},
		}

		testPod = corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testShadowPod.Name,
				Namespace: testShadowPod.Namespace,
			},
			Spec: testShadowPod.Spec.Pod,
		}
	})

	JustBeforeEach(func() {
		r := &shadowpodctrl.Reconciler{
			Client: k8sClient,
			Scheme: scheme.Scheme,
		}

		res, err = r.Reconcile(ctx, req)
		klog.Flush()
	})

	AfterEach(func() {
		deleteAllShadowPodsAndPods(ctx, shadowPodNamespace)
	})

	When("shadowpod is not found", func() {
		It("should ignore it", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeZero())
			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("skip: shadowpod %v not found", req.NamespacedName)))
		})
	})

	When("pod has been already created", func() {
		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, &testShadowPod)).To(Succeed())
			Expect(k8sClient.Create(ctx, &testPod)).To(Succeed())
		})

		It("should ignore it", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeZero())
			Expect(buffer.String()).To(ContainSubstring("skip: pod \"default/test-shadow-pod\" already running"))
		})
	})

	When("create pod", func() {
		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, &testShadowPod)).To(Succeed())
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeZero())
		})

		It("should log a message", func() {
			Expect(buffer.String()).To(ContainSubstring("created pod \"default/test-shadow-pod\" for shadowpod \"default/test-shadow-pod\""))
		})

		It("should set owner reference", func() {
			pod := corev1.Pod{}
			Expect(k8sClient.Get(ctx, req.NamespacedName, &pod)).To(Succeed())
			Expect(pod.GetOwnerReferences()).To(ContainElement(
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal("test-shadow-pod"),
				})),
			)
		})

		It("should set pod metadata to shadowpod metadata", func() {
			pod := corev1.Pod{}
			Expect(k8sClient.Get(ctx, req.NamespacedName, &pod)).To(Succeed())
			Expect(pod.GetName()).To(Equal(testShadowPod.GetName()))
			Expect(pod.GetLabels()).To(Equal(testShadowPod.GetLabels()))
			Expect(pod.GetAnnotations()).To(Equal(testShadowPod.GetAnnotations()))
		})

		It("should set pod spec to shadowpod pod spec", func() {
			pod := corev1.Pod{}
			Expect(k8sClient.Get(ctx, req.NamespacedName, &pod)).To(Succeed())
			Expect(pod.Spec.Containers).To(HaveLen(1))
			podContainer := pod.Spec.Containers[0]
			shadowPodContainer := testShadowPod.Spec.Pod.Containers[0]
			Expect(podContainer.Name).To(Equal(shadowPodContainer.Name))
			Expect(podContainer.Image).To(Equal(shadowPodContainer.Image))
		})
	})
})

func deleteAllShadowPodsAndPods(ctx context.Context, ns string) {
	Expect(k8sClient.DeleteAllOf(ctx, &vkv1alpha1.ShadowPod{}, client.InNamespace(ns))).Should(Succeed())
	Expect(k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(ns))).Should(Succeed())
}

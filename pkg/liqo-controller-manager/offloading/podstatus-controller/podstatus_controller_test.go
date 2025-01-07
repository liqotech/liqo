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

package podstatusctrl

import (
	"bytes"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/indexer"
)

var _ = Describe("PodStatusController", func() {
	const (
		ns            string = "default"
		nodeName      string = "node-test"
		liqoNodeName  string = "liqo-node-test"
		liqoNodeName2 string = "liqo-node-test-2"
		podName       string = "test-pod"
		localPodName  string = "test-local-pod"
		localPodName2 string = "test-local-pod-2"
	)

	var (
		ctx               context.Context
		err               error
		buffer            *bytes.Buffer
		fakeClient        client.WithWatch
		fakeClientBuilder *fake.ClientBuilder
		localPod          *corev1.Pod
		localPod2         *corev1.Pod
		liqoNode2         *corev1.Node

		reqLiqoNode = ctrl.Request{NamespacedName: types.NamespacedName{Name: liqoNodeName}}

		reqLocalPod = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      localPodName,
				Namespace: ns,
			},
		}

		reqLocalPod2 = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      localPodName2,
				Namespace: ns,
			},
		}

		newLiqoNode = func(name string, condReadyStatus corev1.ConditionStatus) *corev1.Node {
			return &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{consts.TypeLabel: consts.TypeNode},
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: condReadyStatus,
						},
					},
				},
			}
		}

		newPod = func() *corev1.Pod {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: ns,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
					NodeName: nodeName,
				},
			}

			return pod
		}

		newLocalPod = func() *corev1.Pod {
			pod := newPod()
			pod.Name = localPodName
			pod.Labels = labels.Merge(pod.Labels, labels.Set{consts.LocalPodLabelKey: consts.LocalPodLabelValue})
			pod.Spec.NodeName = liqoNodeName
			return pod
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		buffer = &bytes.Buffer{}
		klog.SetOutput(buffer)

		fakeClientBuilder = fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithIndex(&corev1.Pod{}, indexer.FieldNodeNameFromPod, indexer.ExtractNodeName)

		// Pod scheduled on reconciled liqo node
		localPod = newLocalPod()
		localPod.Name = localPodName
		localPod.Spec.NodeName = liqoNodeName

		// Create extra pod scheduled on an another liqo node different from the one reconciled
		localPod2 = newLocalPod()
		localPod2.Name = localPodName2
		localPod2.Spec.NodeName = liqoNodeName2

		// Create extra node (always ready)
		liqoNode2 = newLiqoNode(liqoNodeName2, corev1.ConditionTrue)

	})

	JustBeforeEach(func() {
		r := &PodStatusReconciler{
			Client: fakeClient,
			Scheme: scheme.Scheme,
		}
		_, err = r.Reconcile(ctx, reqLiqoNode)
		Expect(err).NotTo(HaveOccurred())
		klog.Flush()
	})

	When("liqo node not ready", func() {
		BeforeEach(func() {
			fakeClient = fakeClientBuilder.WithObjects(newLiqoNode(liqoNodeName, corev1.ConditionFalse), liqoNode2).Build()
		})

		When("remote unavailable label not present", func() {
			BeforeEach(func() {
				fakeClient = fakeClientBuilder.WithObjects(localPod, localPod2).Build()
			})

			It("should add remote unavailable label to local offloaded pods", func() {
				localPodAfter := corev1.Pod{}
				localPod2After := corev1.Pod{}
				Expect(fakeClient.Get(ctx, reqLocalPod.NamespacedName, &localPodAfter)).To(Succeed())
				Expect(fakeClient.Get(ctx, reqLocalPod2.NamespacedName, &localPod2After)).To(Succeed())
				Expect(localPodAfter.Labels).To(HaveKeyWithValue(consts.RemoteUnavailableKey, consts.RemoteUnavailableValue))
				Expect(localPod2After.Labels).ToNot(HaveKeyWithValue(consts.RemoteUnavailableKey, consts.RemoteUnavailableValue))
			})
		})

		When("remote unavailable label present", func() {
			BeforeEach(func() {
				localPod.Labels = labels.Merge(localPod.Labels, labels.Set{consts.RemoteUnavailableKey: consts.RemoteUnavailableValue})
				fakeClient = fakeClientBuilder.WithObjects(localPod, localPod2).Build()
			})

			It("should keep remote unavailable label to local offloaded pods", func() {
				localPodAfter := corev1.Pod{}
				localPod2After := corev1.Pod{}
				Expect(fakeClient.Get(ctx, reqLocalPod.NamespacedName, &localPodAfter)).To(Succeed())
				Expect(fakeClient.Get(ctx, reqLocalPod2.NamespacedName, &localPod2After)).To(Succeed())
				Expect(localPodAfter.Labels).To(HaveKeyWithValue(consts.RemoteUnavailableKey, consts.RemoteUnavailableValue))
				Expect(localPod2After.Labels).ToNot(HaveKeyWithValue(consts.RemoteUnavailableKey, consts.RemoteUnavailableValue))
			})
		})
	})

	When("liqo node ready", func() {
		BeforeEach(func() {
			fakeClient = fakeClientBuilder.WithObjects(newLiqoNode(liqoNodeName, corev1.ConditionTrue), liqoNode2).Build()
		})

		When("remote unavailable label not present", func() {
			BeforeEach(func() {
				fakeClient = fakeClientBuilder.WithObjects(localPod, localPod2).Build()
			})

			It("should not add remote unavailable label to local offloaded pods", func() {
				localPodAfter := corev1.Pod{}
				localPod2After := corev1.Pod{}
				Expect(fakeClient.Get(ctx, reqLocalPod.NamespacedName, &localPodAfter)).To(Succeed())
				Expect(fakeClient.Get(ctx, reqLocalPod2.NamespacedName, &localPod2After)).To(Succeed())
				Expect(localPodAfter.Labels).ToNot(HaveKeyWithValue(consts.RemoteUnavailableKey, consts.RemoteUnavailableValue))
				Expect(localPod2After.Labels).ToNot(HaveKeyWithValue(consts.RemoteUnavailableKey, consts.RemoteUnavailableValue))
			})
		})

		When("remote unavailable label present", func() {
			BeforeEach(func() {
				localPod.Labels = labels.Merge(localPod.Labels, labels.Set{consts.RemoteUnavailableKey: consts.RemoteUnavailableValue})
				fakeClient = fakeClientBuilder.WithObjects(localPod, localPod2).Build()
			})

			It("should remove remote unavailable label to local offloaded pods", func() {
				localPod := corev1.Pod{}
				localPod2 := corev1.Pod{}
				Expect(fakeClient.Get(ctx, reqLocalPod.NamespacedName, &localPod)).To(Succeed())
				Expect(fakeClient.Get(ctx, reqLocalPod2.NamespacedName, &localPod2)).To(Succeed())
				Expect(localPod.Labels).ToNot(HaveKeyWithValue(consts.RemoteUnavailableKey, consts.RemoteUnavailableValue))
				Expect(localPod2.Labels).ToNot(HaveKeyWithValue(consts.RemoteUnavailableKey, consts.RemoteUnavailableValue))
			})
		})
	})
})

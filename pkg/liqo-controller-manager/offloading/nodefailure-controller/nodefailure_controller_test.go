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

package nodefailurectrl

import (
	"bytes"
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/indexer"
)

var _ = Describe("NodeFailureController", func() {
	const (
		ns            string = "default"
		nodeName      string = "test-node"
		podName       string = "test-pod"
		remotePodName string = "test-remote-pod"
	)

	var (
		ctx               context.Context
		err               error
		buffer            *bytes.Buffer
		fakeClientBuilder *fake.ClientBuilder
		fakeClient        client.WithWatch
		timestamp         metav1.Time

		reqNode = ctrl.Request{NamespacedName: types.NamespacedName{Name: nodeName}}
		reqPod  = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      podName,
				Namespace: ns,
			},
		}
		reqRemotePod = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      remotePodName,
				Namespace: ns,
			},
		}

		newNode = func(statusReady bool) *corev1.Node {
			status := corev1.ConditionFalse
			if statusReady == true {
				status = corev1.ConditionTrue
			}

			return &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: status,
						},
					},
				},
			}
		}

		newPod = func(isTerminating bool) *corev1.Pod {
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

			if isTerminating {
				pod.DeletionTimestamp = &timestamp
				pod.Finalizers = []string{"test"}
			}

			return pod
		}

		newRemotePod = func(isTerminating bool) *corev1.Pod {
			pod := newPod(isTerminating)
			pod.Name = remotePodName
			pod.Labels = map[string]string{
				consts.ManagedByLabelKey: consts.ManagedByShadowPodValue,
			}
			pod.OwnerReferences = []metav1.OwnerReference{
				{
					Kind:               "ShadowPod",
					Name:               remotePodName,
					BlockOwnerDeletion: pointer.Bool(true),
					Controller:         pointer.Bool(true),
				},
			}
			return pod
		}
	)

	BeforeEach(func() {
		ctx = context.TODO()
		buffer = &bytes.Buffer{}
		klog.SetOutput(buffer)
		timestamp = metav1.Now()

		fakeClientBuilder = fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithIndex(&corev1.Pod{}, indexer.FieldNodeNameFromPod, indexer.ExtractNodeName)
	})

	JustBeforeEach(func() {
		r := &NodeFailureReconciler{
			Client: fakeClient,
			Scheme: scheme.Scheme,
		}
		_, err = r.Reconcile(ctx, reqNode)
		Expect(err).NotTo(HaveOccurred())
		klog.Flush()
	})

	When("node ready, pod running", func() {
		BeforeEach(func() {
			fakeClient = fakeClientBuilder.
				WithObjects(newNode(true), newRemotePod(false), newPod(false)).
				Build()
		})

		It("should get pod and remotePod", func() {
			pod := corev1.Pod{}
			Expect(fakeClient.Get(ctx, reqPod.NamespacedName, &pod)).To(Succeed())
			Expect(fakeClient.Get(ctx, reqRemotePod.NamespacedName, &pod)).To(Succeed())
		})
	})

	When("node ready, pod terminating", func() {
		BeforeEach(func() {
			fakeClient = fakeClientBuilder.
				WithObjects(newNode(true), newRemotePod(true), newPod(true)).
				Build()
		})

		It("should get pod and remotePod", func() {
			pod := corev1.Pod{}
			Expect(fakeClient.Get(ctx, reqPod.NamespacedName, &pod)).To(Succeed())
			Expect(fakeClient.Get(ctx, reqRemotePod.NamespacedName, &pod)).To(Succeed())
		})
	})

	When("node not ready, pod terminating", func() {
		BeforeEach(func() {
			fakeClient = fakeClientBuilder.
				WithObjects(newNode(false), newRemotePod(true), newPod(true)).
				Build()
		})

		It("should get pod, but not remotePod", func() {
			pod := corev1.Pod{}
			Expect(fakeClient.Get(ctx, reqPod.NamespacedName, &pod)).To(Succeed())
			// the fke client requires a finalizer on the pod, then it is not deleted...
			Expect(fakeClient.Get(ctx, reqRemotePod.NamespacedName, &pod)).To(Succeed())
			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("pod %q running on failed node %s deleted", reqRemotePod.NamespacedName, nodeName)))
		})
	})
})

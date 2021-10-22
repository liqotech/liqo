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

package workload_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
	"k8s.io/utils/trace"

	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/workload"
)

var _ = Describe("Pod Reflection Tests", func() {
	Describe("the NewPodReflector function", func() {
		It("should not return a nil reflector", func() {
			reflector := workload.NewPodReflector(nil, nil, nil, 0)
			Expect(reflector).ToNot(BeNil())
			Expect(reflector.Reflector).ToNot(BeNil())
		})
	})

	Describe("orphan pod handling", func() {
		const PodName = "name"

		var (
			client    kubernetes.Interface
			reflector *workload.PodReflector
			fallback  manager.FallbackReflector

			local corev1.Pod
			err   error
		)

		BeforeEach(func() { local = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: "not-existing"}} })

		JustBeforeEach(func() {
			client = fake.NewSimpleClientset(&local)
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)

			reflector = workload.NewPodReflector(nil, nil, nil, 0)
			fallback = reflector.NewFallback(options.New(client, factory.Core().V1().Pods()).WithHandlerFactory(FakeEventHandler))

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())

			err = fallback.Handle(trace.ContextWithTrace(ctx, trace.New("Pod")), types.NamespacedName{Namespace: LocalNamespace, Name: PodName})
		})

		Context("object reflection", func() {
			When("the local object does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should not be created", func() { Expect(GetPodError(client, LocalNamespace, PodName)).To(BeNotFound()) })
			})

			When("the local object does exist", func() {
				BeforeEach(func() { local = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: LocalNamespace}} })

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should correctly mark the pod as rejected (failed)", func() {
					localAfter := GetPod(client, LocalNamespace, PodName)
					Expect(localAfter.Status.Phase).To(Equal(corev1.PodFailed))
					Expect(localAfter.Status.Reason).To(Equal(forge.PodRejectedReason))
				})

				When("the local object does exist and it is owned by a daemonset", func() {
					BeforeEach(func() {
						local.OwnerReferences = []metav1.OwnerReference{{
							Kind: "DaemonSet", APIVersion: "apps/v1", Name: "foo", UID: "bar", Controller: pointer.Bool(true)}}
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should correctly mark the pod as rejected (pending)", func() {
						localAfter := GetPod(client, LocalNamespace, PodName)
						Expect(localAfter.Status.Phase).To(Equal(corev1.PodPending))
						Expect(localAfter.Status.Reason).To(Equal(forge.PodRejectedReason))
					})
				})

				When("the local object is terminating", func() {
					BeforeEach(func() { local.DeletionTimestamp = &metav1.Time{Time: time.Now()} })

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should correctly remove the pod", func() { Expect(GetPodError(client, LocalNamespace, PodName)).To(BeNotFound()) })
				})
			})
		})

		Context("keys retrieval", func() {
			var keys []types.NamespacedName

			JustBeforeEach(func() { keys = fallback.Keys(LocalNamespace, RemoteNamespace) })

			When("no objects are present", func() {
				It("should return an empty array", func() { Expect(keys).To(HaveLen(0)) })
			})

			When("an object is present", func() {
				BeforeEach(func() { local = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: LocalNamespace}} })
				It("should return the key for that element", func() {
					Expect(keys).To(ConsistOf(types.NamespacedName{Namespace: LocalNamespace, Name: PodName}))
				})
			})
		})

		Context("readiness check", func() {
			When("the reflector is not ready", func() {
				It("should return false", func() { Expect(fallback.Ready()).To(BeFalse()) })
			})

			When("the reflector is ready", func() {
				JustBeforeEach(func() { reflector.StartAllNamespaces() })
				It("should return true", func() { Expect(fallback.Ready()).To(BeTrue()) })
			})
		})
	})
})

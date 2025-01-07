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

package event_test

import (
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"k8s.io/utils/trace"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
	"github.com/liqotech/liqo/pkg/consts"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/event"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
)

var _ = Describe("Event Reflection Tests", func() {
	Describe("the NewEventReflector function", func() {
		It("should not return a nil reflector", func() {
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 1,
				Type:       root.DefaultReflectorsTypes[resources.Event],
			}
			Expect(event.NewEventReflector(&reflectorConfig)).ToNot(BeNil())
		})
	})

	Describe("event handling", func() {
		const EventName = "name"

		var (
			reflector      manager.NamespacedReflector
			reflectionType offloadingv1beta1.ReflectionType

			local, remote                             corev1.Event
			involvedObjectLocal, involvedObjectRemote ctrclient.Object
			err                                       error
		)

		GetEvent := func(namespace string) *corev1.Event {
			event, errevent := client.CoreV1().Events(namespace).Get(ctx, EventName, metav1.GetOptions{})
			Expect(errevent).ToNot(HaveOccurred())
			return event
		}

		CreateEvent := func(event *corev1.Event) *corev1.Event {
			event, errevent := client.CoreV1().Events(event.GetNamespace()).Create(ctx, event, metav1.CreateOptions{})
			Expect(errevent).ToNot(HaveOccurred())
			return event
		}

		CreatePod := func(name, namespace string) *corev1.Pod {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "test",
						},
					},
				},
			}

			pod, errpod := client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
			Expect(errpod).To(Or(BeNil(), WithTransform(kerrors.IsAlreadyExists, BeTrue())))
			return pod
		}

		DeletePod := func(name, namespace string) {
			Expect(client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
			})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
		}

		ForgeEvent := func(event *corev1.Event, createInvolvedObject bool) *corev1.Event {
			resourceName := "test"
			if createInvolvedObject {
				involvedObjectLocal = CreatePod(resourceName, LocalNamespace)
				involvedObjectRemote = CreatePod(resourceName, RemoteNamespace)
			} else {
				DeletePod(resourceName, LocalNamespace)
				DeletePod(resourceName, RemoteNamespace)
				involvedObjectLocal = nil
				involvedObjectRemote = nil
				time.Sleep(1 * time.Second)
			}

			event.InvolvedObject = corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  event.GetNamespace(),
				Name:       resourceName,
			}
			return event
		}

		WhenBodyLocalShouldNotExist := func(createLocal, createInvolvedObject bool) func() {
			return func() {
				BeforeEach(func() {
					if createLocal {
						local.SetLabels(forge.ReflectionLabels())
						ForgeEvent(&local, createInvolvedObject)
						CreateEvent(&local)
					}
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the local object should not be present", func() {
					_, err = client.CoreV1().Events(LocalNamespace).Get(ctx, EventName, metav1.GetOptions{})
					Expect(err).To(BeNotFound())
				})
			}
		}

		BeforeEach(func() {
			local = corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: EventName, Namespace: LocalNamespace}}
			remote = corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: EventName, Namespace: RemoteNamespace}}
			reflectionType = root.DefaultReflectorsTypes[resources.Event]
		})

		AfterEach(func() {
			Expect(client.CoreV1().Events(LocalNamespace).Delete(ctx, EventName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.CoreV1().Events(RemoteNamespace).Delete(ctx, EventName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))

			if involvedObjectLocal != nil && !reflect.ValueOf(involvedObjectLocal).IsNil() && involvedObjectLocal.GetName() != "" {
				Expect(client.CoreV1().Pods(LocalNamespace).Delete(ctx, involvedObjectLocal.GetName(), metav1.DeleteOptions{
					GracePeriodSeconds: pointer.Int64(0),
				})).To(
					Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			}
			if involvedObjectRemote != nil && !reflect.ValueOf(involvedObjectRemote).IsNil() && involvedObjectRemote.GetName() != "" {
				Expect(client.CoreV1().Pods(RemoteNamespace).Delete(ctx, involvedObjectRemote.GetName(), metav1.DeleteOptions{
					GracePeriodSeconds: pointer.Int64(0),
				})).To(
					Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			}
		})

		JustBeforeEach(func() {
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			reflector = event.NewNamespacedEventReflector(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).
				WithRemote(RemoteNamespace, client, factory).
				WithHandlerFactory(FakeEventHandler).
				WithEventBroadcaster(record.NewBroadcaster()).
				WithReflectionType(reflectionType).
				WithForgingOpts(FakeForgingOpts()))

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())

			err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("Event")), EventName)
		})

		When("the remote object does not exist", func() {
			When("the local object does not exist", WhenBodyLocalShouldNotExist(false, false))
			When("the local object does exist", WhenBodyLocalShouldNotExist(true, false))
		})

		When("the remote object does exist", func() {
			BeforeEach(func() {
				remote.SetLabels(map[string]string{"foo": "bar", FakeNotReflectedLabelKey: "true"})
				remote.SetAnnotations(map[string]string{"bar": "baz", FakeNotReflectedAnnotKey: "true"})
				ForgeEvent(&remote, true)
				CreateEvent(&remote)
			})

			When("the local object does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the metadata should have been correctly replicated to the local object", func() {
					localAfter := GetEvent(LocalNamespace)
					Expect(localAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(localAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(localAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(localAfter.Labels).ToNot(HaveKey(FakeNotReflectedLabelKey))
					Expect(localAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					Expect(localAfter.Annotations).ToNot(HaveKey(FakeNotReflectedAnnotKey))
				})
				It("the spec should have been correctly replicated to the local object", func() {
					localAfter := GetEvent(LocalNamespace)
					Expect(localAfter.InvolvedObject.Name).To(Equal(remote.InvolvedObject.Name))
					Expect(localAfter.InvolvedObject.Kind).To(Equal(remote.InvolvedObject.Kind))
				})
			})

			When("the local object already exists", func() {
				BeforeEach(func() {
					local.SetLabels(forge.ReflectionLabels())
					local.SetAnnotations(map[string]string{"bar": "previous", "existing": "existing"})
					ForgeEvent(&local, true)
					CreateEvent(&local)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			})

			When("the local object already exists, but is not managed by the reflection", func() {
				var localBefore *corev1.Event

				BeforeEach(func() {
					ForgeEvent(&local, true)
					localBefore = CreateEvent(&local)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the local object should be unmodified", func() {
					localAfter := GetEvent(LocalNamespace)
					Expect(localAfter).To(Equal(localBefore))
				})
			})
		})

		When("the remote object does exist, but has the skip annotation", func() {
			BeforeEach(func() {
				remote.SetAnnotations(map[string]string{consts.SkipReflectionAnnotationKey: "whatever"})
				ForgeEvent(&remote, true)
				CreateEvent(&remote)
			})

			When("the local object does not exist", WhenBodyLocalShouldNotExist(false, true))
			When("the local object does exist", WhenBodyLocalShouldNotExist(true, true))
		})

		When("the reflection type is AllowList", func() {
			BeforeEach(func() {
				reflectionType = offloadingv1beta1.AllowList
			})

			When("the remote object does exist, but does not have the allow annotation", func() {
				BeforeEach(func() {
					ForgeEvent(&remote, true)
					CreateEvent(&remote)
				})

				When("the local object does not exist", WhenBodyLocalShouldNotExist(false, true))
				When("the local object does exist", WhenBodyLocalShouldNotExist(true, true))
			})

			When("the remote object does exist, and does have the allow annotation", func() {
				BeforeEach(func() {
					remote.SetAnnotations(map[string]string{consts.AllowReflectionAnnotationKey: "whatever"})
					ForgeEvent(&remote, true)
					CreateEvent(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the local object should be present", func() {
					localAfter := GetEvent(LocalNamespace)
					Expect(localAfter).ToNot(BeNil())
				})
			})
		})

	})
})

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

package exposition_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/trace"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
	"github.com/liqotech/liqo/pkg/consts"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/exposition"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
)

var _ = Describe("Service Reflection Tests", func() {
	Describe("the NewServiceReflector function", func() {
		It("should not return a nil reflector", func() {
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 1,
				Type:       root.DefaultReflectorsTypes[resources.Service],
			}
			Expect(exposition.NewServiceReflector(&reflectorConfig, false, "")).ToNot(BeNil())
		})
	})

	Describe("service handling", func() {
		const ServiceName = "name"

		var (
			reflector      manager.NamespacedReflector
			reflectionType offloadingv1beta1.ReflectionType

			local, remote corev1.Service
			err           error
		)

		GetService := func(namespace string) *corev1.Service {
			svc, errsvc := client.CoreV1().Services(namespace).Get(ctx, ServiceName, metav1.GetOptions{})
			Expect(errsvc).ToNot(HaveOccurred())
			return svc
		}

		CreateService := func(svc *corev1.Service) *corev1.Service {
			svc, errsvc := client.CoreV1().Services(svc.GetNamespace()).Create(ctx, svc, metav1.CreateOptions{})
			Expect(errsvc).ToNot(HaveOccurred())
			return svc
		}

		WhenBodyRemoteShouldNotExist := func(createRemote bool) func() {
			return func() {
				BeforeEach(func() {
					if createRemote {
						remote.SetLabels(forge.ReflectionLabels())
						remote.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}
						CreateService(&remote)
					}
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should not be present", func() {
					_, err = client.CoreV1().Services(RemoteNamespace).Get(ctx, ServiceName, metav1.GetOptions{})
					Expect(err).To(BeNotFound())
				})
			}
		}

		BeforeEach(func() {
			local = corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: LocalNamespace}}
			remote = corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: RemoteNamespace}}
			reflectionType = root.DefaultReflectorsTypes[resources.Service]
		})

		AfterEach(func() {
			Expect(client.CoreV1().Services(LocalNamespace).Delete(ctx, ServiceName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.CoreV1().Services(RemoteNamespace).Delete(ctx, ServiceName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
		})

		JustBeforeEach(func() {
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			reflector = exposition.NewNamespacedServiceReflector(false, "")(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).
				WithRemote(RemoteNamespace, client, factory).
				WithHandlerFactory(FakeEventHandler).
				WithEventBroadcaster(record.NewBroadcaster()).
				WithReflectionType(reflectionType).
				WithForgingOpts(FakeForgingOpts()))

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())

			err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("Service")), ServiceName)
		})

		When("the local object does not exist", func() {
			When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
			When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
		})

		When("the local object does exist", func() {
			BeforeEach(func() {
				local.SetLabels(map[string]string{"foo": "bar", FakeNotReflectedLabelKey: "true"})
				local.SetAnnotations(map[string]string{"bar": "baz", FakeNotReflectedAnnotKey: "true"})
				local.Spec = corev1.ServiceSpec{
					Type:  corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}},
				}
				CreateService(&local)
			})

			When("the remote object does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the metadata should have been correctly replicated to the remote object", func() {
					remoteAfter := GetService(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(remoteAfter.Labels).ToNot(HaveKey(FakeNotReflectedLabelKey))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					Expect(remoteAfter.Annotations).ToNot(HaveKey(FakeNotReflectedAnnotKey))
				})
				It("the spec should have been correctly replicated to the remote object", func() {
					remoteAfter := GetService(RemoteNamespace)
					// Here, we assert only a single field, as already tested in the forge package.
					Expect(remoteAfter.Spec.Type).To(Equal(local.Spec.Type))
				})
			})

			When("the remote object already exists", func() {
				BeforeEach(func() {
					remote.SetLabels(labels.Merge(forge.ReflectionLabels(), map[string]string{FakeNotReflectedLabelKey: "true"}))
					remote.SetAnnotations(map[string]string{"bar": "previous", "existing": "existing", FakeNotReflectedAnnotKey: "true"})
					remote.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}
					CreateService(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the metadata should have been correctly replicated to the remote object", func() {
					remoteAfter := GetService(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(remoteAfter.Labels).To(HaveKey(FakeNotReflectedLabelKey))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("existing", "existing"))
					Expect(remoteAfter.Annotations).To(HaveKey(FakeNotReflectedAnnotKey))
				})
				It("the spec should have been correctly replicated to the remote object", func() {
					remoteAfter := GetService(RemoteNamespace)
					// Here, we assert only a single field, as already tested in the forge package.
					Expect(remoteAfter.Spec.Type).To(Equal(local.Spec.Type))
				})
			})

			When("the remote object already exists, but is not managed by the reflection", func() {
				var remoteBefore *corev1.Service

				BeforeEach(func() {
					remote.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}
					remoteBefore = CreateService(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should be unmodified", func() {
					remoteAfter := GetService(RemoteNamespace)
					Expect(remoteAfter).To(Equal(remoteBefore))
				})
			})
		})

		When("the local object does exist, but has the skip annotation", func() {
			BeforeEach(func() {
				local.SetAnnotations(map[string]string{consts.SkipReflectionAnnotationKey: "whatever"})
				local.Spec = corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}}
				CreateService(&local)
			})

			When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
			When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
		})

		When("the reflection type is AllowList", func() {
			BeforeEach(func() {
				reflectionType = offloadingv1beta1.AllowList
			})

			When("the local object does exist, but does not have the allow annotation", func() {
				BeforeEach(func() {
					local.Spec = corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}}
					CreateService(&local)
				})

				When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
				When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
			})

			When("the local object does exist, and does have the allow annotation", func() {
				BeforeEach(func() {
					local.SetAnnotations(map[string]string{consts.AllowReflectionAnnotationKey: "whatever"})
					local.Spec = corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}}
					CreateService(&local)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should be present", func() {
					remoteAfter := GetService(RemoteNamespace)
					Expect(remoteAfter).ToNot(BeNil())
				})
			})
		})
	})
})

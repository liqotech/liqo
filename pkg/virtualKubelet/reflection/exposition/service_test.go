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

package exposition_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	"k8s.io/utils/trace"

	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/exposition"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("Service Reflection Tests", func() {
	Describe("the NewServiceReflector function", func() {
		It("should not return a nil reflector", func() {
			Expect(exposition.NewServiceReflector(1)).ToNot(BeNil())
		})
	})

	Describe("service handling", func() {
		const ServiceName = "name"

		var (
			reflector manager.NamespacedReflector

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

		BeforeEach(func() {
			local = corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: LocalNamespace}}
			remote = corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: RemoteNamespace}}
		})

		AfterEach(func() {
			Expect(client.CoreV1().Services(LocalNamespace).Delete(ctx, ServiceName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.CoreV1().Services(RemoteNamespace).Delete(ctx, ServiceName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
		})

		JustBeforeEach(func() {
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			reflector = exposition.NewNamespacedServiceReflector(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).
				WithRemote(RemoteNamespace, client, factory).
				WithHandlerFactory(FakeEventHandler))

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())

			err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("Service")), ServiceName)
		})

		When("the local object does not exist", func() {
			WhenBody := func(createRemote bool) func() {
				return func() {
					BeforeEach(func() {
						if createRemote {
							remote.SetLabels(forge.ReflectionLabels())
							remote.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}
							CreateService(&remote)
						}
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the remote object should not be created", func() {
						_, err = client.CoreV1().Services(RemoteNamespace).Get(ctx, ServiceName, metav1.GetOptions{})
						Expect(err).To(BeNotFound())
					})
				}
			}

			When("the remote object does not exist", WhenBody(false))
			When("the remote object does exist", WhenBody(true))
		})

		When("the local object does exist", func() {
			BeforeEach(func() {
				local.SetLabels(map[string]string{"foo": "bar"})
				local.SetAnnotations(map[string]string{"bar": "baz"})
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
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
				})
				It("the spec should have been correctly replicated to the remote object", func() {
					remoteAfter := GetService(RemoteNamespace)
					// Here, we assert only a single field, as already tested in the forge package.
					Expect(remoteAfter.Spec.Type).To(Equal(local.Spec.Type))
				})
			})

			When("the remote object already exists", func() {
				BeforeEach(func() {
					remote.SetLabels(forge.ReflectionLabels())
					remote.SetAnnotations(map[string]string{"bar": "previous", "existing": "existing"})
					remote.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}
					CreateService(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the metadata should have been correctly replicated to the remote object", func() {
					remoteAfter := GetService(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("existing", "existing"))
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
	})
})

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
	netv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

var _ = Describe("Ingress Reflection Tests", func() {
	Describe("the NewIngressReflector function", func() {
		It("should not return a nil reflector", func() {
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 1,
				Type:       root.DefaultReflectorsTypes[resources.Ingress],
			}
			Expect(exposition.NewIngressReflector(&reflectorConfig, false, "")).ToNot(BeNil())
		})
	})

	Describe("ingress handling", func() {
		const IngressName = "name"

		var (
			reflector      manager.NamespacedReflector
			reflectionType offloadingv1beta1.ReflectionType

			local, remote netv1.Ingress
			err           error
		)

		GetIngress := func(namespace string) *netv1.Ingress {
			ing, erring := client.NetworkingV1().Ingresses(namespace).Get(ctx, IngressName, metav1.GetOptions{})
			Expect(erring).ToNot(HaveOccurred())
			return ing
		}

		CreateIngress := func(ing *netv1.Ingress) *netv1.Ingress {
			ing, erring := client.NetworkingV1().Ingresses(ing.GetNamespace()).Create(ctx, ing, metav1.CreateOptions{})
			Expect(erring).ToNot(HaveOccurred())
			return ing
		}

		ForgeIngressSpec := func(ing *netv1.Ingress) *netv1.Ingress {
			ing.Spec.DefaultBackend = &netv1.IngressBackend{
				Service: &netv1.IngressServiceBackend{
					Name: "default-backend",
					Port: netv1.ServiceBackendPort{
						Number: 80,
					},
				},
			}

			return ing
		}

		WhenBodyRemoteShouldNotExist := func(createRemote bool) func() {
			return func() {
				BeforeEach(func() {
					if createRemote {
						remote.SetLabels(forge.ReflectionLabels())
						ForgeIngressSpec(&remote)
						CreateIngress(&remote)
					}
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should not be present", func() {
					_, err = client.NetworkingV1().Ingresses(RemoteNamespace).Get(ctx, IngressName, metav1.GetOptions{})
					Expect(err).To(BeNotFound())
				})
			}
		}

		BeforeEach(func() {
			local = netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: IngressName, Namespace: LocalNamespace}}
			remote = netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: IngressName, Namespace: RemoteNamespace}}
			reflectionType = root.DefaultReflectorsTypes[resources.Ingress]
		})

		AfterEach(func() {
			Expect(client.NetworkingV1().Ingresses(LocalNamespace).Delete(ctx, IngressName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.NetworkingV1().Ingresses(RemoteNamespace).Delete(ctx, IngressName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
		})

		JustBeforeEach(func() {
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			reflector = exposition.NewNamespacedIngressReflector(false, "")(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).
				WithRemote(RemoteNamespace, client, factory).
				WithHandlerFactory(FakeEventHandler).
				WithEventBroadcaster(record.NewBroadcaster()).
				WithReflectionType(reflectionType).
				WithForgingOpts(FakeForgingOpts()))

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())

			err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("Ingress")), IngressName)
		})

		When("the local object does not exist", func() {
			When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
			When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
		})

		When("the local object does exist", func() {
			BeforeEach(func() {
				local.SetLabels(map[string]string{"foo": "bar", FakeNotReflectedLabelKey: "true"})
				local.SetAnnotations(map[string]string{"bar": "baz", FakeNotReflectedAnnotKey: "true"})
				ForgeIngressSpec(&local)
				CreateIngress(&local)
			})

			When("the remote object does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the metadata should have been correctly replicated to the remote object", func() {
					remoteAfter := GetIngress(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(remoteAfter.Labels).ToNot(HaveKey(FakeNotReflectedLabelKey))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					Expect(remoteAfter.Annotations).ToNot(HaveKey(FakeNotReflectedAnnotKey))
				})
				It("the spec should have been correctly replicated to the remote object", func() {
					remoteAfter := GetIngress(RemoteNamespace)
					// Here, we assert only a single field, as already tested in the forge package.
					Expect(remoteAfter.Spec.DefaultBackend).ToNot(BeNil())
				})
			})

			When("the remote object already exists", func() {
				BeforeEach(func() {
					remote.SetLabels(labels.Merge(forge.ReflectionLabels(), map[string]string{FakeNotReflectedLabelKey: "true"}))
					remote.SetAnnotations(map[string]string{"bar": "previous", "existing": "existing", FakeNotReflectedAnnotKey: "true"})
					ForgeIngressSpec(&remote)
					CreateIngress(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the metadata should have been correctly replicated to the remote object", func() {
					remoteAfter := GetIngress(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(remoteAfter.Labels).To(HaveKey(FakeNotReflectedLabelKey))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("existing", "existing"))
					Expect(remoteAfter.Annotations).To(HaveKey(FakeNotReflectedAnnotKey))
				})
				It("the spec should have been correctly replicated to the remote object", func() {
					remoteAfter := GetIngress(RemoteNamespace)
					// Here, we assert only a single field, as already tested in the forge package.
					Expect(remoteAfter.Spec.DefaultBackend).ToNot(BeNil())
				})
			})

			When("the remote object already exists, but is not managed by the reflection", func() {
				var remoteBefore *netv1.Ingress

				BeforeEach(func() {
					ForgeIngressSpec(&remote)
					remoteBefore = CreateIngress(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should be unmodified", func() {
					remoteAfter := GetIngress(RemoteNamespace)
					Expect(remoteAfter).To(Equal(remoteBefore))
				})
			})
		})

		When("the local object does exist, but has the skip annotation", func() {
			BeforeEach(func() {
				local.SetAnnotations(map[string]string{consts.SkipReflectionAnnotationKey: "whatever"})
				ForgeIngressSpec(&local)
				CreateIngress(&local)
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
					ForgeIngressSpec(&local)
					CreateIngress(&local)
				})

				When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
				When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
			})

			When("the local object does exist, and does have the allow annotation", func() {
				BeforeEach(func() {
					local.SetAnnotations(map[string]string{consts.AllowReflectionAnnotationKey: "whatever"})
					ForgeIngressSpec(&local)
					CreateIngress(&local)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should be present", func() {
					remoteAfter := GetIngress(RemoteNamespace)
					Expect(remoteAfter).ToNot(BeNil())
				})
			})
		})
	})
})

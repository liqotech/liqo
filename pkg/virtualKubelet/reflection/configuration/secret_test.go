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

package configuration_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/utils/trace"

	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/configuration"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("Secret Reflection", func() {
	Describe("NewSecretReflector", func() {
		It("should create a non-nil reflector", func() {
			Expect(configuration.NewSecretReflector(1)).NotTo(BeNil())
		})
	})

	Describe("Handle", func() {
		const SecretName = "name"

		var (
			reflector manager.NamespacedReflector

			name          string
			local, remote corev1.Secret
			err           error
		)

		GetSecret := func(namespace string) *corev1.Secret {
			cfg, errcfg := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
			Expect(errcfg).ToNot(HaveOccurred())
			return cfg
		}

		CreateSecret := func(cfg *corev1.Secret) *corev1.Secret {
			createdCfg, errCfg := client.CoreV1().Secrets(cfg.GetNamespace()).Create(ctx, cfg, metav1.CreateOptions{})
			Expect(errCfg).ToNot(HaveOccurred())
			return createdCfg
		}

		BeforeEach(func() {
			name = SecretName
			local = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: LocalNamespace}}
			remote = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: RemoteNamespace}}
		})

		AfterEach(func() {
			Expect(client.CoreV1().Secrets(LocalNamespace).Delete(ctx, name, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.CoreV1().Secrets(RemoteNamespace).Delete(ctx, name, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
		})

		JustBeforeEach(func() {
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			reflector = configuration.NewNamespacedSecretReflector(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).
				WithRemote(RemoteNamespace, client, factory).
				WithHandlerFactory(FakeEventHandler))

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())

			err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("Secret")), name)
		})

		When("the local object does not exist", func() {
			WhenBody := func(createRemote bool) func() {
				return func() {
					BeforeEach(func() {
						if createRemote {
							remote.SetLabels(forge.ReflectionLabels())
							CreateSecret(&remote)
						}
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the remote object should not be created", func() {
						_, err = client.CoreV1().Secrets(RemoteNamespace).Get(ctx, name, metav1.GetOptions{})
						Expect(err).To(BeNotFound())
					})
				}
			}

			When("the remote object does not exist", WhenBody(false))
			When("the remote object does exist", WhenBody(true))
		})

		When("the local object does exists", func() {
			BeforeEach(func() {
				local.SetLabels(map[string]string{"foo": "bar"})
				local.SetAnnotations(map[string]string{"bar": "baz"})
				local.Data = map[string][]byte{"data-key": []byte("some secret data")}
				CreateSecret(&local)
			})

			When("the remote object does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })

				It("the metadata should have been correctly replicated to the remote object", func() {
					remoteAfter := GetSecret(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
				})

				It("the spec should have been correctly replicated to the remote object", func() {
					remoteAfter := GetSecret(RemoteNamespace)
					// Here, we assert only a single field, as already tested in the forge package.
					Expect(remoteAfter.Data).To(HaveKeyWithValue("data-key", []byte("some secret data")))
				})
			})

			When("the remote object already exists", func() {
				BeforeEach(func() {
					remote.SetLabels(forge.ReflectionLabels())
					remote.SetAnnotations(map[string]string{"bar": "previous", "existing": "existing"})
					remote.Data = map[string][]byte{"data-key": []byte("some secret data")}
					CreateSecret(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })

				It("the metadata should have been correctly replicated to the remote object", func() {
					remoteAfter := GetSecret(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("existing", "existing"))
				})

				It("the spec should have been correctly replicated to the remote object", func() {
					remoteAfter := GetSecret(RemoteNamespace)
					// Here, we assert only a single field, as already tested in the forge package.
					Expect(remoteAfter.Data).To(HaveKeyWithValue("data-key", []byte("some secret data")))
				})
			})

			When("the remote object already exists, but is not managed by the reflection", func() {
				var remoteBefore *corev1.Secret

				BeforeEach(func() {
					remoteBefore = CreateSecret(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should be unmodified", func() {
					remoteAfter := GetSecret(RemoteNamespace)
					Expect(remoteAfter).To(Equal(remoteBefore))
				})
			})
		})

		When("handling secrets of type kubernetes.io/service-account-token", func() {
			BeforeEach(func() {
				local.SetAnnotations(map[string]string{corev1.ServiceAccountNameKey: "default"})
				local.Type = corev1.SecretTypeServiceAccountToken
				CreateSecret(&local)
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("the remote object should not be created", func() {
				_, err = client.CoreV1().Secrets(RemoteNamespace).Get(ctx, name, metav1.GetOptions{})
				Expect(err).To(BeNotFound())
			})
		})
	})
})

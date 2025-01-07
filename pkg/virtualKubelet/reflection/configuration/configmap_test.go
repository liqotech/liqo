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

package configuration_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/configuration"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
)

var _ = Describe("ConfigMap Reflection", func() {
	Describe("NewConfigMapReflector", func() {
		It("should create a non-nil reflector", func() {
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 1,
				Type:       root.DefaultReflectorsTypes[resources.ConfigMap],
			}
			Expect(configuration.NewConfigMapReflector(&reflectorConfig)).NotTo(BeNil())
		})
	})

	Describe("Handle", func() {
		const ConfigMapName = "name"

		var (
			reflector      manager.NamespacedReflector
			reflectionType offloadingv1beta1.ReflectionType

			name          string
			local, remote corev1.ConfigMap
			err           error
		)

		GetConfigMap := func(namespace string) *corev1.ConfigMap {
			cfg, errcfg := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
			Expect(errcfg).ToNot(HaveOccurred())
			return cfg
		}

		CreateConfigMap := func(cfg *corev1.ConfigMap) *corev1.ConfigMap {
			createdCfg, errCfg := client.CoreV1().ConfigMaps(cfg.GetNamespace()).Create(ctx, cfg, metav1.CreateOptions{})
			Expect(errCfg).ToNot(HaveOccurred())
			return createdCfg
		}

		WhenBodyRemoteShouldNotExist := func(createRemote bool) func() {
			return func() {
				BeforeEach(func() {
					if createRemote {
						remote.SetLabels(forge.ReflectionLabels())
						CreateConfigMap(&remote)
					}
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should not be present", func() {
					_, err = client.CoreV1().ConfigMaps(RemoteNamespace).Get(ctx, name, metav1.GetOptions{})
					Expect(err).To(BeNotFound())
				})
			}
		}

		BeforeEach(func() {
			name = ConfigMapName
			local = corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: LocalNamespace}}
			remote = corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: RemoteNamespace}}
			reflectionType = root.DefaultReflectorsTypes[resources.ConfigMap]
		})

		AfterEach(func() {
			Expect(client.CoreV1().ConfigMaps(LocalNamespace).Delete(ctx, name, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.CoreV1().ConfigMaps(RemoteNamespace).Delete(ctx, name, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
		})

		JustBeforeEach(func() {
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			reflector = configuration.NewNamespacedConfigMapReflector(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).
				WithRemote(RemoteNamespace, client, factory).
				WithHandlerFactory(FakeEventHandler).
				WithEventBroadcaster(record.NewBroadcaster()).
				WithReflectionType(reflectionType).
				WithForgingOpts(FakeForgingOpts()))

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())

			err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("ConfigMap")), name)
		})

		When("the local object does not exist", func() {
			When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
			When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
		})

		When("the local object does exists", func() {
			BeforeEach(func() {
				local.SetLabels(map[string]string{"foo": "bar", FakeNotReflectedLabelKey: "true"})
				local.SetAnnotations(map[string]string{"bar": "baz", FakeNotReflectedAnnotKey: "true"})
				local.Data = map[string]string{"data-key": "some config data"}
				CreateConfigMap(&local)
			})

			When("the remote object does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })

				It("the metadata should have been correctly replicated to the remote object", func() {
					remoteAfter := GetConfigMap(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(remoteAfter.Labels).ToNot(HaveKey(FakeNotReflectedLabelKey))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					Expect(remoteAfter.Annotations).ToNot(HaveKey(FakeNotReflectedAnnotKey))
				})

				It("the spec should have been correctly replicated to the remote object", func() {
					remoteAfter := GetConfigMap(RemoteNamespace)
					// Here, we assert only a single field, as already tested in the forge package.
					Expect(remoteAfter.Data).To(HaveKeyWithValue("data-key", "some config data"))
				})
			})

			When("the remote object already exists", func() {
				BeforeEach(func() {
					remote.SetLabels(labels.Merge(forge.ReflectionLabels(), map[string]string{FakeNotReflectedLabelKey: "true"}))
					remote.SetAnnotations(map[string]string{"bar": "previous", "existing": "existing", FakeNotReflectedAnnotKey: "true"})
					remote.Data = map[string]string{"data-key": "some remote config data"}
					CreateConfigMap(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })

				It("the metadata should have been correctly replicated to the remote object", func() {
					remoteAfter := GetConfigMap(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
					Expect(remoteAfter.Labels).To(HaveKey(FakeNotReflectedLabelKey))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue("existing", "existing"))
					Expect(remoteAfter.Annotations).To(HaveKey(FakeNotReflectedAnnotKey))
				})

				It("the spec should have been correctly replicated to the remote object", func() {
					remoteAfter := GetConfigMap(RemoteNamespace)
					// Here, we assert only a single field, as already tested in the forge package.
					Expect(remoteAfter.Data).To(HaveKeyWithValue("data-key", "some config data"))
				})
			})

			When("the remote object already exists, but is not managed by the reflection", func() {
				var remoteBefore *corev1.ConfigMap

				BeforeEach(func() {
					remoteBefore = CreateConfigMap(&remote)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should be unmodified", func() {
					remoteAfter := GetConfigMap(RemoteNamespace)
					Expect(remoteAfter).To(Equal(remoteBefore))
				})
			})
		})

		When("the local object does exist, but has the skip annotation", func() {
			BeforeEach(func() {
				local.SetAnnotations(map[string]string{consts.SkipReflectionAnnotationKey: "whatever"})
				CreateConfigMap(&local)
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
					CreateConfigMap(&local)
				})

				When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
				When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
			})

			When("the local object does exist, and does have the allow annotation", func() {
				BeforeEach(func() {
					local.SetAnnotations(map[string]string{consts.AllowReflectionAnnotationKey: "whatever"})
					CreateConfigMap(&local)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should be present", func() {
					remoteAfter := GetConfigMap(RemoteNamespace)
					Expect(remoteAfter).ToNot(BeNil())
				})
			})
		})

		When("handling the root CA configmap", func() {
			BeforeEach(func() {
				name = "kube-root-ca.crt"
				local.SetName(name)
				CreateConfigMap(&local)
			})

			When("the reflection type is DenyList", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remapped remote object should be created", func() {
					_, err = client.CoreV1().ConfigMaps(RemoteNamespace).Get(ctx, forge.RemoteConfigMapName(name), metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
				})
			})

			When("the reflection type is AllowList", func() {
				BeforeEach(func() {
					reflectionType = offloadingv1beta1.AllowList
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remapped remote object should be created", func() {
					_, err = client.CoreV1().ConfigMaps(RemoteNamespace).Get(ctx, forge.RemoteConfigMapName(name), metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
})

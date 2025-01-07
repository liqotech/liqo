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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"k8s.io/utils/trace"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/configuration"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
)

var _ = Describe("ServiceAccount Reflection", func() {
	Describe("NewServiceAccountReflector", func() {
		It("should create a non-nil reflector", func() {
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 1,
				Type:       root.DefaultReflectorsTypes[resources.ServiceAccount],
			}
			Expect(configuration.NewServiceAccountReflector(true, &reflectorConfig)).NotTo(BeNil())
		})
	})

	Describe("Handle", func() {
		const (
			PodName            = "pod-name"
			ServiceAccountName = "sa-name"
		)

		var (
			reflector     manager.NamespacedReflector
			secretsLister corev1listers.SecretNamespaceLister

			secretName string

			local   *corev1.Pod
			remote  *corev1.Secret
			localSA *corev1.ServiceAccount

			err error
		)

		CreatePod := func(pod *corev1.Pod) *corev1.Pod {
			pod.Spec.NodeName = LiqoNodeName
			pod, errpod := client.CoreV1().Pods(pod.GetNamespace()).Create(ctx, pod, metav1.CreateOptions{})
			ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
			return pod
		}

		GetSecret := func(namespace string) *corev1.Secret {
			cfg, errcfg := client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
			ExpectWithOffset(1, errcfg).ToNot(HaveOccurred())
			return cfg
		}

		CreateSecret := func(secret *corev1.Secret) *corev1.Secret {
			createdSecret, errSecret := client.CoreV1().Secrets(secret.GetNamespace()).Create(ctx, secret, metav1.CreateOptions{})
			ExpectWithOffset(1, errSecret).ToNot(HaveOccurred())
			return createdSecret
		}

		CreateServiceAccount := func(sa *corev1.ServiceAccount) *corev1.ServiceAccount {
			createdSA, errSA := client.CoreV1().ServiceAccounts(sa.GetNamespace()).Create(ctx, sa, metav1.CreateOptions{})
			ExpectWithOffset(1, errSA).ToNot(HaveOccurred())
			return createdSA
		}

		WhenBodyRemoteShouldNotExist := func(createRemote bool) func() {
			return func() {
				BeforeEach(func() {
					if createRemote {
						tokens := forge.ServiceAccountPodTokens{PodName: PodName, ServiceAccountName: ServiceAccountName}
						l := labels.Merge(forge.ReflectionLabels(), forge.RemoteServiceAccountSecretLabels(&tokens))
						l = labels.Merge(l, map[string]string{forge.LiqoOriginClusterNodeName: LiqoNodeName})
						remote.SetLabels(l)
						CreateSecret(remote)
					}
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should not be present", func() {
					_, err = client.CoreV1().Secrets(RemoteNamespace).Get(ctx, secretName, metav1.GetOptions{})
					Expect(err).To(BeNotFound())
				})
			}
		}

		BeforeEach(func() {
			secretName = forge.ServiceAccountSecretName(PodName)
			local = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: LocalNamespace}}
			localSA = &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: ServiceAccountName, Namespace: LocalNamespace, UID: "pod-uid"}}
			remote = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: RemoteNamespace}}
		})

		AfterEach(func() {
			Expect(client.CoreV1().Pods(LocalNamespace).Delete(ctx, PodName, metav1.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
			})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.CoreV1().ServiceAccounts(LocalNamespace).Delete(ctx, ServiceAccountName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.CoreV1().Secrets(RemoteNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
		})

		JustBeforeEach(func() {
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			secretsLister = factory.Core().V1().Secrets().Lister().Secrets(RemoteNamespace)
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 0,
				Type:       root.DefaultReflectorsTypes[resources.ServiceAccount],
			}
			rfl := configuration.NewServiceAccountReflector(true, &reflectorConfig).(*configuration.ServiceAccountReflector)
			rfl.Start(ctx, options.New(client, factory.Core().V1().Pods()))
			reflector = rfl.NewNamespaced(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).
				WithRemote(RemoteNamespace, client, factory).
				WithHandlerFactory(FakeEventHandler).
				WithEventBroadcaster(record.NewBroadcaster()))

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())

			err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("ServiceAccount")), PodName)
		})

		When("the local object does not exist", func() {
			When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
			When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
		})

		When("the local object does exists, but refers to no volumes", func() {
			BeforeEach(func() {
				local.Spec.ServiceAccountName = ServiceAccountName
				local.Spec.Containers = []corev1.Container{{Name: "bar", Image: "foo"}}
				local = CreatePod(local)
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should create no remote secrets", func() {
				_, err = client.CoreV1().Secrets(RemoteNamespace).Get(ctx, secretName, metav1.GetOptions{})
				Expect(err).To(BeNotFound())
			})
		})

		When("the local object does exists", func() {
			BeforeEach(func() {
				local.Spec.ServiceAccountName = ServiceAccountName
				local.Spec.Containers = []corev1.Container{{Name: "bar", Image: "foo"}}
				local.Spec.Volumes = []corev1.Volume{{Name: "token", VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Path: "path", ExpirationSeconds: pointer.Int64(7200)},
					}, {
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Path: "other", Audience: "audience", ExpirationSeconds: pointer.Int64(3600)},
					}}}}}}
				local = CreatePod(local)
			})

			When("the local service account does not exist", func() {
				It("should fail", func() { Expect(err).To(BeNotFound()) })
			})

			When("the remote object does not exist", func() {
				BeforeEach(func() { CreateServiceAccount(localSA) })

				It("should reenqueue the object to refresh the tokens", func() {
					Expect(err).To(BeAssignableToTypeOf(generic.EnqueueAfter(time.Second)))
				})

				It("the metadata should have been correctly set for the remote object", func() {
					remoteAfter := GetSecret(RemoteNamespace)
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoSASecretForPodNameKey, PodName))
					Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoSASecretForServiceAccountKey, ServiceAccountName))
					Expect(remoteAfter.Annotations).To(HaveKeyWithValue(forge.LiqoSASecretForPodUIDKey, string(local.GetUID())))
					Expect(remoteAfter.Annotations).To(HaveKey(forge.LiqoSASecretExpirationKey))
				})

				It("the spec should have been correctly set for the remote object", func() {
					remoteAfter := GetSecret(RemoteNamespace)
					Expect(remoteAfter.Data).To(HaveLen(2))
					Expect(remoteAfter.Data).To(HaveKey(forge.ServiceAccountTokenKey("token", "path")))
					Expect(remoteAfter.Data).To(HaveKey(forge.ServiceAccountTokenKey("token", "other")))
					Expect(remoteAfter.Immutable).To(PointTo(BeFalse()))
				})

				When("the Handle function is executed again", func() {
					var remoteBefore *corev1.Secret

					JustBeforeEach(func() {
						// Delete the service account, to prevent token refresh (the logic shall keep the cached one)
						Expect(client.CoreV1().ServiceAccounts(LocalNamespace).
							Delete(ctx, ServiceAccountName, metav1.DeleteOptions{})).ToNot(HaveOccurred())

						// Delete the secret, to verify whether it is recreated correctly
						remoteBefore = GetSecret(RemoteNamespace)
						Expect(client.CoreV1().Secrets(RemoteNamespace).
							Delete(ctx, secretName, metav1.DeleteOptions{})).ToNot(HaveOccurred())

						// Make sure that the secret deletion gets propagated to the informer cache
						Eventually(func() error {
							_, errSecret := secretsLister.Get(secretName)
							return errSecret
						}).Should(BeNotFound())

						err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("ServiceAccount")), PodName)
					})

					It("should not renew the tokens, since they did not yet expire", func() {
						// Checking whether no error (but reenqueue) is returned, since given the service account has been
						// deleted it would be impossible to properly refresh the tokens, returning the corresponding error.
						Expect(err).To(BeAssignableToTypeOf(generic.EnqueueAfter(time.Second)))
					})

					It("should recreate an identical secret", func() {
						remoteAfter := GetSecret(RemoteNamespace)
						Expect(remoteAfter.Labels).To(Equal(remoteBefore.Labels))
						Expect(remoteAfter.Annotations).To(Equal(remoteBefore.Annotations))
						Expect(remoteAfter.Data).To(Equal(remoteBefore.Data))
						Expect(remoteAfter.Type).To(Equal(remoteBefore.Type))
					})
				})
			})

			When("the remote object already exists", func() {
				BeforeEachBody := func(uid types.UID, expiration time.Time) {
					tokens := forge.ServiceAccountPodTokens{
						PodName: PodName, ServiceAccountName: ServiceAccountName, PodUID: uid,
						Tokens: []*forge.ServiceAccountPodToken{{ActualExpiration: expiration}},
					}
					l := labels.Merge(forge.ReflectionLabels(), forge.RemoteServiceAccountSecretLabels(&tokens))
					l = labels.Merge(l, map[string]string{forge.LiqoOriginClusterNodeName: LiqoNodeName})
					remote.SetLabels(l)
					remote.SetAnnotations(forge.RemoteServiceAccountSecretAnnotations(&tokens))
					remote.Data = map[string][]byte{forge.ServiceAccountTokenKey("token", "path"): []byte("tkn")}
					CreateSecret(remote)
				}

				BeforeEach(func() { CreateServiceAccount(localSA) })

				Context("the cached UID matches the one of the pod, and the tokens have not yet expired", func() {
					BeforeEach(func() { BeforeEachBody(local.GetUID(), time.Now().Add(1*time.Hour)) })

					It("should reenqueue the object to refresh the tokens", func() {
						Expect(err).To(BeAssignableToTypeOf(generic.EnqueueAfter(time.Second)))
					})

					It("the metadata should not have been modified", func() {
						remoteAfter := GetSecret(RemoteNamespace)
						Expect(remoteAfter.Labels).To(Equal(remote.Labels))
						Expect(remoteAfter.Annotations).To(Equal(remote.Annotations))
					})

					It("only non-existing tokens should have been updated", func() {
						remoteAfter := GetSecret(RemoteNamespace)
						Expect(remoteAfter.Data).To(HaveLen(2))
						Expect(remoteAfter.Data).To(HaveKeyWithValue(forge.ServiceAccountTokenKey("token", "path"), []byte("tkn")))
						Expect(remoteAfter.Data).To(HaveKey(forge.ServiceAccountTokenKey("token", "other")))
					})
				})

				Context("the cached UID matches the one of the pod, but the tokens have already expired", func() {
					BeforeEach(func() { BeforeEachBody(local.GetUID(), time.Now().Add(-1*time.Minute)) })

					It("should reenqueue the object to refresh the tokens", func() {
						Expect(err).To(BeAssignableToTypeOf(generic.EnqueueAfter(time.Second)))
					})

					It("the metadata should have been properly updated", func() {
						remoteAfter := GetSecret(RemoteNamespace)
						Expect(remoteAfter.Labels).To(Equal(remote.Labels))
						Expect(remoteAfter.Annotations).To(HaveKeyWithValue(forge.LiqoSASecretForPodUIDKey, string(local.GetUID())))
						Expect(remoteAfter.Annotations).To(HaveKey(forge.LiqoSASecretExpirationKey))
						Expect(remoteAfter.Annotations[forge.LiqoSASecretExpirationKey]).
							ToNot(Equal(remote.Annotations[forge.LiqoSASecretExpirationKey]))
					})

					It("all tokens should have been updated", func() {
						remoteAfter := GetSecret(RemoteNamespace)
						Expect(remoteAfter.Data).To(HaveLen(2))
						Expect(remoteAfter.Data).To(HaveKey(forge.ServiceAccountTokenKey("token", "path")))
						Expect(remoteAfter.Data).To(HaveKey(forge.ServiceAccountTokenKey("token", "other")))
						Expect(remoteAfter.Data[forge.ServiceAccountTokenKey("token", "path")]).ToNot(Equal([]byte("tkn")))
					})
				})

				Context("the cached UID does not match the one of the pod, but the tokens have not yet expired", func() {
					BeforeEach(func() { BeforeEachBody(types.UID("other"), time.Now().Add(1*time.Hour)) })

					It("should return an error to reenqueue the object", func() {
						Expect(err).To(MatchError(fmt.Errorf("mismatching UID detected for pod %q", klog.KObj(local))))
					})

					It("the secret should have been deleted", func() {
						_, err = client.CoreV1().Secrets(RemoteNamespace).Get(ctx, secretName, metav1.GetOptions{})
						Expect(err).To(BeNotFound())
					})
				})
			})

			When("the remote object already exists, but is not managed by the reflection", func() {
				var remoteBefore *corev1.Secret

				BeforeEach(func() { remoteBefore = CreateSecret(remote) })

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should be unmodified", func() {
					remoteAfter := GetSecret(RemoteNamespace)
					Expect(remoteAfter).To(Equal(remoteBefore))
				})
			})
		})
	})

	Describe("Fallback reflection", func() {
		const PodName = "name"

		var (
			fallback manager.FallbackReflector

			local                  corev1.Pod
			fallbackReflectorReady bool
		)

		BeforeEach(func() { local = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: "not-existing"}} })

		JustBeforeEach(func() {
			client := fake.NewSimpleClientset(&local)
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 0,
				Type:       root.DefaultReflectorsTypes[resources.ServiceAccount],
			}
			rfl := configuration.NewServiceAccountReflector(true, &reflectorConfig).(*configuration.ServiceAccountReflector)
			opts := options.New(client, factory.Core().V1().Pods()).
				WithHandlerFactory(FakeEventHandler).
				WithReadinessFunc(func() bool { return fallbackReflectorReady })
			fallback = rfl.NewFallback(opts)

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())
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
				JustBeforeEach(func() { fallbackReflectorReady = true })
				It("should return true", func() { Expect(fallback.Ready()).To(BeTrue()) })
			})
		})
	})
})

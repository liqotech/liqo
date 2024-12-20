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

package workload_test

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"k8s.io/utils/trace"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoclientfake "github.com/liqotech/liqo/pkg/client/clientset/versioned/fake"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	"github.com/liqotech/liqo/pkg/consts"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/workload"
)

var _ = Describe("Namespaced Pod Reflection Tests", func() {

	Describe("pod handling", func() {
		var (
			reflector  manager.NamespacedReflector
			client     *fake.Clientset
			liqoClient liqoclient.Interface
		)

		BeforeEach(func() {
			client = fake.NewSimpleClientset()
			liqoClient = liqoclientfake.NewSimpleClientset()
		})

		JustBeforeEach(func() {
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			liqoFactory := liqoinformers.NewSharedInformerFactory(liqoClient, 10*time.Hour)

			broadcaster := record.NewBroadcaster()
			metricsFactory := func(string) metricsv1beta1.PodMetricsInterface { return nil }
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 0,
				Type:       root.DefaultReflectorsTypes[resources.Pod],
			}
			rfl := workload.NewPodReflector(nil, metricsFactory,
				&workload.PodReflectorConfig{forge.APIServerSupportTokenAPI, false, "", "", fakeAPIServerRemapping(""),
					&networkingv1beta1.Configuration{
						Spec: networkingv1beta1.ConfigurationSpec{
							Remote: networkingv1beta1.ClusterConfig{
								CIDR: networkingv1beta1.ClusterConfigCIDR{
									Pod:      cidrutils.SetPrimary("192.168.200.0/24"),
									External: cidrutils.SetPrimary("192.168.100.0/24"),
								},
							},
						},
						Status: networkingv1beta1.ConfigurationStatus{
							Remote: &networkingv1beta1.ClusterConfig{
								CIDR: networkingv1beta1.ClusterConfigCIDR{
									Pod:      cidrutils.SetPrimary("192.168.201.0/24"),
									External: cidrutils.SetPrimary("192.168.101.0/24"),
								},
							},
						},
					}}, &reflectorConfig)
			rfl.Start(ctx, options.New(client, factory.Core().V1().Pods()).WithEventBroadcaster(broadcaster))
			reflector = rfl.NewNamespaced(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).WithLiqoLocal(liqoClient, liqoFactory).
				WithRemote(RemoteNamespace, client, factory).WithLiqoRemote(liqoClient, liqoFactory).
				WithHandlerFactory(FakeEventHandler).WithEventBroadcaster(broadcaster).WithForgingOpts(FakeForgingOpts()))

			factory.Start(ctx.Done())
			liqoFactory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())
			liqoFactory.WaitForCacheSync(ctx.Done())
		})

		Context("object reflection", func() {
			const PodName = "name"

			var (
				local, remote corev1.Pod
				shadow        offloadingv1beta1.ShadowPod
				err           error
			)

			WhenBodyRemoteNotManagedByReflection := func() func() {
				return func() {
					var shadowBefore *offloadingv1beta1.ShadowPod

					BeforeEach(func() {
						shadowBefore = CreateShadowPod(liqoClient, &shadow)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the remote object should be unmodified", func() {
						shadowAfter := GetShadowPod(liqoClient, RemoteNamespace, PodName)
						Expect(shadowAfter).To(Equal(shadowBefore))
					})
				}
			}

			BeforeEach(func() {
				local = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: LocalNamespace}}
				remote = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: RemoteNamespace}}
				shadow = offloadingv1beta1.ShadowPod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: RemoteNamespace}}
			})

			JustBeforeEach(func() {
				err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("Pod")), PodName)
			})

			When("the local object does not exist", func() {
				WhenBody := func(createRemote bool) func() {
					return func() {
						BeforeEach(func() {
							if createRemote {
								shadow.SetLabels(forge.ReflectionLabels())
								CreateShadowPod(liqoClient, &shadow)
							}
						})

						It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
						It("the remote object should not be created", func() {
							_, err = liqoClient.OffloadingV1beta1().ShadowPods(RemoteNamespace).Get(ctx, PodName, metav1.GetOptions{})
							Expect(GetShadowPodError(liqoClient, RemoteNamespace, PodName)).To(BeNotFound())
						})
					}
				}

				When("the remote object does not exist", WhenBody(false))
				When("the remote object does exist", WhenBody(true))
				When("the remote object does exist, but is not managed by the reflection", WhenBodyRemoteNotManagedByReflection())
			})

			When("the local object does exist and is not terminating", func() {
				var shouldDenyPodPatches bool

				BeforeEach(func() {
					shouldDenyPodPatches = false
					local.SetLabels(map[string]string{"foo": "bar", FakeNotReflectedLabelKey: "true"})
					local.SetAnnotations(map[string]string{"bar": "baz", FakeNotReflectedAnnotKey: "true"})
					local.Spec.Containers = []corev1.Container{{Name: "bar", Image: "foo"}}
					CreatePod(client, &local)

					// Currently, the fake client does not handle SSA, and we need to add a custom reactor for that.
					client.PrependReactor("patch", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
						patch, ok := action.(testing.PatchAction)
						if !ok {
							return true, nil, fmt.Errorf("failed to retrieve patch action details")
						}

						if patch.GetName() != PodName || patch.GetNamespace() != LocalNamespace {
							return true, nil, fmt.Errorf("received patch for unexpected pod %s/%s", patch.GetNamespace(), patch.GetName())
						}

						if patch.GetPatchType() != types.ApplyPatchType {
							return true, nil, fmt.Errorf("unsupported patch type %s", patch.GetPatchType())
						}

						if shouldDenyPodPatches {
							return true, nil, fmt.Errorf("pod patches disabled")
						}

						return true, nil, nil
					})
				})

				When("the remote object does not exist", func() {
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the metadata should have been correctly replicated to the remote object", func() {
						shadowAfter := GetShadowPod(liqoClient, RemoteNamespace, PodName)
						Expect(shadowAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
						Expect(shadowAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
						Expect(shadowAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
						Expect(shadowAfter.Labels).ToNot(HaveKey(FakeNotReflectedLabelKey))
						Expect(shadowAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
						Expect(shadowAfter.Annotations).ToNot(HaveKey(FakeNotReflectedAnnotKey))
					})
					It("the spec should have been correctly replicated to the remote object", func() {
						shadowAfter := GetShadowPod(liqoClient, RemoteNamespace, PodName)
						// Here, we assert only a few fields, as already tested in the forge package.
						Expect(shadowAfter.Spec.Pod.Containers).To(HaveLen(1))
						Expect(shadowAfter.Spec.Pod.Containers[0].Name).To(BeIdenticalTo("bar"))
						Expect(shadowAfter.Spec.Pod.Containers[0].Image).To(BeIdenticalTo("foo"))
					})
				})

				When("the remote object already exists and needs to be updated", func() {
					BeforeEach(func() {
						shadow.SetLabels(labels.Merge(forge.ReflectionLabels(), map[string]string{FakeNotReflectedLabelKey: "true"}))
						shadow.SetAnnotations(map[string]string{"bar": "previous", "existing": "existing", FakeNotReflectedAnnotKey: "true"})
						CreateShadowPod(liqoClient, &shadow)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the metadata should have been correctly replicated to the remote object", func() {
						shadowAfter := GetShadowPod(liqoClient, RemoteNamespace, PodName)
						Expect(shadowAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
						Expect(shadowAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
						Expect(shadowAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
						Expect(shadowAfter.Labels).ToNot(HaveKey(FakeNotReflectedLabelKey))
						Expect(shadowAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
						Expect(shadowAfter.Annotations).NotTo(HaveKeyWithValue("existing", "existing"))
						Expect(shadowAfter.Annotations).ToNot(HaveKey(FakeNotReflectedAnnotKey))
					})
					It("the spec should not have been replicated to the remote object, to prevent possible issues", func() {
						shadowAfter := GetShadowPod(liqoClient, RemoteNamespace, PodName)
						// Here, we assert only a few fields, as already tested in the forge package.
						Expect(shadowAfter.Spec.Pod).To(Equal(shadow.Spec.Pod))
					})
				})

				When("the remote object already exists and is correct", func() {
					BeforeEach(func() {
						shadow.SetLabels(labels.Merge(map[string]string{"foo": "bar"}, forge.ReflectionLabelsWithNodeName(LiqoNodeName)))
						shadow.SetAnnotations(map[string]string{"bar": "baz"})
						shadow.Spec.Pod.Containers = []corev1.Container{{Name: "bar", Image: "foo"}}

						// Here, we create a modified fake client which returns an error when trying to perform an update operation.
						tmp := liqoclientfake.NewSimpleClientset(&shadow)
						tmp.PrependReactor("update", "*", func(action testing.Action) (handled bool, _ runtime.Object, err error) {
							return true, nil, errors.New("should not call update")
						})
						liqoClient = tmp
					})

					// An error is generated if the reflection attempts to update the ShadowPod.
					It("should succeed (i.e., do not update the shadowpod)", func() { Expect(err).ToNot(HaveOccurred()) })

					When("the remote pod also already exists", func() {
						BeforeEach(func() {
							remote.SetLabels(forge.ReflectionLabels())
							remote.Status.Phase = corev1.PodRunning
							CreatePod(client, &remote)
						})

						It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
						It("should update the status of the local pod", func() {
							localAfter := GetPod(client, LocalNamespace, PodName)
							Expect(localAfter.Status.Phase).To(BeIdenticalTo(corev1.PodRunning))
						})
					})
				})

				When("the remote object already exists, but is not managed by the reflection", WhenBodyRemoteNotManagedByReflection())

				When("the local object has already the appropriate offloading label", func() {
					BeforeEach(func() {
						shouldDenyPodPatches = true
						local.Labels[consts.LocalPodLabelKey] = consts.LocalPodLabelValue
						UpdatePod(client, &local)
					})

					// The fake client is configured to return an error in case a patch operation on pods is attempted.
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				})
			})

			When("the local object does exist and it is terminating", func() {
				BeforeEach(func() {
					local.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
					CreatePod(client, &local)
				})

				When("neither the remote shadowpod nor the remote pod are present", func() {
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should delete the local pod", func() {
						Expect(GetPodError(client, LocalNamespace, PodName)).To(BeNotFound())
					})
				})

				When("the remote shadowpod is present and not yet terminating", func() {
					BeforeEach(func() {
						shadow.SetLabels(forge.ReflectionLabels())
						CreateShadowPod(liqoClient, &shadow)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should not delete the local pod", func() {
						Expect(GetPodError(client, LocalNamespace, PodName)).ToNot(HaveOccurred())
					})
					It("should delete the remote shadowpod", func() {
						Expect(GetShadowPodError(liqoClient, RemoteNamespace, PodName)).To(BeNotFound())
					})
				})

				When("the remote shadowpod is present and terminating", func() {
					BeforeEach(func() {
						shadow.SetLabels(forge.ReflectionLabels())
						shadow.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
						CreateShadowPod(liqoClient, &shadow)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should not delete the local pod", func() {
						Expect(GetPodError(client, LocalNamespace, PodName)).ToNot(HaveOccurred())
					})
					It("should not delete the remote shadowpod", func() {
						Expect(GetShadowPodError(liqoClient, RemoteNamespace, PodName)).ToNot(HaveOccurred())
					})
				})

				When("the remote pod is present", func() {
					BeforeEach(func() {
						remote.SetLabels(forge.ReflectionLabels())
						remote.Status.Phase = corev1.PodRunning
						CreatePod(client, &remote)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should not delete the local pod", func() {
						Expect(GetPodError(client, LocalNamespace, PodName)).ToNot(HaveOccurred())
					})
					It("should update the status of the local pod", func() {
						localAfter := GetPod(client, LocalNamespace, PodName)
						Expect(localAfter.Status.Phase).To(BeIdenticalTo(corev1.PodRunning))
					})
				})
			})

			When("the local object does exist and has been rejected (OffloadingAborted)", func() {
				BeforeEach(func() {
					local.Status.Phase = corev1.PodFailed
					local.Status.Reason = forge.PodOffloadingAbortedReason
					CreatePod(client, &local)

					shadow.SetLabels(forge.ReflectionLabels())
					CreateShadowPod(liqoClient, &shadow)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should remove the remote shadow pod (if present)", func() {
					_, err = liqoClient.OffloadingV1beta1().ShadowPods(RemoteNamespace).Get(ctx, PodName, metav1.GetOptions{})
					Expect(GetShadowPodError(liqoClient, RemoteNamespace, PodName)).To(BeNotFound())
				})
			})
		})

		Context("status reflection", func() {
			const PodName = "name"

			var (
				local, remote *corev1.Pod
				podInfo       workload.PodInfo
				err           error
			)

			BeforeEach(func() {
				local = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: LocalNamespace}}
				remote = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: RemoteNamespace, UID: "uuid"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning, PodIP: "192.168.200.25",
						ContainerStatuses: []corev1.ContainerStatus{{RestartCount: 1}},
						Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
					},
				}
				podInfo = workload.PodInfo{}
			})

			JustBeforeEach(func() {
				CreatePod(client, local)
				err = reflector.(*workload.NamespacedPodReflector).HandleStatus(
					trace.ContextWithTrace(ctx, trace.New("Pod")), local, remote, &podInfo)
			})

			When("the local pod has remote unavailable label", func() {
				BeforeEach(func() {
					local.SetLabels(map[string]string{consts.RemoteUnavailableKey: consts.RemoteUnavailableValue})
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should update the Ready condition of the local pod to False", func() {
					localAfter := GetPod(client, LocalNamespace, PodName)
					Expect(localAfter.Status.Conditions[0].Type).To(BeIdenticalTo(corev1.PodReady))
					Expect(localAfter.Status.Conditions[0].Status).To(BeIdenticalTo(corev1.ConditionFalse))
				})
			})

			When("the local pod does not have remote unavailable label", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should keep the Ready condition equal to the one on the remote pod", func() {
					localAfter := GetPod(client, LocalNamespace, PodName)
					Expect(localAfter.Status.Conditions[0].Type).To(BeIdenticalTo(corev1.PodReady))
					Expect(localAfter.Status.Conditions[0].Status).To(BeIdenticalTo(corev1.ConditionTrue))
				})
			})

			When("the local status is not up to date", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should update the status of the local pod", func() {
					// Here, we assert only a few fields, as already tested in the forge package.
					localAfter := GetPod(client, LocalNamespace, PodName)
					Expect(localAfter.Status.Phase).To(BeIdenticalTo(corev1.PodRunning))
					Expect(localAfter.Status.PodIP).To(BeIdenticalTo("192.168.201.25"))
					Expect(localAfter.Status.ContainerStatuses).To(HaveLen(1))
					Expect(localAfter.Status.ContainerStatuses[0].RestartCount).To(BeNumerically("==", 1))
				})
			})

			When("the local status is already up to date", func() {
				BeforeEach(func() {
					local.Status.Phase = corev1.PodRunning
					local.Status.PodIP = "192.168.201.25"
					local.Status.PodIPs = []corev1.PodIP{{IP: "192.168.201.25"}}
					local.Status.HostIP = LiqoNodeIP
					local.Status.HostIPs = []corev1.HostIP{{IP: LiqoNodeIP}}
					local.Status.ContainerStatuses = []corev1.ContainerStatus{{RestartCount: 1}}
					local.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}

					// Here, we create a modified fake client which returns an error when trying to perform an update operation.
					client.PrependReactor("update", "*", func(action testing.Action) (handled bool, _ runtime.Object, err error) {
						return true, nil, errors.New("should not call update")
					})
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			})

			When("the remote pod UID changes", func() {
				JustBeforeEach(func() {
					remote.SetUID("something-different")
					err = reflector.(*workload.NamespacedPodReflector).HandleStatus(
						trace.ContextWithTrace(ctx, trace.New("Pod")), local, remote, &podInfo)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should increment the local pod restart count", func() {
					// Here, we assert only a few fields, as already tested in the forge package.
					localAfter := GetPod(client, LocalNamespace, PodName)
					Expect(localAfter.Status.ContainerStatuses).To(HaveLen(1))
					Expect(localAfter.Status.ContainerStatuses[0].RestartCount).To(BeNumerically("==", 2))
				})
			})

			When("the remote pod is nil", func() {
				BeforeEach(func() {
					remote = nil

					// Here, we create a modified fake client which returns an error when trying to perform an update operation.
					client.PrependReactor("update", "*", func(action testing.Action) (handled bool, _ runtime.Object, err error) {
						return true, nil, errors.New("should not call update")
					})
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			})
		})

		Context("retrieval of the secret name associated with a given service account", func() {
			var (
				input, output string
				podinfo       workload.PodInfo
				err           error
			)

			BeforeEach(func() { input = "service-account"; podinfo = workload.PodInfo{} })

			JustBeforeEach(func() {
				output, err = reflector.(*workload.NamespacedPodReflector).RetrieveLegacyServiceAccountSecretName(&podinfo, input)
			})

			When("no secret is associated with the given service account", func() {
				BeforeEach(func() { CreateServiceAccountSecret(client, RemoteNamespace, "foo", "bar") })
				Context("the cached information is empty", func() {
					It("should return an error", func() { Expect(err).To(HaveOccurred()) })
				})

				Context("the cached information is present", func() {
					BeforeEach(func() { podinfo.ServiceAccountSecret = "secret-name" })
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should return the correct secret name", func() { Expect(output).To(BeIdenticalTo("secret-name")) })
					It("should not mutate the pod cache", func() { Expect(podinfo.ServiceAccountSecret).To(BeIdenticalTo("secret-name")) })
				})
			})

			When("multiple secrets are associated with the given service account", func() {
				BeforeEach(func() {
					CreateServiceAccountSecret(client, RemoteNamespace, "foo", "service-account")
					CreateServiceAccountSecret(client, RemoteNamespace, "bar", "service-account")
				})
				It("should return an error", func() { Expect(err).To(HaveOccurred()) })
			})

			When("a secret is associated with the given service account", func() {
				BeforeEach(func() {
					CreateServiceAccountSecret(client, RemoteNamespace, "secret-name", "service-account")
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should return the correct secret name", func() { Expect(output).To(BeIdenticalTo("secret-name")) })
				It("should correctly update the pod cache", func() { Expect(podinfo.ServiceAccountSecret).To(BeIdenticalTo("secret-name")) })
			})
		})

		Context("address translation", func() {
			var (
				input, output string
				podinfo       workload.PodInfo
				err           error
			)

			BeforeEach(func() { input = "192.168.200.25"; podinfo = workload.PodInfo{} })

			When("translating a remote to a local address", func() {
				JustBeforeEach(func() {
					output, err = reflector.(*workload.NamespacedPodReflector).MapPodIP(ctx, &podinfo, input)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should return the correct translations", func() { Expect(output).To(BeIdenticalTo("192.168.201.25")) })

				When("translating again the same set of IP addresses", func() {
					JustBeforeEach(func() {
						output, err = reflector.(*workload.NamespacedPodReflector).MapPodIP(ctx, &podinfo, input)
					})

					// The IPAMClient is configured to return an error if the same translation is requested twice.
					It("should succeed (i.e., use the cached values)", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should return the same translations", func() { Expect(output).To(BeIdenticalTo("192.168.201.25")) })
				})
			})
		})

		Context("pod restarts inference", func() {
			Status := func(name string, restarts int32) *corev1.PodStatus {
				return &corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: name, RestartCount: restarts}}}
			}

			DescribeTable("the InferAdditionalRestarts function",
				func(local, remote *corev1.PodStatus, expected int) {
					Expect(reflector.(*workload.NamespacedPodReflector).InferAdditionalRestarts(local, remote)).
						To(BeNumerically("==", expected))
				},
				Entry("when the local status is not yet configured", &corev1.PodStatus{}, &corev1.PodStatus{}, 0),
				Entry("when the local restarts are higher than the remote ones", Status("foo", 5), Status("foo", 3), 2),
				Entry("when the local restarts are equal to the remote ones", Status("foo", 5), Status("foo", 5), 0),
				Entry("when the local restarts are lower than the remote ones", Status("foo", 3), Status("foo", 5), 0),
				Entry("when the container names do not match", Status("foo", 5), Status("bar", 1), 0),
			)
		})
	})
})

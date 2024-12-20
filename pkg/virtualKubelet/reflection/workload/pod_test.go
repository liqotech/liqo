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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"k8s.io/utils/trace"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/workload"
)

var _ = Describe("Pod Reflection Tests", func() {
	Describe("the NewPodReflector function", func() {
		It("should not return a nil reflector", func() {
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 0,
				Type:       root.DefaultReflectorsTypes[resources.Pod],
			}
			reflector := workload.NewPodReflector(nil, nil,
				&workload.PodReflectorConfig{forge.APIServerSupportDisabled, false, "", "", fakeAPIServerRemapping(""), nil}, &reflectorConfig)
			Expect(reflector).ToNot(BeNil())
			Expect(reflector.Reflector).ToNot(BeNil())
		})
	})

	Describe("kubernetes.default service IP remapping", func() {
		var (
			kubernetesServiceIPGetter func(ctx context.Context) (string, error)

			output string
			err    error
		)

		BeforeEach(func() {
			metricsFactory := func(string) metricsv1beta1.PodMetricsInterface { return nil }
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 0,
				Type:       root.DefaultReflectorsTypes[resources.Pod],
			}
			reflector := workload.NewPodReflector(nil, metricsFactory,
				&workload.PodReflectorConfig{forge.APIServerSupportDisabled, false, "", "",
					fakeAPIServerRemapping("192.168.200.1"), &networkingv1beta1.Configuration{
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
			kubernetesServiceIPGetter = reflector.KubernetesServiceIPGetter()
		})

		JustBeforeEach(func() { output, err = kubernetesServiceIPGetter(ctx) })

		Context("the IP resource is correctly set", func() {
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should return the correct IP address", func() { Expect(output).To(BeIdenticalTo("192.168.200.1")) })

			When("retrieving again the remapped IP address", func() {
				JustBeforeEach(func() {
					output, err = kubernetesServiceIPGetter(ctx)
				})

				// The IPAMClient is configured to return an error if the same translation is requested twice.
				It("should succeed (i.e., use the cached values)", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should return the same translations", func() { Expect(output).To(BeIdenticalTo("192.168.200.1")) })
			})
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

			fallbackReflectorReady bool
		)

		BeforeEach(func() { local = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: PodName, Namespace: "not-existing"}} })

		JustBeforeEach(func() {
			client = fake.NewSimpleClientset(&local)
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)

			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 0,
				Type:       root.DefaultReflectorsTypes[resources.Pod],
			}
			reflector = workload.NewPodReflector(nil, nil,
				&workload.PodReflectorConfig{forge.APIServerSupportDisabled, false, "", "", fakeAPIServerRemapping(""), nil}, &reflectorConfig)

			opts := options.New(client, factory.Core().V1().Pods()).
				WithHandlerFactory(FakeEventHandler).
				WithReadinessFunc(func() bool { return fallbackReflectorReady }).
				WithEventBroadcaster(record.NewBroadcaster())
			fallback = reflector.NewFallback(opts)

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

				When("it is not terminating", func() {
					WhenBody := func(status corev1.PodStatus, phase corev1.PodPhase, reason string) func() {
						return func() {
							BeforeEach(func() { local.Status = status })

							It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
							It("should correctly set the pod phase and reason", func() {
								localAfter := GetPod(client, LocalNamespace, PodName)
								Expect(localAfter.Status.Phase).To(BeIdenticalTo(phase))
								Expect(localAfter.Status.Reason).To(BeIdenticalTo(reason))
							})
						}
					}

					When("phase is succeeded", WhenBody(corev1.PodStatus{Phase: corev1.PodSucceeded}, corev1.PodSucceeded, ""))
					When("phase is pending", WhenBody(corev1.PodStatus{Phase: corev1.PodPending}, corev1.PodPending, forge.PodOffloadingBackOffReason))
					When("phase is pending (and containers are present)", WhenBody(
						corev1.PodStatus{Phase: corev1.PodPending, ContainerStatuses: []corev1.ContainerStatus{{Name: "foo"}}},
						corev1.PodFailed, forge.PodOffloadingAbortedReason),
					)
					When("phase is running", WhenBody(corev1.PodStatus{Phase: corev1.PodRunning}, corev1.PodFailed, forge.PodOffloadingAbortedReason))
					When("phase is failed", WhenBody(corev1.PodStatus{Phase: corev1.PodFailed}, corev1.PodFailed, forge.PodOffloadingAbortedReason))
				})

				When("it is terminating", func() {
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
				JustBeforeEach(func() { fallbackReflectorReady = true })
				It("should return true", func() { Expect(fallback.Ready()).To(BeTrue()) })
			})
		})
	})
})

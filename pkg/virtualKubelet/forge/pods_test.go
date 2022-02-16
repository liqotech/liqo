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

package forge_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"k8s.io/utils/pointer"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Pod forging", func() {
	Translator := func(input string) string {
		return input + "-reflected"
	}

	BeforeEach(func() { forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP) })

	Describe("the LocalPod function", func() {
		const restarts = 3
		var local, remote, output *corev1.Pod

		BeforeEach(func() {
			local = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "local-name", Namespace: "local-namespace"}}
			remote = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "remote-name", Namespace: "remote-namespace"},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning, PodIP: "remote-ip",
					ContainerStatuses: []corev1.ContainerStatus{{Ready: true, RestartCount: 1}}},
			}
		})

		JustBeforeEach(func() { output = forge.LocalPod(local, remote, Translator, restarts) })

		It("should correctly propagate the local object meta", func() { Expect(output.ObjectMeta).To(Equal(local.ObjectMeta)) })
		It("should correctly propagate the remote status, translating the appropriate fields", func() {
			Expect(output.Status.Phase).To(Equal(corev1.PodRunning))
			Expect(output.Status.PodIP).To(Equal("remote-ip-reflected"))
			Expect(output.Status.PodIPs).To(ConsistOf(corev1.PodIP{IP: "remote-ip-reflected"}))
			Expect(output.Status.HostIP).To(Equal(LiqoNodeIP))
			Expect(output.Status.ContainerStatuses).To(HaveLen(1))
			Expect(output.Status.ContainerStatuses[0].Ready).To(BeTrue())
			Expect(output.Status.ContainerStatuses[0].RestartCount).To(BeNumerically("==", 4))
		})
	})

	Describe("the LocalPodOffloadedLabel function", func() {
		var (
			local       *corev1.Pod
			mutation    *corev1apply.PodApplyConfiguration
			needsUpdate bool
		)

		BeforeEach(func() {
			local = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "local-name", Namespace: "local-namespace"}}
		})

		JustBeforeEach(func() { mutation, needsUpdate = forge.LocalPodOffloadedLabel(local) })

		When("the expected labels is not present", func() {
			It("should mark update as needed", func() { Expect(needsUpdate).To(BeTrue()) })
			It("should correctly forge the apply patch", func() {
				Expect(mutation.Name).To(PointTo(Equal(local.GetName())))
				Expect(mutation.Namespace).To(PointTo(Equal(local.GetNamespace())))
				Expect(mutation.Labels).To(HaveKeyWithValue(consts.LocalPodLabelKey, consts.LocalPodLabelValue))
			})
		})

		When("the expected labels is already present", func() {
			BeforeEach(func() { local.Labels = map[string]string{consts.LocalPodLabelKey: consts.LocalPodLabelValue} })
			It("should mark update as not needed", func() { Expect(needsUpdate).To(BeFalse()) })
			It("should return a nil apply patch", func() { Expect(mutation).To(BeNil()) })
		})
	})

	Describe("the LocalRejectedPod function", func() {
		var local, original, output *corev1.Pod

		BeforeEach(func() {
			local = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "local-name", Namespace: "local-namespace"},
				Status: corev1.PodStatus{
					PodIP:             "1.1.1.1",
					Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
					ContainerStatuses: []corev1.ContainerStatus{{Name: "foo", Ready: true}},
				},
			}
		})

		JustBeforeEach(func() {
			original = local.DeepCopy()
			output = forge.LocalRejectedPod(local, corev1.PodFailed, forge.PodOffloadingAbortedReason)
		})

		It("should correctly propagate the local object meta", func() { Expect(output.ObjectMeta).To(Equal(local.ObjectMeta)) })
		It("should not mutate the input object", func() { Expect(local).To(Equal(original)) })
		It("should correctly set the rejected phase and reason", func() {
			Expect(output.Status.Phase).To(Equal(corev1.PodFailed))
			Expect(output.Status.Reason).To(Equal(forge.PodOffloadingAbortedReason))
		})
		It("should correctly mutate the pod conditions", func() {
			Expect(output.Status.Conditions).To(HaveLen(1))
			Expect(output.Status.Conditions[0].Type).To(Equal(corev1.PodReady))
			Expect(output.Status.Conditions[0].Status).To(Equal(corev1.ConditionFalse))
			Expect(output.Status.Conditions[0].Reason).To(Equal(forge.PodOffloadingAbortedReason))
			Expect(output.Status.Conditions[0].LastTransitionTime.Time).To(BeTemporally("~", time.Now()))
		})
		It("should correctly mutate the container statuses", func() {
			Expect(output.Status.ContainerStatuses).To(HaveLen(1))
			Expect(output.Status.ContainerStatuses[0].Ready).To(Equal(false))
		})
		It("should preserve the other status fields", func() { Expect(output.Status.PodIP).To(Equal(local.Status.PodIP)) })
	})

	Describe("the RemoteShadowPod function", func() {
		var (
			local          *corev1.Pod
			remote, output *vkv1alpha1.ShadowPod
		)

		BeforeEach(func() {
			local = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "local-name", Namespace: "local-namespace",
					Labels: map[string]string{"foo": "bar", consts.LocalPodLabelKey: consts.LocalPodLabelValue}},
				Spec: corev1.PodSpec{TerminationGracePeriodSeconds: pointer.Int64(15)},
			}
		})

		JustBeforeEach(func() { output = forge.RemoteShadowPod(local, remote, "remote-namespace") })

		Context("the remote pod does not exist", func() {
			It("should correctly forge the object meta", func() {
				Expect(output.GetName()).To(Equal("local-name"))
				Expect(output.GetNamespace()).To(Equal("remote-namespace"))
				Expect(output.GetLabels()).To(HaveKeyWithValue("foo", "bar"))
				Expect(output.GetLabels()).ToNot(HaveKeyWithValue(consts.LocalPodLabelKey, consts.LocalPodLabelValue))
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
			})

			It("should correctly reflect the pod spec", func() {
				// Here we assert only a single field, leaving the complete checks to the child functions tests.
				Expect(output.Spec.Pod.TerminationGracePeriodSeconds).To(PointTo(BeNumerically("==", 15)))
			})
		})

		Context("the remote pod already exists", func() {
			BeforeEach(func() {
				remote = &vkv1alpha1.ShadowPod{ObjectMeta: metav1.ObjectMeta{Name: "remote-name", Namespace: "remote-namespace", UID: "remote-uid"}}

				It("should correctly update the object meta", func() {
					Expect(output.GetName()).To(Equal("local-name"))
					Expect(output.GetNamespace()).To(Equal("remote-namespace"))
					Expect(output.UID).To(BeEquivalentTo("remote-uid"))
					Expect(output.GetLabels()).To(HaveKeyWithValue("foo", "bar"))
					Expect(output.GetLabels()).ToNot(HaveKeyWithValue(consts.LocalPodLabelKey, consts.LocalPodLabelValue))
				})

				It("should correctly update the pod spec", func() {
					// Here we assert only a single field, leaving the complete checks to the child functions tests.
					Expect(output.Spec.Pod.TerminationGracePeriodSeconds).To(PointTo(BeNumerically("==", 15)))
				})
			})
		})
	})

	Describe("the RemoteTolerations function", func() {
		var (
			included, excluded corev1.Toleration
			output             []corev1.Toleration
		)

		BeforeEach(func() {
			included = corev1.Toleration{
				Key:      corev1.TaintNodeNotReady,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			}
			excluded = corev1.Toleration{
				Key:      consts.VirtualNodeTolerationKey,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			}
		})

		JustBeforeEach(func() { output = forge.RemoteTolerations([]corev1.Toleration{included, excluded}) })
		It("should filter out liqo-related tolerations", func() { Expect(output).To(ConsistOf(included)) })
	})

	Describe("the *Stats functions", func() {
		PodStats := func(cpu, ram float64) statsv1alpha1.PodStats {
			Uint64Ptr := func(value uint64) *uint64 { return &value }
			return statsv1alpha1.PodStats{
				CPU:    &statsv1alpha1.CPUStats{UsageNanoCores: Uint64Ptr(uint64(cpu * 1e9))},
				Memory: &statsv1alpha1.MemoryStats{UsageBytes: Uint64Ptr(uint64(ram * 1e6)), WorkingSetBytes: Uint64Ptr(uint64(ram * 1e6))},
			}
		}

		ContainerMetrics := func(name string) *metricsv1beta1.ContainerMetrics {
			return &metricsv1beta1.ContainerMetrics{
				Name: name,
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(10, resource.Mega),
				},
			}
		}

		Describe("the LocalNodeStats function", func() {
			var (
				input  []statsv1alpha1.PodStats
				output *statsv1alpha1.Summary
			)

			BeforeEach(func() {
				forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP)
				input = []statsv1alpha1.PodStats{PodStats(0.2, 10), PodStats(0.5, 100)}
			})

			JustBeforeEach(func() { output = forge.LocalNodeStats(input) })

			It("should configure the correct node name and startup time", func() {
				Expect(output.Node.NodeName).To(BeIdenticalTo(LiqoNodeName))
				Expect(output.Node.StartTime.Time).To(BeTemporally("==", forge.StartTime))
			})

			It("should configure the correct CPU metrics", func() {
				Expect(output.Node.CPU.Time.Time).To(BeTemporally("~", time.Now(), time.Second))
				Expect(output.Node.CPU.UsageNanoCores).To(PointTo(BeNumerically("==", 700*1e6)))
			})

			It("should configure the correct memory metrics", func() {
				Expect(output.Node.Memory.Time.Time).To(BeTemporally("~", time.Now(), time.Second))
				Expect(output.Node.Memory.UsageBytes).To(PointTo(BeNumerically("==", 110*1e6)))
				Expect(output.Node.Memory.WorkingSetBytes).To(PointTo(BeNumerically("==", 110*1e6)))
			})

			It("should propagate the correct pod stats", func() {
				Expect(output.Pods).To(Equal(input))
			})
		})

		Describe("the LocalPodStats function", func() {
			var (
				pod     corev1.Pod
				metrics metricsv1beta1.PodMetrics
				output  statsv1alpha1.PodStats
			)

			BeforeEach(func() {
				pod = corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name: "name", Namespace: "namespace", UID: "uid", CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				}}
				metrics = metricsv1beta1.PodMetrics{
					Containers: []metricsv1beta1.ContainerMetrics{*ContainerMetrics("foo"), *ContainerMetrics("bar")},
				}
			})
			JustBeforeEach(func() { output = forge.LocalPodStats(&pod, &metrics) })

			It("should configure the correct pod reference", func() {
				Expect(output.PodRef.Name).To(BeIdenticalTo("name"))
				Expect(output.PodRef.Namespace).To(BeIdenticalTo("namespace"))
				Expect(output.PodRef.UID).To(Equal("uid"))
			})

			It("should configure the correct start time", func() {
				Expect(output.StartTime).To(Equal(pod.CreationTimestamp))
			})

			It("should configure the correct container stats", func() {
				GetName := func(cs statsv1alpha1.ContainerStats) string { return cs.Name }
				Expect(output.Containers).To(ContainElements(
					WithTransform(GetName, Equal("foo")),
					WithTransform(GetName, Equal("bar")),
				))
			})

			It("should configure the correct CPU metrics", func() {
				Expect(output.CPU.Time.Time).To(BeTemporally("~", time.Now(), time.Second))
				Expect(output.CPU.UsageNanoCores).To(PointTo(BeNumerically("==", 200*1e6)))
			})

			It("should configure the correct memory metrics", func() {
				Expect(output.Memory.Time.Time).To(BeTemporally("~", time.Now(), time.Second))
				Expect(output.Memory.UsageBytes).To(PointTo(BeNumerically("==", 20*1e6)))
				Expect(output.Memory.WorkingSetBytes).To(PointTo(BeNumerically("==", 20*1e6)))
			})
		})

		Describe("the LocalContainerStats function", func() {
			var (
				output statsv1alpha1.ContainerStats
				start  metav1.Time
				now    metav1.Time
			)

			BeforeEach(func() {
				start = metav1.NewTime(time.Now().Add(-1 * time.Hour))
				now = metav1.Now()
			})
			JustBeforeEach(func() { output = forge.LocalContainerStats(ContainerMetrics("container"), start, now) })

			It("should configure the correct name and start time", func() {
				Expect(output.Name).To(BeIdenticalTo("container"))
				Expect(output.StartTime).To(Equal(start))
			})

			It("should configure the correct CPU metrics", func() {
				Expect(output.CPU.Time).To(Equal(now))
				Expect(output.CPU.UsageNanoCores).To(PointTo(BeNumerically("==", 100*1e6)))
			})

			It("should configure the correct memory metrics", func() {
				Expect(output.Memory.Time).To(Equal(now))
				Expect(output.Memory.UsageBytes).To(PointTo(BeNumerically("==", 10*1e6)))
				Expect(output.Memory.WorkingSetBytes).To(PointTo(BeNumerically("==", 10*1e6)))
			})
		})
	})
})

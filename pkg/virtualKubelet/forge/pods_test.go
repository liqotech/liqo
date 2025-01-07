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

package forge_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Pod forging", func() {
	Translator := func(input string) string { return input + "-reflected" }
	SASecretRetriever := func(input string) string { return input + "-secret" }
	KubernetesServiceIPGetter := func() string { return "k8ssvcaddr" }

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
			Expect(output.Status.HostIPs).To(HaveLen(1))
			Expect(output.Status.HostIPs[0]).To(Equal(corev1.HostIP{IP: LiqoNodeIP}))
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
			remote, output *offloadingv1beta1.ShadowPod
			forgingOpts    *forge.ForgingOpts
		)

		Mutator := func(remote *corev1.PodSpec) {
			remote.ActiveDeadlineSeconds = pointer.Int64(99)
		}

		BeforeEach(func() {
			local = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "local-name", Namespace: "local-namespace",
					Labels: map[string]string{
						"foo":                             "bar",
						consts.LocalPodLabelKey:           consts.LocalPodLabelValue,
						testutil.FakeNotReflectedLabelKey: "true",
					},
					Annotations: map[string]string{
						testutil.FakeNotReflectedAnnotKey: "true",
					},
				},
				Spec: corev1.PodSpec{TerminationGracePeriodSeconds: pointer.Int64(15)},
			}

			forgingOpts = testutil.FakeForgingOpts()
		})

		JustBeforeEach(func() {
			output = forge.RemoteShadowPod(local, remote, "remote-namespace", forgingOpts, Mutator)
		})

		Context("the remote pod does not exist", func() {
			It("should correctly forge the object meta", func() {
				Expect(output.GetName()).To(Equal("local-name"))
				Expect(output.GetNamespace()).To(Equal("remote-namespace"))
				Expect(output.Labels).To(HaveKeyWithValue("foo", "bar"))
				Expect(output.Labels).ToNot(HaveKeyWithValue(consts.LocalPodLabelKey, consts.LocalPodLabelValue))
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
				Expect(output.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
				Expect(output.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
			})

			It("should correctly reflect the pod spec", func() {
				// Here we assert only a single field, leaving the complete checks to the child functions tests.
				Expect(output.Spec.Pod.TerminationGracePeriodSeconds).To(PointTo(BeNumerically("==", 15)))
			})

			It("should correctly trigger the additional mutators", func() {
				// Here we assert only a single field, leaving the complete checks to the child functions tests.
				Expect(output.Spec.Pod.ActiveDeadlineSeconds).To(PointTo(BeNumerically("==", 99)))
			})
		})

		Context("the remote pod already exists", func() {
			BeforeEach(func() {
				remote = &offloadingv1beta1.ShadowPod{ObjectMeta: metav1.ObjectMeta{Name: "remote-name", Namespace: "remote-namespace", UID: "remote-uid"}}
			})

			It("should correctly update the object meta", func() {
				Expect(output.GetName()).To(Equal("remote-name"))
				Expect(output.GetNamespace()).To(Equal("remote-namespace"))
				Expect(output.UID).To(BeEquivalentTo("remote-uid"))
				Expect(output.Labels).To(HaveKeyWithValue("foo", "bar"))
				Expect(output.Labels).ToNot(HaveKeyWithValue(consts.LocalPodLabelKey, consts.LocalPodLabelValue))
				Expect(output.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
				Expect(output.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
			})

			It("should not update the pod spec", func() {
				Expect(output.Spec.Pod).To(Equal(corev1.PodSpec{}))
			})
		})
	})

	Describe("the APIServerSupportMutator function", func() {
		const saName = "service-account"

		var (
			apiServerSupport                     forge.APIServerSupportType
			remote, original                     *corev1.PodSpec
			homeAPIServerHost, homeAPIServerPort string
			localAnnotations                     map[string]string
		)

		BeforeEach(func() {
			remote = &corev1.PodSpec{
				Containers:     []corev1.Container{{Name: "foo", Image: "foo/bar:v0.1-alpha3"}},
				InitContainers: []corev1.Container{{Name: "bar", Image: "foo/baz:v0.1-alpha3"}},
				Volumes: []corev1.Volume{{Name: "volume", VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{
						{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{Path: "other", Audience: "custom"}},
					}}}}},
			}
			localAnnotations = map[string]string{}
		})

		JustBeforeEach(func() {
			original = remote.DeepCopy()
			forge.APIServerSupportMutator(apiServerSupport, localAnnotations, saName, SASecretRetriever,
				KubernetesServiceIPGetter, homeAPIServerHost, homeAPIServerPort)(remote)
		})

		When("API server support is enabled", func() {
			WhenBody := func() {
				It("should correctly mutate the volumes", func() {
					Expect(remote.Volumes).To(Equal(forge.RemoteVolumes(original.Volumes, apiServerSupport,
						func() string { return SASecretRetriever(saName) })))
				})

				It("should appropriately mutate the remote containers", func() {
					Expect(remote.Containers).To(Equal(forge.RemoteContainersAPIServerSupport(original.Containers, saName, homeAPIServerHost, homeAPIServerPort)))
				})

				It("should appropriately mutate the remote init containers", func() {
					Expect(remote.InitContainers).To(Equal(forge.RemoteContainersAPIServerSupport(original.InitContainers, saName,
						homeAPIServerHost, homeAPIServerPort)))
				})

				It("should appropriately mutate host aliases", func() {
					Expect(remote.HostAliases).To(Equal(forge.RemoteHostAliasesAPIServerSupport(original.HostAliases, KubernetesServiceIPGetter)))
				})
			}

			When("legacy mode is set", func() {
				BeforeEach(func() {
					apiServerSupport = forge.APIServerSupportLegacy
					homeAPIServerHost = ""
					homeAPIServerPort = ""
				})
				WhenBody()
			})

			When("token API mode is set", func() {
				BeforeEach(func() {
					apiServerSupport = forge.APIServerSupportTokenAPI
					homeAPIServerHost = ""
					homeAPIServerPort = ""
				})
				WhenBody()
			})
		})

		When("API server support is enabled with custom host API server Host & Port", func() {

			WhenBody := func() {
				It("should correctly mutate the volumes", func() {
					Expect(remote.Volumes).To(Equal(forge.RemoteVolumes(original.Volumes, apiServerSupport,
						func() string { return SASecretRetriever(saName) })))
				})

				It("should appropriately mutate the remote containers", func() {
					Expect(remote.Containers).To(Equal(forge.RemoteContainersAPIServerSupport(original.Containers, saName, homeAPIServerHost, homeAPIServerPort)))
				})

				It("should appropriately mutate the remote init containers", func() {
					Expect(remote.InitContainers).To(Equal(forge.RemoteContainersAPIServerSupport(original.InitContainers, saName,
						homeAPIServerHost, homeAPIServerPort)))
				})
			}

			When("legacy mode is set with custom host API server Host & Port", func() {
				BeforeEach(func() {
					apiServerSupport = forge.APIServerSupportLegacy
					homeAPIServerHost = "custom.apiserver.com"
					homeAPIServerPort = "6443"
				})
				WhenBody()
			})

			When("token API mode is set with custom host API server Host & Port", func() {
				BeforeEach(func() {
					apiServerSupport = forge.APIServerSupportTokenAPI
					homeAPIServerHost = "custom1.apiserver.com"
					homeAPIServerPort = "6443"
				})
				WhenBody()
			})
		})

		When("API server support is disabled", func() {
			BeforeEach(func() { apiServerSupport = forge.APIServerSupportDisabled })

			It("should correctly mutate the volumes", func() {
				Expect(remote.Volumes).To(Equal(forge.RemoteVolumes(original.Volumes, apiServerSupport,
					func() string { return SASecretRetriever(saName) })))
			})

			It("should not mutate the remote containers", func() { Expect(remote.Containers).To(Equal(original.Containers)) })
			It("should not mutate the remote init containers", func() { Expect(remote.InitContainers).To(Equal(original.InitContainers)) })
			It("should not mutate the remote host aliases", func() { Expect(remote.HostAliases).To(Equal(original.HostAliases)) })
		})
	})

	Describe("the AntiAffinityMutator functions", func() {
		var (
			labels map[string]string
			remote *corev1.PodSpec
		)

		BeforeEach(func() {
			labels = map[string]string{"foo": "bar", "baz": "qux"}
			remote = &corev1.PodSpec{}
		})

		CommonBody := func() {
			It("should only mutate the pod affinities", func() {
				remote.Affinity = nil
				Expect(remote).To(Equal(&corev1.PodSpec{}))
			})

			It("should forge the appropriate anti-affinity", func() {
				Expect(remote.Affinity).ToNot(BeNil())
				Expect(remote.Affinity.PodAffinity).To(BeNil())
				Expect(remote.Affinity.PodAntiAffinity).ToNot(BeNil())
				Expect(remote.Affinity.NodeAffinity).To(BeNil())
			})
		}

		Describe("the AntiAffinityPropagateMutator function", func() {
			var affinity *corev1.Affinity

			JustBeforeEach(func() { forge.AntiAffinityPropagateMutator(affinity)(remote) })

			When("the local affinity is not set", func() {
				BeforeEach(func() { affinity = nil })
				It("should not mutate the remote affinity", func() { Expect(remote.Affinity).To(BeNil()) })
			})

			When("the local pod anti-affinity is not set", func() {
				BeforeEach(func() { affinity = &corev1.Affinity{} })
				It("should not mutate the remote affinity", func() { Expect(remote.Affinity).To(BeNil()) })
			})

			When("the local pod anti-affinity is set", func() {
				BeforeEach(func() {
					affinity = &corev1.Affinity{
						PodAffinity:  &corev1.PodAffinity{},
						NodeAffinity: &corev1.NodeAffinity{},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "kubernetes.io/hostname"}},
						}}
				})

				Context("preliminary checks", func() { CommonBody() })
				It("should propagate the appropriate anti-affinity constraints", func() {
					constraints := remote.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
					Expect(constraints).To(HaveLen(1))
					Expect(constraints[0].TopologyKey).To(Equal("kubernetes.io/hostname"))
				})
			})
		})

		Describe("the AntiAffinitySoftMutator function", func() {
			JustBeforeEach(func() { forge.AntiAffinitySoftMutator(labels)(remote) })

			Context("preliminary checks", func() { CommonBody() })

			It("should forge the appropriate anti-affinity constraints", func() {
				constraints := remote.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution
				Expect(constraints).To(HaveLen(1))
				Expect(constraints[0].PodAffinityTerm.LabelSelector.MatchLabels).To(Equal(labels))
				Expect(constraints[0].PodAffinityTerm.LabelSelector.MatchExpressions).To(HaveLen(0))
				Expect(constraints[0].PodAffinityTerm.TopologyKey).To(Equal("kubernetes.io/hostname"))
			})
		})

		Describe("the AntiAffinityHardMutator function", func() {
			JustBeforeEach(func() { forge.AntiAffinityHardMutator(labels)(remote) })

			Context("preliminary checks", func() { CommonBody() })

			It("should forge the appropriate anti-affinity constraints", func() {
				constraints := remote.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
				Expect(constraints[0].LabelSelector.MatchLabels).To(Equal(labels))
				Expect(constraints[0].LabelSelector.MatchExpressions).To(HaveLen(0))
				Expect(constraints[0].TopologyKey).To(Equal("kubernetes.io/hostname"))
			})
		})
	})

	Describe("the FilterAntiAffinityLabels function", func() {
		var (
			input, output map[string]string
			whitelist     string
		)

		BeforeEach(func() {
			input = map[string]string{
				"key1":                     "value1",
				"key2":                     "value2",
				"key3":                     "value3",
				"controller-revision-hash": "value4",
			}
		})

		JustBeforeEach(func() { output = forge.FilterAntiAffinityLabels(input, whitelist) })

		When("a whitelist is specified", func() {
			BeforeEach(func() { whitelist = "key2,key3" })
			It("should preserve only the appropriate label", func() {
				Expect(output).To(HaveLen(2))
				Expect(output).To(HaveKeyWithValue("key2", "value2"))
				Expect(output).To(HaveKeyWithValue("key3", "value3"))
			})
		})

		When("no whitelist is specified", func() {
			BeforeEach(func() { whitelist = "" })
			It("should remove the labels added by kubernetes", func() {
				Expect(output).To(HaveLen(3))
				Expect(output).To(HaveKeyWithValue("key1", "value1"))
				Expect(output).To(HaveKeyWithValue("key2", "value2"))
				Expect(output).To(HaveKeyWithValue("key3", "value3"))
			})
		})
	})

	Describe("the runtimeClassNameMutator function", func() {
		const (
			fakeRuntimeClassOffPatch = "foo"
			fakeRuntimeClassPodSpec  = "bar"
			fakeRuntimeClassPodAnnot = "baz"
		)

		var (
			local       *corev1.Pod
			remote      *corev1.PodSpec
			forgingOpts *forge.ForgingOpts
		)

		BeforeEach(func() {
			local = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: corev1.PodSpec{},
			}
			remote = &corev1.PodSpec{}
			forgingOpts = &forge.ForgingOpts{}
		})

		JustBeforeEach(func() { forge.RuntimeClassNameMutator(local, forgingOpts)(remote) })

		When("runtimeclass is set in the OffloadingPatch", func() {
			BeforeEach(func() {
				forgingOpts.RuntimeClassName = ptr.To(fakeRuntimeClassOffPatch)
			})

			It("should set the runtimeclass of the OffloadingPatch", func() {
				Expect(remote.RuntimeClassName).To(PointTo(Equal(fakeRuntimeClassOffPatch)))
			})
		})

		When("runtimeclass is set in the OffloadingPatch and in the Pod spec", func() {
			BeforeEach(func() {
				forgingOpts.RuntimeClassName = ptr.To(fakeRuntimeClassOffPatch)
				local.Spec.RuntimeClassName = ptr.To(fakeRuntimeClassPodSpec)
			})

			It("should set the runtimeclass of the pod spec", func() {
				Expect(remote.RuntimeClassName).To(PointTo(Equal(fakeRuntimeClassPodSpec)))
			})
		})

		When("runtimeclass is set in the OffloadingPatch, Pod spec and Pod annotation", func() {
			BeforeEach(func() {
				forgingOpts.RuntimeClassName = ptr.To(fakeRuntimeClassOffPatch)
				local.Spec.RuntimeClassName = ptr.To(fakeRuntimeClassPodSpec)
				local.Annotations = map[string]string{consts.RemoteRuntimeClassNameAnnotKey: fakeRuntimeClassPodAnnot}
			})

			It("should set the runtimeclass of the pod annotation", func() {
				Expect(remote.RuntimeClassName).To(PointTo(Equal(fakeRuntimeClassPodAnnot)))
			})
		})

		When("runtimeclass is set to the liqo one", func() {
			BeforeEach(func() {
				local.Spec.RuntimeClassName = ptr.To(consts.LiqoRuntimeClassName)
			})

			When("OffloadingPatch and Pod annotation are not set", func() {
				It("should leave the runtimeclass empty", func() {
					Expect(remote.RuntimeClassName).To(BeNil())
				})
			})

			When("OffloadingPatch is set", func() {
				BeforeEach(func() {
					forgingOpts.RuntimeClassName = ptr.To(fakeRuntimeClassOffPatch)
				})

				It("should set the runtimeclass of the OffloadingPatch", func() {
					Expect(remote.RuntimeClassName).To(PointTo(Equal(fakeRuntimeClassOffPatch)))
				})
			})

			When("Pod annotation is set", func() {
				BeforeEach(func() {
					local.Annotations = map[string]string{consts.RemoteRuntimeClassNameAnnotKey: fakeRuntimeClassPodAnnot}
				})

				It("should set the runtimeclass of the Pod annotation", func() {
					Expect(remote.RuntimeClassName).To(PointTo(Equal(fakeRuntimeClassPodAnnot)))
				})
			})
		})
	})

	Describe("the RemoteContainersAPIServerSupport function", func() {
		var container corev1.Container
		var output []corev1.Container
		var homeAPIServerHost, homeAPIServerPort = "", ""

		BeforeEach(func() {
			container = corev1.Container{
				Name:  "foo",
				Image: "foo/bar:v0.1-alpha3",
				Args:  []string{"--first", "--second"},
				Ports: []corev1.ContainerPort{{Name: "foo-port", ContainerPort: 8080}},
				Env: []corev1.EnvVar{
					{Name: "ENV_1", Value: "VALUE_1"},
					{Name: "ENV_2", Value: "VALUE_2"},
					{Name: "ENV_3", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{Key: "foo"}}},
					{Name: "ENV_4", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
					{Name: "ENV_5", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.serviceAccountName"}}},
				},
			}
		})

		JustBeforeEach(func() {
			output = forge.RemoteContainersAPIServerSupport([]corev1.Container{container}, "service-account-name", homeAPIServerHost, homeAPIServerPort)
		})

		It("should propagate all container values", func() {
			// Remove the environment variables from the containers, as checked in the test below.
			envcleaner := func(c corev1.Container) corev1.Container {
				c.Env = nil
				return c
			}

			Expect(output).To(HaveLen(1))
			Expect(envcleaner(output[0])).To(Equal(envcleaner(container)))
		})

		It("should configure the appropriate environment variables", func() {
			Expect(output).To(HaveLen(1))
			Expect(output[0].Env).To(ConsistOf(
				corev1.EnvVar{Name: "ENV_1", Value: "VALUE_1"},
				corev1.EnvVar{Name: "ENV_2", Value: "VALUE_2"},
				corev1.EnvVar{Name: "ENV_3", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{Key: "foo"}}},
				corev1.EnvVar{Name: "ENV_4", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
				corev1.EnvVar{Name: "ENV_5", Value: "service-account-name"},
				corev1.EnvVar{Name: "KUBERNETES_SERVICE_PORT", Value: "8443"},
				corev1.EnvVar{Name: "KUBERNETES_SERVICE_HOST", Value: "kubernetes.default"},
				corev1.EnvVar{Name: "KUBERNETES_PORT", Value: "tcp://kubernetes.default:8443"},
				corev1.EnvVar{Name: "KUBERNETES_PORT_8443_TCP", Value: "tcp://kubernetes.default:8443"},
				corev1.EnvVar{Name: "KUBERNETES_PORT_8443_TCP_PROTO", Value: "tcp"},
				corev1.EnvVar{Name: "KUBERNETES_PORT_8443_TCP_PORT", Value: "8443"},
				corev1.EnvVar{Name: "KUBERNETES_PORT_8443_TCP_ADDR", Value: "kubernetes.default"},
			))
		})
	})

	Describe("the RemoteContainersAPIServerSupport function with custom host API server Host & Port", func() {
		var container corev1.Container
		var output []corev1.Container
		var homeAPIServerHost, homeAPIServerPort = "custom.apiserver.com", "6443"

		BeforeEach(func() {
			container = corev1.Container{
				Name:  "foo",
				Image: "foo/bar:v0.1-alpha3",
				Args:  []string{"--first", "--second"},
				Ports: []corev1.ContainerPort{{Name: "foo-port", ContainerPort: 8080}},
				Env: []corev1.EnvVar{
					{Name: "ENV_1", Value: "VALUE_1"},
					{Name: "ENV_2", Value: "VALUE_2"},
					{Name: "ENV_3", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{Key: "foo"}}},
					{Name: "ENV_4", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
					{Name: "ENV_5", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.serviceAccountName"}}},
				},
			}
		})

		JustBeforeEach(func() {
			output = forge.RemoteContainersAPIServerSupport([]corev1.Container{container}, "service-account-name", homeAPIServerHost, homeAPIServerPort)
		})

		It("should propagate all container values", func() {
			// Remove the environment variables from the containers, as checked in the test below.
			envcleaner := func(c corev1.Container) corev1.Container {
				c.Env = nil
				return c
			}

			Expect(output).To(HaveLen(1))
			Expect(envcleaner(output[0])).To(Equal(envcleaner(container)))
		})

		It("should configure the appropriate environment variables", func() {
			Expect(output).To(HaveLen(1))
			Expect(output[0].Env).To(ConsistOf(
				corev1.EnvVar{Name: "ENV_1", Value: "VALUE_1"},
				corev1.EnvVar{Name: "ENV_2", Value: "VALUE_2"},
				corev1.EnvVar{Name: "ENV_3", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{Key: "foo"}}},
				corev1.EnvVar{Name: "ENV_4", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
				corev1.EnvVar{Name: "ENV_5", Value: "service-account-name"},
				corev1.EnvVar{Name: "KUBERNETES_SERVICE_PORT", Value: homeAPIServerPort},
				corev1.EnvVar{Name: "KUBERNETES_SERVICE_HOST", Value: homeAPIServerHost},
				corev1.EnvVar{Name: "KUBERNETES_PORT", Value: fmt.Sprintf("tcp://%s:%s", homeAPIServerHost, homeAPIServerPort)},
				corev1.EnvVar{Name: fmt.Sprintf("KUBERNETES_PORT_%s_TCP", homeAPIServerPort),
					Value: fmt.Sprintf("tcp://%s:%s", homeAPIServerHost, homeAPIServerPort)},
				corev1.EnvVar{Name: fmt.Sprintf("KUBERNETES_PORT_%s_TCP_PROTO", homeAPIServerPort), Value: "tcp"},
				corev1.EnvVar{Name: fmt.Sprintf("KUBERNETES_PORT_%s_TCP_PORT", homeAPIServerPort), Value: homeAPIServerPort},
				corev1.EnvVar{Name: fmt.Sprintf("KUBERNETES_PORT_%s_TCP_ADDR", homeAPIServerPort), Value: homeAPIServerHost},
			))
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

	Describe("the RemoteVolumes function", func() {
		var volumes, output []corev1.Volume
		var apiServerSupport forge.APIServerSupportType

		BeforeEach(func() {
			volumes = []corev1.Volume{
				{Name: "first", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{}}},
				{Name: "second", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{}}},
				{Name: "third", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{}}},
				{Name: "forth", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{}}},
				{Name: "kube-api-access-foo", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{
					{ConfigMap: &corev1.ConfigMapProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: forge.RootCAConfigMapName},
						Items:                []corev1.KeyToPath{{Key: "ca.crt", Path: "ca.crt"}}}},
					{DownwardAPI: &corev1.DownwardAPIProjection{Items: []corev1.DownwardAPIVolumeFile{{Path: "namespace"}}}},
					{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{Path: "token"}},
				}}}},
				{Name: "custom-bar", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{
					{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{Path: "other", Audience: "custom"}},
				}}}},
			}
		})

		JustBeforeEach(func() {
			output = forge.RemoteVolumes(volumes, apiServerSupport, func() string { return "service-account-secret" })
		})

		WhenBodyCommon := func(projectedSourcesLenKubeAPI, projectedSourcesLenCustom int) {
			It("should propagate all volume types, except the one referring to the service account (which is mutated)", func() {
				Expect(output).To(HaveLen(6))
				Expect(output[0:3]).To(ConsistOf(volumes[0:3]))
			})

			It("should mutate the service account projected volume (with the kube-api token)", func() {
				Expect(output).To(HaveLen(6))
				Expect(output[4].Name).To(Equal("kube-api-access-foo"))
				Expect(output[4].Projected.Sources).To(HaveLen(projectedSourcesLenKubeAPI))

				Expect(output[4].Projected.Sources[0]).To(Equal(corev1.VolumeProjection{
					ConfigMap: &corev1.ConfigMapProjection{
						// The configmap name is mutated to account for the remapping.
						LocalObjectReference: corev1.LocalObjectReference{Name: forge.RootCAConfigMapName + ".local"},
						Items:                []corev1.KeyToPath{{Key: "ca.crt", Path: "ca.crt"}},
					}},
				))

				Expect(output[4].Projected.Sources[1]).To(Equal(corev1.VolumeProjection{
					DownwardAPI: &corev1.DownwardAPIProjection{Items: []corev1.DownwardAPIVolumeFile{{Path: "namespace"}}}},
				))
			})

			It("should mutate the service account projected volume (with the custom token)", func() {
				Expect(output).To(HaveLen(6))
				Expect(output[5].Name).To(Equal("custom-bar"))
				Expect(output[5].Projected.Sources).To(HaveLen(projectedSourcesLenCustom))
			})
		}

		When("API server support is enabled", func() {
			ItBody := func(key string) func() {
				return func() {
					Expect(output).To(HaveLen(6))
					Expect(output[4].Name).To(Equal("kube-api-access-foo"))
					Expect(output[4].Projected.Sources).To(HaveLen(3))

					Expect(output[4].Projected.Sources[2]).To(Equal(corev1.VolumeProjection{
						// The service account entry is replaced with the one of the corresponding secret.
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "service-account-secret"},
							Items:                []corev1.KeyToPath{{Key: key, Path: corev1.ServiceAccountTokenKey}},
						}},
					))
				}
			}

			When("legacy mode is set", func() {
				BeforeEach(func() { apiServerSupport = forge.APIServerSupportLegacy })
				WhenBodyCommon(3, 0)

				It("should mutate the service account projected volume (with the kube-api token), adding a secret entry",
					ItBody(corev1.ServiceAccountTokenKey))
			})

			When("token API mode is set", func() {
				BeforeEach(func() { apiServerSupport = forge.APIServerSupportTokenAPI })
				WhenBodyCommon(3, 1)

				It("should mutate the service account projected volume (with the kube-api token), adding a secret entry",
					ItBody(forge.ServiceAccountTokenKey("kube-api-access-foo", "token")))

				It("should mutate the service account projected volume (with the custom token), adding a secret entry", func() {
					Expect(output).To(HaveLen(6))
					Expect(output[5].Name).To(Equal("custom-bar"))
					Expect(output[5].Projected.Sources).To(HaveLen(1))

					Expect(output[5].Projected.Sources[0]).To(Equal(corev1.VolumeProjection{
						// The service account entry is replaced with the one of the corresponding secret.
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "service-account-secret"},
							Items: []corev1.KeyToPath{
								{Key: forge.ServiceAccountTokenKey("custom-bar", "other"), Path: "other"}},
						}},
					))
				})
			})
		})

		When("API server support is disabled", func() {
			BeforeEach(func() { apiServerSupport = forge.APIServerSupportDisabled })

			WhenBodyCommon(2, 0)
		})
	})

	Describe("the RemoteHostAliasesAPIServerSupport function", func() {
		var aliases, output []corev1.HostAlias

		BeforeEach(func() {
			aliases = []corev1.HostAlias{
				{IP: "8.8.4.4", Hostnames: []string{"dns.google."}},
				{IP: "8.8.8.8", Hostnames: []string{"dns.google."}},
			}
		})

		JustBeforeEach(func() { output = forge.RemoteHostAliasesAPIServerSupport(aliases, KubernetesServiceIPGetter) })

		It("should preserve the existing aliases", func() { Expect(output).To(ContainElements(aliases)) })
		It("should append the alias corresponding to the kubernetes.default service", func() {
			Expect(output).To(ContainElement(corev1.HostAlias{
				Hostnames: []string{"kubernetes.default", "kubernetes.default.svc"}, IP: KubernetesServiceIPGetter(),
			}))
		})
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

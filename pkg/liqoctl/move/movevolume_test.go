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

package move

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
)

// Create an alias to avoid "dot" importing the gstruct package, as it conflicts with the Options struct.
var PointTo = gstruct.PointTo

var _ = Context("Move Volumes", func() {

	var ctx = context.Background()
	var resticRepositoryURL = "restic.example.com"
	var resticPassword = "restic-password"

	var newPv = func(name string) *corev1.PersistentVolume {
		return &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
	}

	var newPvc = func(name string) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				UID:       types.UID(name),
			},
		}
	}

	var addNodeMount = func(pvc *corev1.PersistentVolumeClaim, nodeName string) *corev1.PersistentVolumeClaim {
		if pvc.Annotations == nil {
			pvc.Annotations = make(map[string]string)
		}
		pvc.Annotations["volume.kubernetes.io/selected-node"] = nodeName
		return pvc
	}

	var newPod = func(name, namespace string, volumes []string) *corev1.Pod {
		mountedVolumes := make([]corev1.Volume, len(volumes))
		for i, volume := range volumes {
			mountedVolumes[i] = corev1.Volume{
				Name: volume,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: volume,
					},
				},
			}
		}

		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				Volumes: mountedVolumes,
			},
		}
	}

	var newStatefulSet = func(name, namespace string) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	}

	var newService = func(name, namespace string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	}

	var newNode = func(name string, local bool) *corev1.Node {
		labels := map[string]string{}
		if !local {
			labels[liqoconst.TypeLabel] = liqoconst.TypeNode
		}

		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
		}
	}

	Context("volume utils", func() {

		type mounterTestcase struct {
			client      client.Client
			pvc         *corev1.PersistentVolumeClaim
			expectedErr OmegaMatcher
		}

		DescribeTable("checkNoMounter function", func(c mounterTestcase) {
			err := checkNoMounter(ctx, c.client, c.pvc)
			Expect(err).To(c.expectedErr)
		}, Entry("should return nil if no mounter pod is found", mounterTestcase{
			client:      fake.NewClientBuilder().WithObjects(newPod("pod1", "default", []string{})).Build(),
			pvc:         newPvc("pvc1"),
			expectedErr: BeNil(),
		}), Entry("should return an error if a pod is mounting the volume", mounterTestcase{
			client: fake.NewClientBuilder().WithObjects(
				newPod("pod1", "default", []string{"pvc1"}),
				newPod("pod2", "default", []string{}),
				newPod("pod3", "default", []string{"pvc3"}),
				newPod("pod3", "ns2", []string{"pvc3"}),
				newPod("pod4", "default", []string{"pvc4"}),
			).Build(),
			pvc: newPvc("pvc3"),
			expectedErr: MatchError(fmt.Errorf("the volume (%s/%s) must not to be mounted by any pod, but found mounter pod %s/%s",
				"default", "pvc3", "default", "pod3")),
		}))

		type isLocalVolumeTestcase struct {
			client        client.Client
			pvc           *corev1.PersistentVolumeClaim
			expectedErr   OmegaMatcher
			expectedLocal OmegaMatcher
			expectedNode  OmegaMatcher
		}

		DescribeTable("isLocalVolume function", func(c isLocalVolumeTestcase) {
			local, node, err := isLocalVolume(ctx, c.client, c.pvc)
			Expect(err).To(c.expectedErr)
			Expect(local).To(c.expectedLocal)
			Expect(node).To(c.expectedNode)
		}, Entry("should return error if no node annotation is found", isLocalVolumeTestcase{
			client:        fake.NewClientBuilder().Build(),
			pvc:           newPvc("pvc1"),
			expectedErr:   HaveOccurred(),
			expectedLocal: BeFalse(),
			expectedNode:  BeNil(),
		}), Entry("should return true if the node is local", isLocalVolumeTestcase{
			client:        fake.NewClientBuilder().WithObjects(newNode("node1", true)).Build(),
			pvc:           addNodeMount(newPvc("pvc1"), "node1"),
			expectedErr:   BeNil(),
			expectedLocal: BeTrue(),
			expectedNode:  &MatchObject{Name: "node1"},
		}), Entry("should return false if the node is not local", isLocalVolumeTestcase{
			client:        fake.NewClientBuilder().WithObjects(newNode("node1", false)).Build(),
			pvc:           addNodeMount(newPvc("pvc1"), "node1"),
			expectedErr:   BeNil(),
			expectedLocal: BeFalse(),
			expectedNode:  &MatchObject{Name: "node1"},
		}))

		Context("recreatePvc function", func() {

			var (
				cl     client.Client
				oldPvc *corev1.PersistentVolumeClaim
			)

			BeforeEach(func() {
				oldPvc = newPvc("pvc1")
				oldPvc.Spec.VolumeName = "pv1"
				cl = fake.NewClientBuilder().WithObjects(oldPvc).Build()
			})

			It("should recreate the PVC with an empty volume name", func() {
				pvc, err := recreatePvc(ctx, cl, oldPvc)
				Expect(err).To(BeNil())
				Expect(pvc.Name).To(Equal(oldPvc.Name))
				Expect(pvc.Namespace).To(Equal(oldPvc.Namespace))
				Expect(pvc.Spec.VolumeName).To(BeEmpty())
			})
		})
	})

	Context("forge jobs", func() {

		Context("createSnapshotterJob", func() {

			var (
				o   Options
				cl  client.Client
				pvc *corev1.PersistentVolumeClaim
			)

			BeforeEach(func() {
				pv := newPv("pv1")

				pvc = newPvc("pvc1")
				pvc.Spec.VolumeName = pv.Name

				cl = fake.NewClientBuilder().WithObjects(pv, pvc).Build()
				o = Options{Factory: &factory.Factory{CRClient: cl}, ResticPassword: resticPassword,
					ContainersCPURequests: resource.MustParse("100m"), ContainersRAMLimits: resource.MustParse("100M"),
					ResticImage: DefaultResticImage, ResticServerImage: DefaultResticServerImage}
			})

			When("creates a snapshotter job", func() {

				var (
					job     *batchv1.Job
					err     error
					podSpec *corev1.PodSpec
				)

				BeforeEach(func() {
					job, err = o.createSnapshotterJob(ctx, pvc, resticRepositoryURL)
					Expect(err).ToNot(HaveOccurred())
					Expect(job).ToNot(BeNil())

					podSpec = &job.Spec.Template.Spec
				})

				It("should create a job with the correct namespace", func() {
					Expect(job.Namespace).To(Equal(pvc.Namespace))
				})

				It("should create pod that restart on failures", func() {
					Expect(podSpec.RestartPolicy).To(Equal(corev1.RestartPolicyOnFailure))
				})

				It("should populate the initContainers", func() {
					Expect(podSpec.InitContainers).To(HaveLen(1))
					Expect(podSpec.InitContainers[0].Image).To(Equal(DefaultResticImage))
					Expect(podSpec.InitContainers[0].Args).To(Equal([]string{
						"-r",
						fmt.Sprintf("%s%s", resticRepositoryURL, pvc.GetUID()),
						"init",
					}))
					Expect(podSpec.InitContainers[0].Env).To(ContainElement(corev1.EnvVar{
						Name:  "RESTIC_PASSWORD",
						Value: resticPassword,
					}))
					Expect(podSpec.InitContainers[0].Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
					Expect(podSpec.InitContainers[0].Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("100M")))
				})

				It("should populate the containers", func() {
					Expect(podSpec.Containers).To(HaveLen(1))
					Expect(podSpec.Containers[0].Image).To(Equal(DefaultResticImage))
					Expect(podSpec.Containers[0].Args).To(Equal([]string{
						"-r",
						fmt.Sprintf("%s%s", resticRepositoryURL, pvc.GetUID()),
						"backup", ".",
						"--host", "liqo",
					}))
					Expect(podSpec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
						Name:  "RESTIC_PASSWORD",
						Value: resticPassword,
					}))
					Expect(podSpec.InitContainers[0].Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
					Expect(podSpec.InitContainers[0].Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("100M")))
					Expect(podSpec.Containers[0].WorkingDir).To(Equal("/backup"))
					Expect(podSpec.Containers[0].VolumeMounts).To(HaveLen(1))
					Expect(podSpec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
						Name:      "backup",
						MountPath: "/backup",
					}))
				})

				It("should populate the volumes", func() {
					Expect(podSpec.Volumes).To(HaveLen(1))
					Expect(podSpec.Volumes).To(ContainElement(corev1.Volume{
						Name: "backup",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvc.GetName(),
							},
						},
					}))
				})
			})

		})

		Context("createRestorerJob", func() {

			var (
				o    Options
				cl   client.Client
				oPvc *corev1.PersistentVolumeClaim
				nPvc *corev1.PersistentVolumeClaim
			)

			BeforeEach(func() {
				oPvc = newPvc("pvc1")
				nPvc = newPvc("pvc2")

				cl = fake.NewClientBuilder().WithObjects(oPvc, nPvc).Build()
				o = Options{Factory: &factory.Factory{CRClient: cl}, ResticPassword: resticPassword, TargetNode: "node1",
					ContainersCPURequests: resource.MustParse("100m"), ContainersRAMLimits: resource.MustParse("100M"),
					ResticImage: DefaultResticImage, ResticServerImage: DefaultResticServerImage}
			})

			When("creates a restorer job", func() {

				var (
					job     *batchv1.Job
					err     error
					podSpec *corev1.PodSpec
				)

				BeforeEach(func() {
					job, err = o.createRestorerJob(ctx, oPvc, nPvc, resticRepositoryURL)
					Expect(err).ToNot(HaveOccurred())
					Expect(job).ToNot(BeNil())

					podSpec = &job.Spec.Template.Spec
				})

				It("should create a job with the correct namespace", func() {
					Expect(job.Namespace).To(Equal(nPvc.Namespace))
				})

				It("should create pod that restart on failures", func() {
					Expect(podSpec.RestartPolicy).To(Equal(corev1.RestartPolicyOnFailure))
				})

				It("should populate the containers", func() {
					Expect(podSpec.Containers).To(HaveLen(1))
					Expect(podSpec.Containers[0].Image).To(Equal(DefaultResticImage))
					Expect(podSpec.Containers[0].Args).To(Equal([]string{
						"-r",
						fmt.Sprintf("%s%s", resticRepositoryURL, oPvc.GetUID()),
						"restore", "latest",
						"--target", "/restore",
					}))
					Expect(podSpec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
						Name:  "RESTIC_PASSWORD",
						Value: resticPassword,
					}))
					Expect(podSpec.Containers[0].Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
					Expect(podSpec.Containers[0].Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("100M")))
					Expect(podSpec.Containers[0].VolumeMounts).To(HaveLen(1))
					Expect(podSpec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
						Name:      "restore",
						MountPath: "/restore",
					}))
				})

				It("should bind the pod to the target node", func() {
					Expect(podSpec.Affinity).To(PointTo(Equal(corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/hostname",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{o.TargetNode},
											},
										},
									},
								},
							},
						},
					})))
				})

				It("should populate the volumes", func() {
					Expect(podSpec.Volumes).To(HaveLen(1))
					Expect(podSpec.Volumes).To(ContainElement(corev1.Volume{
						Name: "restore",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: nPvc.GetName(),
							},
						},
					}))
				})

			})

		})

	})

	Context("ensure restic repository", func() {

		var (
			o  Options
			cl client.Client
		)

		Context("setup", func() {

			var (
				targetPvc *corev1.PersistentVolumeClaim
			)

			BeforeEach(func() {
				cl = fake.NewClientBuilder().Build()
				o = Options{Factory: &factory.Factory{CRClient: cl},
					ResticServerImage: DefaultResticServerImage, ResticPassword: resticPassword}

				targetPvc = newPvc("pvc1")
				targetPvc.Spec.Resources = corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				}
			})

			It("creates a restic repository", func() {
				Expect(o.ensureResticRepository(ctx, targetPvc)).To(Succeed())

				var repo appsv1.StatefulSet
				Expect(cl.Get(ctx, types.NamespacedName{Name: resticRegistry, Namespace: liqoStorageNamespace}, &repo)).To(Succeed())

				var svc corev1.Service
				Expect(cl.Get(ctx, types.NamespacedName{Name: resticRegistry, Namespace: liqoStorageNamespace}, &svc)).To(Succeed())
			})

		})

		Context("teardown", func() {

			type deleteResticRepositoryTestcase struct {
				client client.Client
			}

			DescribeTable("delete restic repository", func(c deleteResticRepositoryTestcase) {
				Expect(deleteResticRepository(ctx, c.client)).To(Succeed())

				var svc corev1.Service
				Eventually(func() error {
					return c.client.Get(ctx, types.NamespacedName{Name: resticRegistry, Namespace: liqoStorageNamespace}, &svc)
				}).Should(BeNotFound())

				var statefulSet appsv1.StatefulSet
				Eventually(func() error {
					return c.client.Get(ctx, types.NamespacedName{Name: resticRegistry, Namespace: liqoStorageNamespace}, &statefulSet)
				}).Should(BeNotFound())
			}, Entry("no service and no statefulset", deleteResticRepositoryTestcase{
				client: fake.NewClientBuilder().Build(),
			}), Entry("service and no statefulset", deleteResticRepositoryTestcase{
				client: fake.NewClientBuilder().WithObjects(newService(resticRegistry, liqoStorageNamespace)).Build(),
			}), Entry("no service and statefulset", deleteResticRepositoryTestcase{
				client: fake.NewClientBuilder().WithObjects(newStatefulSet(resticRegistry, liqoStorageNamespace)).Build(),
			}), Entry("service and statefulset", deleteResticRepositoryTestcase{
				client: fake.NewClientBuilder().WithObjects(newService(resticRegistry, liqoStorageNamespace),
					newStatefulSet(resticRegistry, liqoStorageNamespace)).Build(),
			}))

		})

	})

	Context("manage NamespaceOffloading", func() {

		var (
			cl     client.Client
			scheme *runtime.Scheme
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(offloadingv1beta1.AddToScheme(scheme)).To(Succeed())

			cl = fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&offloadingv1beta1.NamespaceOffloading{}).
				Build()
		})

		Context("setup", func() {

			var (
				originNodeName      string
				targetNodeName      string
				otherRemoteNodeName string
				originNode          *corev1.Node
				targetNode          *corev1.Node
				otherRemoteNode     *corev1.Node
			)

			BeforeEach(func() {
				originNodeName = "origin-node"
				targetNodeName = "target-node"
				otherRemoteNodeName = "other-remote-node"

				originNode = newNode(originNodeName, true)
				targetNode = newNode(targetNodeName, false)
				otherRemoteNode = newNode(otherRemoteNodeName, false)
			})

			JustBeforeEach(func() {
				Expect(cl.Create(ctx, originNode)).To(Succeed())
				Expect(cl.Create(ctx, targetNode)).To(Succeed())
				Expect(cl.Create(ctx, otherRemoteNode)).To(Succeed())
			})

			It("creates the NamespaceOffloading resource", func() {
				Expect(offloadLiqoStorageNamespace(ctx, cl, originNode, targetNode)).To(Succeed())

				var nsOffload offloadingv1beta1.NamespaceOffloading
				Expect(cl.Get(ctx, types.NamespacedName{
					Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: liqoStorageNamespace}, &nsOffload)).To(Succeed())

				Expect(nsOffload.Spec.NamespaceMappingStrategy).To(Equal(offloadingv1beta1.DefaultNameMappingStrategyType))
				Expect(nsOffload.Spec.PodOffloadingStrategy).To(Equal(offloadingv1beta1.LocalPodOffloadingStrategyType))
				Expect(nsOffload.Spec.ClusterSelector).To(Equal(corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{targetNodeName},
								},
							},
						},
					},
				}))
			})

			It("does not overwrite existing NamespaceOffloading resource", func() {
				Expect(offloadLiqoStorageNamespace(ctx, cl, originNode, targetNode)).To(Succeed())

				var nsOffload offloadingv1beta1.NamespaceOffloading
				Expect(cl.Get(ctx, types.NamespacedName{
					Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: liqoStorageNamespace}, &nsOffload)).To(Succeed())

				nsOffload.Spec.NamespaceMappingStrategy = offloadingv1beta1.EnforceSameNameMappingStrategyType
				Expect(cl.Update(ctx, &nsOffload)).To(Succeed())

				Expect(offloadLiqoStorageNamespace(ctx, cl, originNode, targetNode)).To(Succeed())

				Expect(cl.Get(ctx, types.NamespacedName{
					Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: liqoStorageNamespace}, &nsOffload)).To(Succeed())
				Expect(nsOffload.Spec.NamespaceMappingStrategy).To(Equal(offloadingv1beta1.EnforceSameNameMappingStrategyType))
			})

		})

		Context("teardown", func() {

			JustBeforeEach(func() {
				namespaceOffloading := &offloadingv1beta1.NamespaceOffloading{
					ObjectMeta: metav1.ObjectMeta{
						Name:      liqoconst.DefaultNamespaceOffloadingName,
						Namespace: liqoStorageNamespace,
					},
				}
				Expect(cl.Create(ctx, namespaceOffloading)).To(Succeed())
			})

			It("deletes the NamespaceOffloading resource", func() {
				By("deleting it once")
				Expect(repatriateLiqoStorageNamespace(ctx, cl)).To(Succeed())

				var nsOffload offloadingv1beta1.NamespaceOffloading
				Eventually(func() error {
					return cl.Get(ctx, types.NamespacedName{
						Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: liqoStorageNamespace}, &nsOffload)
				}).Should(HaveOccurred())

				By("deleting it twice")
				Expect(repatriateLiqoStorageNamespace(ctx, cl)).To(Succeed())
				Eventually(func() error {
					return cl.Get(ctx, types.NamespacedName{
						Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: liqoStorageNamespace}, &nsOffload)
				}).Should(HaveOccurred())
			})

		})

		Context("get remote namespace name", func() {

			const remoteNamespaceName = "remote-ns"

			type getRemoteStorageNamespaceNameTestcase struct {
				nsOffloading  *offloadingv1beta1.NamespaceOffloading
				backoff       *wait.Backoff
				expected      string
				expectedError OmegaMatcher
			}

			DescribeTable("getRemoteStorageNamespaceName function", func(c getRemoteStorageNamespaceNameTestcase) {
				status := c.nsOffloading.Status.DeepCopy()
				Expect(cl.Create(ctx, c.nsOffloading)).To(Succeed())
				c.nsOffloading.Status = *status
				Expect(cl.Status().Update(ctx, c.nsOffloading)).To(Succeed())

				nsName, err := getRemoteStorageNamespaceName(ctx, cl, c.backoff)
				Expect(err).To(c.expectedError)
				Expect(nsName).To(Equal(c.expected))
			}, Entry("namespace offloading not ready", getRemoteStorageNamespaceNameTestcase{
				nsOffloading: &offloadingv1beta1.NamespaceOffloading{
					ObjectMeta: metav1.ObjectMeta{
						Name:      liqoconst.DefaultNamespaceOffloadingName,
						Namespace: liqoStorageNamespace,
					},
					Status: offloadingv1beta1.NamespaceOffloadingStatus{
						OffloadingPhase: offloadingv1beta1.InProgressOffloadingPhaseType,
					},
				},
				backoff:       &retry.DefaultBackoff,
				expected:      "",
				expectedError: HaveOccurred(),
			}), Entry("namespace offloading with empty namespace name", getRemoteStorageNamespaceNameTestcase{
				nsOffloading: &offloadingv1beta1.NamespaceOffloading{
					ObjectMeta: metav1.ObjectMeta{
						Name:      liqoconst.DefaultNamespaceOffloadingName,
						Namespace: liqoStorageNamespace,
					},
					Status: offloadingv1beta1.NamespaceOffloadingStatus{
						OffloadingPhase:     offloadingv1beta1.ReadyOffloadingPhaseType,
						RemoteNamespaceName: "",
					},
				},
				backoff:       &retry.DefaultBackoff,
				expected:      "",
				expectedError: HaveOccurred(),
			}), Entry("ready namespace offloading", getRemoteStorageNamespaceNameTestcase{
				nsOffloading: &offloadingv1beta1.NamespaceOffloading{
					ObjectMeta: metav1.ObjectMeta{
						Name:      liqoconst.DefaultNamespaceOffloadingName,
						Namespace: liqoStorageNamespace,
					},
					Status: offloadingv1beta1.NamespaceOffloadingStatus{
						OffloadingPhase:     offloadingv1beta1.ReadyOffloadingPhaseType,
						RemoteNamespaceName: remoteNamespaceName,
					},
				},
				backoff:       nil,
				expected:      remoteNamespaceName,
				expectedError: Not(HaveOccurred()),
			}))

		})

	})

})

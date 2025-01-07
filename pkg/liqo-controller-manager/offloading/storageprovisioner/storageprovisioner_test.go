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

package storageprovisioner

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

const (
	remoteClusterID = "foreign-cluster-id"
)

var _ = Describe("Test Storage Provisioner", func() {

	var (
		ctx    context.Context
		cancel context.CancelFunc

		k8sClient client.Client
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		k8sClient = ctrlfake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	})

	AfterEach(func() { cancel() })

	Context("Provision function", func() {

		const (
			virtualStorageClassName = "liqo"
			storageNamespace        = "liqo-storage"
		)

		var (
			forgeNode = func(name string, isVirtual bool) *corev1.Node {
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: map[string]string{liqoconst.RemoteClusterID: remoteClusterID},
					},
				}

				if isVirtual {
					node.ObjectMeta.Labels[liqoconst.TypeLabel] = liqoconst.TypeNode
				}

				return node
			}

			forgePVC = func(name, namespace string) *corev1.PersistentVolumeClaim {
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						UID:       "uuid",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: *resource.NewQuantity(10, resource.BinarySI),
							},
						},
					},
				}

				return pvc
			}
		)

		type provisionTestcase struct {
			options                   controller.ProvisionOptions
			localRealStorageClassName string
			expectedState             types.GomegaMatcher
			expectedError             types.GomegaMatcher
		}

		DescribeTable("Provision table",
			func(c provisionTestcase) {
				Expect(k8sClient.Create(ctx, c.options.SelectedNode)).To(Succeed())
				defer Expect(k8sClient.Delete(ctx, c.options.SelectedNode)).To(Succeed())

				provisioner, err := NewLiqoLocalStorageProvisioner(ctx, k8sClient, virtualStorageClassName, storageNamespace, c.localRealStorageClassName)
				Expect(err).ToNot(HaveOccurred())
				Expect(provisioner).NotTo(BeNil())

				_, state, err := provisioner.Provision(ctx, c.options)
				Expect(err).To(c.expectedError)
				Expect(state).To(c.expectedState)
			},

			Entry("virtual node", provisionTestcase{
				options: controller.ProvisionOptions{
					SelectedNode: forgeNode("test", true),
				},
				expectedError: MatchError(&controller.IgnoredError{
					Reason: "the local storage provider is not providing storage for remote nodes",
				}),
				expectedState: Equal(controller.ProvisioningFinished),
			}),

			Entry("local node", provisionTestcase{
				options: controller.ProvisionOptions{
					SelectedNode: forgeNode("test", false),
					PVC:          forgePVC("test", "default"),
				},
				expectedError: MatchError("provisioning real PVC"),
				expectedState: Equal(controller.ProvisioningInBackground),
			}),
		)

		type provisionRealTestcase struct {
			pvc                       *corev1.PersistentVolumeClaim
			node                      *corev1.Node
			localRealStorageClassName string
			pvName                    string
			realPvName                string
		}

		DescribeTable("provision a real PVC",
			func(c provisionRealTestcase) {
				forgeOpts := func() controller.ProvisionOptions {
					return controller.ProvisionOptions{
						SelectedNode: c.node,
						PVC:          c.pvc,
						PVName:       c.pvName,
						StorageClass: &storagev1.StorageClass{
							ObjectMeta: metav1.ObjectMeta{
								Name: virtualStorageClassName,
							},
							ReclaimPolicy: func() *corev1.PersistentVolumeReclaimPolicy {
								policy := corev1.PersistentVolumeReclaimDelete
								return &policy
							}(),
						},
					}
				}

				Expect(k8sClient.Create(ctx, c.node)).To(Succeed())

				genericProvisioner, err := NewLiqoLocalStorageProvisioner(ctx, k8sClient,
					virtualStorageClassName, storageNamespace, c.localRealStorageClassName)
				Expect(err).ToNot(HaveOccurred())
				Expect(genericProvisioner).ToNot(BeNil())
				provisioner := genericProvisioner.(*liqoLocalStorageProvisioner)
				Expect(provisioner).ToNot(BeNil())

				By("first creation")
				pv, state, err := provisioner.provisionLocalPVC(ctx, forgeOpts())
				Expect(err).To(MatchError("provisioning real PVC"))
				Expect(state).To(Equal(controller.ProvisioningInBackground))
				Expect(pv).To(BeNil())

				var realPvc corev1.PersistentVolumeClaim
				Expect(k8sClient.Get(ctx, apitypes.NamespacedName{
					Name:      string(c.pvc.GetUID()),
					Namespace: storageNamespace,
				}, &realPvc)).To(Succeed())

				if c.localRealStorageClassName != "" {
					Expect(realPvc.Spec.StorageClassName).To(PointTo(Equal(c.localRealStorageClassName)))
				} else {
					Expect(realPvc.Spec.StorageClassName).To(BeNil())
				}
				Expect(realPvc.Labels).To(HaveKey(liqoconst.VirtualPvcNamespaceLabel))
				Expect(realPvc.Labels).To(HaveKey(liqoconst.VirtualPvcNameLabel))
				Expect(realPvc.Labels[liqoconst.VirtualPvcNamespaceLabel]).To(Equal(c.pvc.GetNamespace()))
				Expect(realPvc.Labels[liqoconst.VirtualPvcNameLabel]).To(Equal(c.pvc.GetName()))

				By("second attempt with no real pvc provisioned")
				pv, state, err = provisioner.provisionLocalPVC(ctx, forgeOpts())
				Expect(err).To(MatchError("real PV not provided yet"))
				Expect(state).To(Equal(controller.ProvisioningInBackground))
				Expect(pv).To(BeNil())

				By("second attempt with real pvc provisioned")
				realPv := &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: c.realPvName,
					},
					Spec: corev1.PersistentVolumeSpec{
						Capacity: corev1.ResourceList{
							corev1.ResourceStorage: *resource.NewQuantity(10, resource.BinarySI),
						},
						PersistentVolumeSource: corev1.PersistentVolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/test",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, realPv)).To(Succeed())

				realPvc.Spec.VolumeName = c.realPvName
				Expect(k8sClient.Update(ctx, &realPvc)).To(Succeed())
				pv, state, err = provisioner.provisionLocalPVC(ctx, forgeOpts())
				Expect(err).ToNot(HaveOccurred())
				Expect(state).To(Equal(controller.ProvisioningFinished))
				Expect(pv).ToNot(BeNil())
				Expect(pv.Spec.Capacity).To(Equal(realPv.Spec.Capacity))
				Expect(pv.Spec.PersistentVolumeSource).To(Equal(realPv.Spec.PersistentVolumeSource))
				Expect(pv.Spec.StorageClassName).To(Equal(virtualStorageClassName))
			},

			Entry("empty storage class", provisionRealTestcase{
				pvc:                       forgePVC("test-real", "default"),
				node:                      forgeNode("test", false),
				localRealStorageClassName: "",
				pvName:                    "pv-name",
				realPvName:                "real-pv-name",
			}),

			Entry("defined storage class", provisionRealTestcase{
				pvc:                       forgePVC("test-real-2", "default"),
				node:                      forgeNode("test-2", false),
				localRealStorageClassName: "other-class",
				pvName:                    "pv-name-2",
				realPvName:                "real-pv-name-2",
			}),
		)

		Context("ProvisionRemotePVC", func() {

			const (
				LocalNamespace  = "local-namespace"
				RemoteNamespace = "remote-namespace"

				virtualNodeName      = "virtual-node"
				pvcName              = "test-remote"
				pvName               = "pv-name"
				realStorageClassName = "other-class"
			)

			var (
				remotePersistentVolumeClaims        corev1listers.PersistentVolumeClaimNamespaceLister
				remotePersistentVolumesClaimsClient corev1clients.PersistentVolumeClaimInterface
				forgingOpts                         *forge.ForgingOpts
			)

			BeforeEach(func() {
				_, err := testEnvClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: RemoteNamespace,
					},
				}, metav1.CreateOptions{})
				if err != nil && !apierrors.IsAlreadyExists(err) {
					Expect(err).ToNot(HaveOccurred())
				}

				forgingOpts = testutil.FakeForgingOpts()

				factory := informers.NewSharedInformerFactory(testEnvClient, 10*time.Hour)
				remote := factory.Core().V1().PersistentVolumeClaims()

				remotePersistentVolumeClaims = remote.Lister().PersistentVolumeClaims(RemoteNamespace)
				remotePersistentVolumesClaimsClient = testEnvClient.CoreV1().PersistentVolumeClaims(RemoteNamespace)

				factory.Start(ctx.Done())
			})

			AfterEach(func() {
				err := testEnvClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
				Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
			})

			It("provision remote pvc", func() {
				tester := func(storageClass string) error {
					pv, state, err := ProvisionRemotePVC(ctx, controller.ProvisionOptions{
						SelectedNode: &corev1.Node{
							ObjectMeta: metav1.ObjectMeta{
								Name:   virtualNodeName,
								Labels: map[string]string{liqoconst.RemoteClusterID: remoteClusterID},
							},
						},
						PVC: &corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      pvcName,
								Namespace: LocalNamespace,
								UID:       "uuid",
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								StorageClassName: pointer.String(virtualStorageClassName),
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: *resource.NewQuantity(10, resource.BinarySI),
									},
								},
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							},
						},
						PVName: pvName,
						StorageClass: &storagev1.StorageClass{
							ObjectMeta: metav1.ObjectMeta{
								Name: virtualStorageClassName,
							},
							ReclaimPolicy: func() *corev1.PersistentVolumeReclaimPolicy {
								policy := corev1.PersistentVolumeReclaimDelete
								return &policy
							}(),
						},
					}, RemoteNamespace, storageClass, remotePersistentVolumeClaims, remotePersistentVolumesClaimsClient, forgingOpts)

					if err != nil {
						return err
					}
					Expect(state).To(Equal(controller.ProvisioningFinished))
					Expect(pv).ToNot(BeNil())
					Expect(pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal(liqoconst.RemoteClusterID))
					Expect(pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Operator).To(Equal(corev1.NodeSelectorOpIn))
					Expect(pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values).To(ContainElement(remoteClusterID))
					Expect(pv.Spec.StorageClassName).To(Equal(virtualStorageClassName))

					_, err = testEnvClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).Get(ctx, pvcName, metav1.GetOptions{})
					return err
				}

				By("the remote real PVC does not exists")
				Eventually(func() error {
					return tester(realStorageClassName)
				}).Should(Succeed())

				By("the remote real PVC already exists, check idempotency")
				Eventually(func() error {
					return tester(realStorageClassName)
				}).Should(Succeed())

				By("using different storage class, it should not modify already existent volumes")
				Eventually(func() error {
					return tester("other-class-2")
				}).Should(Succeed())

				realPvc, err := testEnvClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).Get(ctx, pvcName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(realPvc.Spec.StorageClassName).To(PointTo(Equal(realStorageClassName)))
				Expect(realPvc.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
				Expect(realPvc.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
			})

		})

	})

	Context("utility functions", func() {

		Context("mergeAffinities", func() {

			var getAffinity = func(key, val string) *corev1.VolumeNodeAffinity {
				return &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      key,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{val},
									},
								},
							},
						},
					},
				}
			}

			type mergeAffinitiesTestcase struct {
				pv1      *corev1.PersistentVolumeSpec
				pv2      *corev1.PersistentVolumeSpec
				expected corev1.VolumeNodeAffinity
			}

			DescribeTable("merge affinities table", func(c mergeAffinitiesTestcase) {
				affinity := mergeAffinities(c.pv1, c.pv2)
				Expect(affinity).To(PointTo(Equal(c.expected)))
			}, Entry("no affinity on pv2", mergeAffinitiesTestcase{
				pv1: &corev1.PersistentVolumeSpec{
					NodeAffinity: getAffinity("foo", "bar"),
				},
				pv2: &corev1.PersistentVolumeSpec{
					NodeAffinity: nil,
				},
				expected: *getAffinity("foo", "bar"),
			}), Entry("no affinity on pv1", mergeAffinitiesTestcase{
				pv1: &corev1.PersistentVolumeSpec{
					NodeAffinity: nil,
				},
				pv2: &corev1.PersistentVolumeSpec{
					NodeAffinity: getAffinity("foo", "bar"),
				},
				expected: *getAffinity("foo", "bar"),
			}), Entry("affinity on pv1 and pv2", mergeAffinitiesTestcase{
				pv1: &corev1.PersistentVolumeSpec{
					NodeAffinity: getAffinity("foo", "bar"),
				},
				pv2: &corev1.PersistentVolumeSpec{
					NodeAffinity: getAffinity("foo2", "bar2"),
				},
				expected: corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "foo",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"bar"},
									},
									{
										Key:      "foo2",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"bar2"},
									},
								},
							},
						},
					},
				},
			}))

		})

	})

})

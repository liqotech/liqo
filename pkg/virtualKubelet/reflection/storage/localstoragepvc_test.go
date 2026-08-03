// Copyright 2019-2026 The Liqo Authors
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

package storage

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	localStoragePVCName = "local-storage-pvc"
	localPVName         = "local-storage-pv"
)

var _ = Describe("local storage PVC reflector helpers", func() {

	Context("isLocalStoragePV", func() {
		type isLocalStorageTestcase struct {
			pv       *corev1.PersistentVolume
			expected OmegaMatcher
		}

		DescribeTable("isLocalStoragePV table",
			func(c isLocalStorageTestcase) {
				Expect(isLocalStoragePV(c.pv)).To(c.expected)
			},
			Entry("local volume source", isLocalStorageTestcase{
				pv: &corev1.PersistentVolume{
					Spec: corev1.PersistentVolumeSpec{
						PersistentVolumeSource: corev1.PersistentVolumeSource{
							Local: &corev1.LocalVolumeSource{Path: "/mnt/disks/vol1"},
						},
					},
				},
				expected: BeTrue(),
			}),
			Entry("host path volume source", isLocalStorageTestcase{
				pv: &corev1.PersistentVolume{
					Spec: corev1.PersistentVolumeSpec{
						PersistentVolumeSource: corev1.PersistentVolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/tmp"},
						},
					},
				},
				expected: BeFalse(),
			}),
			Entry("nil volume source", isLocalStorageTestcase{
				pv: &corev1.PersistentVolume{
					Spec: corev1.PersistentVolumeSpec{},
				},
				expected: BeFalse(),
			}),
		)
	})

	Context("sanitizeNodeAffinity", func() {
		It("should return a dummy selector when affinity is nil", func() {
			res := sanitizeNodeAffinity(nil)
			Expect(res).ToNot(BeNil())
			Expect(res.Required).ToNot(BeNil())
			Expect(res.Required.NodeSelectorTerms).To(HaveLen(1))
			Expect(res.Required.NodeSelectorTerms[0].MatchExpressions).To(HaveLen(1))
			Expect(*res.Required.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal("kubernetes.io/os"))
			Expect(*res.Required.NodeSelectorTerms[0].MatchExpressions[0].Operator).
				To(Equal(corev1.NodeSelectorOpExists))
		})

		It("should strip virtual-node selectors and preserve real-node selectors", func() {
			affinity := &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: consts.TypeLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{consts.TypeNode}},
								{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"real-node"}},
							},
						},
					},
				},
			}
			res := sanitizeNodeAffinity(affinity)
			Expect(res.Required.NodeSelectorTerms).To(HaveLen(1))
			Expect(res.Required.NodeSelectorTerms[0].MatchExpressions).To(HaveLen(1))
			Expect(*res.Required.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal("kubernetes.io/hostname"))
		})

		It("should add a dummy selector when all terms are stripped", func() {
			affinity := &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: consts.TypeLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{consts.TypeNode}},
							},
						},
					},
				},
			}
			res := sanitizeNodeAffinity(affinity)
			Expect(res.Required.NodeSelectorTerms).To(HaveLen(1))
			Expect(*res.Required.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal("kubernetes.io/os"))
		})
	})

	Context("filterPVCAnnotations", func() {
		It("should strip cluster-specific annotations", func() {
			annotations := map[string]string{
				"pv.kubernetes.io/bind-completed":          "yes",
				"volume.kubernetes.io/selected-node":       "node-1",
				"volume.kubernetes.io/storage-provisioner": "local-volume-provisioner",
				"keep-this": "value",
			}
			res := filterPVCAnnotations(annotations)
			Expect(res).ToNot(HaveKey("pv.kubernetes.io/bind-completed"))
			Expect(res).ToNot(HaveKey("volume.kubernetes.io/selected-node"))
			Expect(res).ToNot(HaveKey("volume.kubernetes.io/storage-provisioner"))
			Expect(res).To(HaveKeyWithValue("keep-this", "value"))
		})
	})

	Context("remotePVNameFromLocal", func() {
		It("should generate a deterministic name from the local PV UID", func() {
			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					UID: "abc-123",
				},
			}
			Expect(remotePVNameFromLocal(pv)).To(Equal("liqo-local-abc-123"))
		})
	})
})

var _ = Describe("local storage PVC reflector integration", func() {
	var lsReflector *NamespacedLocalStoragePVCReflector

	BeforeEach(func() {
		lsReflectorBuilder := NewNamespacedLocalStoragePVCReflector()
		opts := options.NewNamespaced().
			WithLocal(LocalNamespace, k8sClient, factory).
			WithRemote(RemoteNamespace, k8sClient, factory).
			WithHandlerFactory(FakeEventHandler).
			WithEventBroadcaster(record.NewBroadcaster()).
			WithForgingOpts(testutil.FakeForgingOpts())

		lsReflector = lsReflectorBuilder(opts).(*NamespacedLocalStoragePVCReflector)
		Expect(lsReflector).ToNot(BeNil())
	})

	It("should reflect a bound local-storage PVC and its PV", func() {
		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: localPVName,
				Annotations: map[string]string{
					"pv.kubernetes.io/provisioned-by": "local-volume-provisioner",
				},
			},
			Spec: corev1.PersistentVolumeSpec{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					Local: &corev1.LocalVolumeSource{
						Path: "/mnt/disks/vol1",
					},
				},
				StorageClassName: "local-storage",
				NodeAffinity: &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{Key: consts.TypeLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{consts.TypeNode}},
								},
							},
						},
					},
				},
			},
		}

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      localStoragePVCName,
				Namespace: LocalNamespace,
				Labels: map[string]string{
					testutil.FakeNotReflectedLabelKey: "true",
				},
				Annotations: map[string]string{
					"volume.kubernetes.io/selected-node":       "local-node",
					"volume.kubernetes.io/storage-provisioner": "local-volume-provisioner",
					testutil.FakeNotReflectedAnnotKey:          "true",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.To("local-storage"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				VolumeName: localPVName,
			},
		}

		pv, err := k8sClient.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		_, err = k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).Create(ctx, pvc, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		factory.Start(ctx.Done())
		factory.WaitForCacheSync(ctx.Done())

		Expect(lsReflector.Handle(ctx, localStoragePVCName)).To(Succeed())

		expectedRemotePVName := remotePVNameFromLocal(pv)

		// Verify the remote PVC.
		remotePVC, err := k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).
			Get(ctx, localStoragePVCName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(remotePVC.Spec.StorageClassName).To(HaveValue(Equal("local-storage")))
		Expect(remotePVC.Spec.VolumeName).To(Equal(expectedRemotePVName))
		Expect(remotePVC.Labels).To(HaveKey(forge.LiqoOriginClusterIDKey))
		Expect(remotePVC.Annotations).ToNot(HaveKey("volume.kubernetes.io/selected-node"))
		Expect(remotePVC.Annotations).ToNot(HaveKey("volume.kubernetes.io/storage-provisioner"))

		// Verify the remote PV.
		remotePV, err := k8sClient.CoreV1().PersistentVolumes().Get(ctx, expectedRemotePVName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(remotePV.Spec.StorageClassName).To(Equal("local-storage"))
		Expect(remotePV.Spec.Local.Path).To(Equal("/mnt/disks/vol1"))
		Expect(remotePV.Spec.ClaimRef.Namespace).To(Equal(RemoteNamespace))
		Expect(remotePV.Spec.ClaimRef.Name).To(Equal(localStoragePVCName))
		Expect(remotePV.Annotations).ToNot(HaveKey("pv.kubernetes.io/provisioned-by"))
		Expect(remotePV.Spec.NodeAffinity.Required.NodeSelectorTerms).To(HaveLen(1))
		Expect(remotePV.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions).To(HaveLen(1))
		Expect(remotePV.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal("kubernetes.io/os"))
		Expect(remotePV.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Operator).
			To(Equal(corev1.NodeSelectorOpExists))
	})

	It("should not reflect a local-storage PVC that is not bound", func() {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unbound-local-storage-pvc",
				Namespace: LocalNamespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.To("local-storage"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}

		_, err := k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).Create(ctx, pvc, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		factory.Start(ctx.Done())
		factory.WaitForCacheSync(ctx.Done())

		Expect(lsReflector.Handle(ctx, "unbound-local-storage-pvc")).To(Succeed())

		_, err = k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).
			Get(ctx, "unbound-local-storage-pvc", metav1.GetOptions{})
		Expect(err).To(HaveOccurred())
	})

	It("should delete the remote PVC and PV when the local PVC is deleted", func() {
		deleteTestPVName := "delete-test-local-pv"
		deleteTestPVCName := "delete-test-local-storage-pvc"

		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: deleteTestPVName,
			},
			Spec: corev1.PersistentVolumeSpec{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					Local: &corev1.LocalVolumeSource{Path: "/mnt/disks/vol1"},
				},
				StorageClassName: "local-storage",
				NodeAffinity: &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"real-node"}},
							}},
						},
					},
				},
			},
		}

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deleteTestPVCName,
				Namespace: LocalNamespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: ptr.To("local-storage"),
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
				},
				VolumeName: deleteTestPVName,
			},
		}

		pv, err := k8sClient.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		_, err = k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).Create(ctx, pvc, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		factory.Start(ctx.Done())
		factory.WaitForCacheSync(ctx.Done())

		Expect(lsReflector.Handle(ctx, deleteTestPVCName)).To(Succeed())

		remotePVName := remotePVNameFromLocal(pv)
		_, err = k8sClient.CoreV1().PersistentVolumes().Get(ctx, remotePVName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		// Delete the local PVC via the API. In envtest, the PVC protection
		// finalizer is not automatically removed, so we clear it to let the
		// deletion complete. Then create a fresh reflector so that its informer
		// lists the current state (PVC missing) and triggers cleanup of the
		// remote objects.
		Expect(k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).
			Delete(ctx, deleteTestPVCName, metav1.DeleteOptions{})).To(Succeed())
		deletedPVC, err := k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).
			Get(ctx, deleteTestPVCName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		deletedPVC.Finalizers = nil
		_, err = k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).
			Update(ctx, deletedPVC, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		freshFactory := informers.NewSharedInformerFactory(k8sClient, 10*time.Hour)
		freshOpts := options.NewNamespaced().
			WithLocal(LocalNamespace, k8sClient, freshFactory).
			WithRemote(RemoteNamespace, k8sClient, freshFactory).
			WithHandlerFactory(FakeEventHandler).
			WithEventBroadcaster(record.NewBroadcaster()).
			WithForgingOpts(testutil.FakeForgingOpts())
		freshReflector := NewNamespacedLocalStoragePVCReflector()(freshOpts).(*NamespacedLocalStoragePVCReflector)
		freshFactory.Start(ctx.Done())
		freshFactory.WaitForCacheSync(ctx.Done())

		Expect(freshReflector.Handle(ctx, deleteTestPVCName)).To(Succeed())

		// In envtest, protection finalizers can delay deletion. If the objects
		// are still present with a deletion timestamp, clear their finalizers to
		// let the deletion complete, then assert they are gone.
		remotePVC, err := k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).
			Get(ctx, deleteTestPVCName, metav1.GetOptions{})
		if err == nil && remotePVC.DeletionTimestamp != nil {
			remotePVC.Finalizers = nil
			_, _ = k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).
				Update(ctx, remotePVC, metav1.UpdateOptions{})
		}
		remotePV, err := k8sClient.CoreV1().PersistentVolumes().Get(ctx, remotePVName, metav1.GetOptions{})
		if err == nil && remotePV.DeletionTimestamp != nil {
			remotePV.Finalizers = nil
			_, _ = k8sClient.CoreV1().PersistentVolumes().Update(ctx, remotePV, metav1.UpdateOptions{})
		}

		_, err = k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).
			Get(ctx, deleteTestPVCName, metav1.GetOptions{})
		Expect(err).To(HaveOccurred())
		_, err = k8sClient.CoreV1().PersistentVolumes().Get(ctx, remotePVName, metav1.GetOptions{})
		Expect(err).To(HaveOccurred())
	})
})

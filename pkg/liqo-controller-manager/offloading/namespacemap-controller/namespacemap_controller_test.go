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

package namespacemapctrl_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	namespacemapctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/namespacemap-controller"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("Enforcement logic", func() {
	var (
		ctx           context.Context
		clientBuilder fake.ClientBuilder
		reconciler    namespacemapctrl.NamespaceMapReconciler

		nm  offloadingv1beta1.NamespaceMap
		err error
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientBuilder = fake.ClientBuilder{}
		nm = offloadingv1beta1.NamespaceMap{ObjectMeta: metav1.ObjectMeta{
			Name: "name", Namespace: "tenant-namespace",
			Labels: map[string]string{liqoconst.ReplicationOriginLabel: "origin"}},
		}
	})

	JustBeforeEach(func() {
		reconciler = namespacemapctrl.NamespaceMapReconciler{Client: clientBuilder.WithObjects(&nm).
			WithStatusSubresource(&offloadingv1beta1.NamespaceMap{}).
			Build()}
		_, err = reconciler.Reconcile(ctx, controllerruntime.Request{NamespacedName: client.ObjectKeyFromObject(&nm)})
	})

	Context("namespace enforcement", func() {
		Context("finalizer enforcement", func() {
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should ensure the finalizer is present", func() {
				var updated offloadingv1beta1.NamespaceMap
				Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
				Expect(ctrlutils.ContainsFinalizer(&updated, namespacemapctrl.NamespaceMapControllerFinalizer)).To(BeTrue())
			})
		})

		Context("creation", func() {
			BeforeEach(func() {
				nm.Spec.DesiredMapping = map[string]string{"namespace": "namespace-remote"}
			})

			SuccessWhenBody := func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should correctly ensure the namespace is present", func() {
					var namespace corev1.Namespace
					Expect(reconciler.Get(ctx, types.NamespacedName{Name: "namespace-remote"}, &namespace)).To(Succeed())
					Expect(namespace.GetAnnotations()).To(HaveKeyWithValue(liqoconst.RemoteNamespaceManagedByAnnotationKey, "tenant-namespace/name"))
					Expect(namespace.GetAnnotations()).To(HaveKeyWithValue(liqoconst.RemoteNamespaceOriginalNameAnnotationKey, "namespace"))
					Expect(namespace.GetLabels()).To(HaveKeyWithValue(liqoconst.RemoteClusterID, "origin"))
				})
				It("should correctly ensure the rolebinding is present", func() {
					var binding rbacv1.RoleBinding
					Expect(reconciler.Get(ctx, types.NamespacedName{Namespace: "namespace-remote", Name: "tenant-namespace"}, &binding)).To(Succeed())
					Expect(binding.Subjects).To(ConsistOf(rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: "origin"}))
					Expect(binding.RoleRef).To(Equal(
						rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: liqoconst.RemoteNamespaceClusterRoleName}))
					Expect(binding.GetAnnotations()).To(HaveKeyWithValue(liqoconst.RemoteNamespaceManagedByAnnotationKey, "tenant-namespace/name"))
				})
				It("should correctly update the NamespaceMap status", func() {
					var updated offloadingv1beta1.NamespaceMap
					Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
					Expect(updated.Status.CurrentMapping).To(HaveKeyWithValue("namespace",
						offloadingv1beta1.RemoteNamespaceStatus{RemoteNamespace: "namespace-remote", Phase: offloadingv1beta1.MappingAccepted}))
				})
			}

			When("the namespace does not yet exist", func() {
				Describe("perform checks", func() { SuccessWhenBody() })
			})

			When("the namespace already exists and it is managed by the NamespaceMap", func() {
				BeforeEach(func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "namespace-remote",
						Annotations: map[string]string{
							liqoconst.RemoteNamespaceManagedByAnnotationKey:    "tenant-namespace/name",
							liqoconst.RemoteNamespaceOriginalNameAnnotationKey: "namespace",
						},
						Labels: map[string]string{liqoconst.RemoteClusterID: "origin"},
					}}
					clientBuilder.WithObjects(&namespace)
				})

				Describe("perform checks", func() { SuccessWhenBody() })
			})

			When("the namespace already exists but it is not managed by the NamespaceMap", func() {
				BeforeEach(func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "namespace-remote"}}
					clientBuilder.WithObjects(&namespace)
				})

				Describe("perform checks", func() {
					It("should fail", func() { Expect(err).To(HaveOccurred()) })
					It("should correctly update the NamespaceMap status", func() {
						var updated offloadingv1beta1.NamespaceMap
						Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
						Expect(updated.Status.CurrentMapping).To(HaveKeyWithValue("namespace",
							offloadingv1beta1.RemoteNamespaceStatus{RemoteNamespace: "namespace-remote", Phase: offloadingv1beta1.MappingCreationLoopBackOff}))
					})
				})
			})
		})

		Context("multiple creations", func() {
			BeforeEach(func() {
				nm.Spec.DesiredMapping = map[string]string{
					"first":  "first-remote",
					"second": "second-remote",
					"third":  "third-remote",
				}
			})

			When("all namespaces are valid", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should correctly update the NamespaceMap status", func() {
					var updated offloadingv1beta1.NamespaceMap
					Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
					for origin, remapped := range nm.Spec.DesiredMapping {
						Expect(updated.Status.CurrentMapping).To(HaveKeyWithValue(origin,
							offloadingv1beta1.RemoteNamespaceStatus{RemoteNamespace: remapped, Phase: offloadingv1beta1.MappingAccepted}))
					}
				})
			})

			When("one namespace is expected to fail", func() {
				BeforeEach(func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "second-remote"}}
					clientBuilder.WithObjects(&namespace)
				})

				It("should fail", func() { Expect(err).To(HaveOccurred()) })
				It("should correctly update the NamespaceMap status", func() {
					var updated offloadingv1beta1.NamespaceMap
					Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
					Expect(updated.Status.CurrentMapping).To(HaveKeyWithValue("first",
						offloadingv1beta1.RemoteNamespaceStatus{RemoteNamespace: "first-remote", Phase: offloadingv1beta1.MappingAccepted}))
					Expect(updated.Status.CurrentMapping).To(HaveKeyWithValue("second",
						offloadingv1beta1.RemoteNamespaceStatus{RemoteNamespace: "second-remote", Phase: offloadingv1beta1.MappingCreationLoopBackOff}))
					Expect(updated.Status.CurrentMapping).To(HaveKeyWithValue("third",
						offloadingv1beta1.RemoteNamespaceStatus{RemoteNamespace: "third-remote", Phase: offloadingv1beta1.MappingAccepted}))
				})
			})
		})

		Context("deletion", func() {
			BeforeEach(func() {
				nm.Status.CurrentMapping = map[string]offloadingv1beta1.RemoteNamespaceStatus{
					"namespace": {RemoteNamespace: "namespace-remote", Phase: offloadingv1beta1.MappingAccepted},
				}
			})

			When("the namespace does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should correctly ensure the namespace is absent", func() {
					var namespace corev1.Namespace
					Expect(reconciler.Get(ctx, types.NamespacedName{Name: "namespace-remote"}, &namespace)).To(BeNotFound())
				})
				It("should correctly update the NamespaceMap status", func() {
					var updated offloadingv1beta1.NamespaceMap
					Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
					Expect(updated.Status.CurrentMapping).ToNot(HaveKey("namespace"))
				})
			})

			When("the namespace exists and it is managed by the NamespaceMap", func() {
				BeforeEach(func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "namespace-remote",
						Annotations: map[string]string{
							liqoconst.RemoteNamespaceManagedByAnnotationKey:    "tenant-namespace/name",
							liqoconst.RemoteNamespaceOriginalNameAnnotationKey: "namespace",
						},
						Labels: map[string]string{liqoconst.RemoteClusterID: "origin"},
					}}
					clientBuilder.WithObjects(&namespace)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should correctly ensure the namespace is absent", func() {
					var namespace corev1.Namespace
					Expect(reconciler.Get(ctx, types.NamespacedName{Name: "namespace-remote"}, &namespace)).To(BeNotFound())
				})
				It("should correctly update the NamespaceMap status", func() {
					var updated offloadingv1beta1.NamespaceMap
					Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
					// The entry will be removed when the deletion event is received by the reconciler.
					Expect(updated.Status.CurrentMapping).To(HaveKeyWithValue("namespace",
						offloadingv1beta1.RemoteNamespaceStatus{RemoteNamespace: "namespace-remote", Phase: offloadingv1beta1.MappingTerminating}))
				})
			})

			When("the namespace exists but it is not managed by the NamespaceMap", func() {
				BeforeEach(func() {
					namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "namespace-remote"}}
					clientBuilder.WithObjects(&namespace)
				})

				Describe("perform checks", func() {
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should not delete the namespace", func() {
						var namespace corev1.Namespace
						Expect(reconciler.Get(ctx, types.NamespacedName{Name: "namespace-remote"}, &namespace)).To(Succeed())
					})
					It("should correctly update the NamespaceMap status", func() {
						var updated offloadingv1beta1.NamespaceMap
						Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
						Expect(updated.Status.CurrentMapping).ToNot(HaveKey("namespace"))
					})
				})
			})
		})
	})

	Context("the NamespaceMap is being terminated", func() {
		BeforeEach(func() {
			nm.SetDeletionTimestamp(&metav1.Time{Time: time.Now().Add(-10 * time.Second)})
			ctrlutils.AddFinalizer(&nm, namespacemapctrl.NamespaceMapControllerFinalizer)
			nm.Spec.DesiredMapping = map[string]string{"namespace": "namespace-remote"}
		})

		When("no current mappings are present", func() {
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should have removed the finalizer", func() {
				var updated offloadingv1beta1.NamespaceMap
				Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(BeNotFound())
			})
		})

		When("all current mappings have already terminated", func() {
			BeforeEach(func() {
				nm.Status.CurrentMapping = map[string]offloadingv1beta1.RemoteNamespaceStatus{
					"first":  {RemoteNamespace: "first-remote", Phase: offloadingv1beta1.MappingTerminating},
					"second": {RemoteNamespace: "second-remote", Phase: offloadingv1beta1.MappingTerminating},
				}
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should have removed the finalizer", func() {
				var updated offloadingv1beta1.NamespaceMap
				Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(BeNotFound())
			})
		})

		When("some namespaces should still be deleted", func() {
			BeforeEach(func() {
				nm.Status.CurrentMapping = map[string]offloadingv1beta1.RemoteNamespaceStatus{
					"first":  {RemoteNamespace: "first-remote", Phase: offloadingv1beta1.MappingTerminating},
					"second": {RemoteNamespace: "second-remote", Phase: offloadingv1beta1.MappingAccepted},
				}

				namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "second-remote",
					Annotations: map[string]string{
						liqoconst.RemoteNamespaceManagedByAnnotationKey:    "tenant-namespace/name",
						liqoconst.RemoteNamespaceOriginalNameAnnotationKey: "namespace",
					},
					Labels: map[string]string{liqoconst.RemoteClusterID: "origin"},
				}}
				clientBuilder.WithObjects(&namespace)
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should ensure the finalizer is present", func() {
				var updated offloadingv1beta1.NamespaceMap
				Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
				Expect(ctrlutils.ContainsFinalizer(&updated, namespacemapctrl.NamespaceMapControllerFinalizer)).To(BeTrue())
			})
			It("should correctly update the NamespaceMap status", func() {
				var updated offloadingv1beta1.NamespaceMap
				Expect(reconciler.Get(ctx, client.ObjectKeyFromObject(&nm), &updated)).To(Succeed())
				Expect(updated.Status.CurrentMapping).ToNot(HaveKey("first"))
				Expect(updated.Status.CurrentMapping).To(HaveKeyWithValue("second",
					offloadingv1beta1.RemoteNamespaceStatus{RemoteNamespace: "second-remote", Phase: offloadingv1beta1.MappingTerminating}))
			})
		})
	})
})

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

package controlplanesecretcontroller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqocorev1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

func TestControlPlaneSecretController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlaneSecret Controller Suite")
}

var _ = Describe("ControlPlaneSecret Controller", func() {
	const (
		testNamespace     = "liqo-tenant-test-cluster"
		testRemoteCluster = "test-cluster-id"
		testSecretName    = "test-controlplane-secret"
	)

	var (
		ctx                context.Context
		fakeClient         client.Client
		cpSecretReconciler *ControlPlaneSecretReconciler
		recorder           *events.FakeRecorder
		testScheme         *runtime.Scheme
	)

	forgeControlPlaneSecret := func(name, namespace, remoteClusterID string, annotations map[string]string) *corev1.Secret {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					consts.RemoteClusterID:      remoteClusterID,
					consts.IdentityTypeLabelKey: string(authv1beta1.ControlPlaneIdentityType),
				},
				Annotations: annotations,
			},
			Data: map[string][]byte{
				"kubeconfig": []byte("fake-kubeconfig-data"),
			},
		}
		return secret
	}

	BeforeEach(func() {
		ctx = context.Background()
		testScheme = runtime.NewScheme()

		Expect(corev1.AddToScheme(testScheme)).To(Succeed())
		Expect(offloadingv1beta1.AddToScheme(testScheme)).To(Succeed())
		Expect(authv1beta1.AddToScheme(testScheme)).To(Succeed())
		Expect(liqocorev1beta1.AddToScheme(testScheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
		recorder = events.NewFakeRecorder(100)

		cpSecretReconciler = NewControlPlaneSecretReconciler(fakeClient, testScheme, recorder)
	})

	Describe("Reconcile", func() {
		When("the secret does not exist", func() {
			It("should return without error", func() {
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "non-existent-secret",
						Namespace: testNamespace,
					},
				}

				result, err := cpSecretReconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		When("the secret has skip annotation", func() {
			It("should skip NamespaceMap creation", func() {
				annotations := map[string]string{
					consts.SkipNamespaceMapCreationAnnotationKey: consts.SkipNamespaceMapCreationAnnotationValue,
				}
				secret := forgeControlPlaneSecret(testSecretName, testNamespace, testRemoteCluster, annotations)
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testSecretName,
						Namespace: testNamespace,
					},
				}

				result, err := cpSecretReconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				nmList := &offloadingv1beta1.NamespaceMapList{}
				Expect(fakeClient.List(ctx, nmList, client.InNamespace(testNamespace))).To(Succeed())
				Expect(nmList.Items).To(HaveLen(0))
			})
		})

		When("the secret has skip annotation with different case", func() {
			It("should skip NamespaceMap creation (case insensitive)", func() {
				annotations := map[string]string{
					consts.SkipNamespaceMapCreationAnnotationKey: "TRUE",
				}
				secret := forgeControlPlaneSecret(testSecretName, testNamespace, testRemoteCluster, annotations)
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testSecretName,
						Namespace: testNamespace,
					},
				}

				result, err := cpSecretReconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				nmList := &offloadingv1beta1.NamespaceMapList{}
				Expect(fakeClient.List(ctx, nmList, client.InNamespace(testNamespace))).To(Succeed())
				Expect(nmList.Items).To(HaveLen(0))
			})
		})

		When("the secret is missing remote cluster ID label", func() {
			It("should return without creating NamespaceMap", func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecretName,
						Namespace: testNamespace,
						Labels: map[string]string{
							consts.IdentityTypeLabelKey: string(authv1beta1.ControlPlaneIdentityType),
						},
					},
				}
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testSecretName,
						Namespace: testNamespace,
					},
				}

				result, err := cpSecretReconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				nmList := &offloadingv1beta1.NamespaceMapList{}
				Expect(fakeClient.List(ctx, nmList, client.InNamespace(testNamespace))).To(Succeed())
				Expect(nmList.Items).To(HaveLen(0))
			})
		})

		When("the secret is a valid controlplane secret", func() {
			It("should create a NamespaceMap", func() {
				secret := forgeControlPlaneSecret(testSecretName, testNamespace, testRemoteCluster, nil)
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testSecretName,
						Namespace: testNamespace,
					},
				}

				result, err := cpSecretReconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				nmList := &offloadingv1beta1.NamespaceMapList{}
				Expect(fakeClient.List(ctx, nmList, client.InNamespace(testNamespace))).To(Succeed())
				Expect(nmList.Items).To(HaveLen(1))

				nm := nmList.Items[0]
				Expect(nm.Labels).To(HaveKeyWithValue(consts.RemoteClusterID, testRemoteCluster))
				Expect(nm.Labels).To(HaveKeyWithValue(consts.ReplicationRequestedLabel, "true"))
				Expect(nm.Labels).To(HaveKeyWithValue(consts.ReplicationDestinationLabel, testRemoteCluster))

				Expect(nm.OwnerReferences).To(HaveLen(1))
				Expect(nm.OwnerReferences[0].Name).To(Equal(testSecretName))
				Expect(nm.OwnerReferences[0].Kind).To(Equal("Secret"))
			})

			It("should record an event", func() {
				secret := forgeControlPlaneSecret(testSecretName, testNamespace, testRemoteCluster, nil)
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testSecretName,
						Namespace: testNamespace,
					},
				}

				_, err := cpSecretReconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())

				Expect(recorder.Events).To(Receive(ContainSubstring("NamespaceMapEnsured")))
			})
		})

		When("the NamespaceMap already exists", func() {
			It("should update the existing NamespaceMap", func() {
				secret := forgeControlPlaneSecret(testSecretName, testNamespace, testRemoteCluster, nil)
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				existingNM := &offloadingv1beta1.NamespaceMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-id",
						Namespace: testNamespace,
						Labels: map[string]string{
							consts.RemoteClusterID:           testRemoteCluster,
							consts.ReplicationRequestedLabel: "true",
						},
					},
				}
				Expect(fakeClient.Create(ctx, existingNM)).To(Succeed())

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testSecretName,
						Namespace: testNamespace,
					},
				}

				result, err := cpSecretReconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				nmList := &offloadingv1beta1.NamespaceMapList{}
				Expect(fakeClient.List(ctx, nmList, client.InNamespace(testNamespace))).To(Succeed())
				Expect(nmList.Items).To(HaveLen(1))

				nm := nmList.Items[0]
				Expect(nm.Labels).To(HaveKeyWithValue(consts.ReplicationRequestedLabel, "true"))
			})
		})

		When("the NamespaceMap already exists with incomplete labels", func() {
			It("should update the NamespaceMap labels through Reconcile", func() {
				secret := forgeControlPlaneSecret(testSecretName, testNamespace, testRemoteCluster, nil)
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				existingNM := &offloadingv1beta1.NamespaceMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testRemoteCluster,
						Namespace: testNamespace,
						Labels: map[string]string{
							consts.RemoteClusterID: testRemoteCluster,
						},
					},
				}
				Expect(fakeClient.Create(ctx, existingNM)).To(Succeed())

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testSecretName,
						Namespace: testNamespace,
					},
				}

				result, err := cpSecretReconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				updatedNM := &offloadingv1beta1.NamespaceMap{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{
					Name:      testRemoteCluster,
					Namespace: testNamespace,
				}, updatedNM)).To(Succeed())

				Expect(updatedNM.Labels).To(HaveKeyWithValue(consts.ReplicationRequestedLabel, "true"))
				Expect(updatedNM.Labels).To(HaveKeyWithValue(consts.ReplicationDestinationLabel, testRemoteCluster))
			})
		})
	})
})

var _ = BeforeSuite(func() {
	Expect(offloadingv1beta1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(authv1beta1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(liqocorev1beta1.AddToScheme(scheme.Scheme)).To(Succeed())
})

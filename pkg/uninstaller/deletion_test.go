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

package uninstaller_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/uninstaller"
)

var _ = Describe("Force-deletion helpers", func() {
	var (
		ctx    context.Context
		client *fake.FakeDynamicClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		_ = offloadingv1beta1.AddToScheme(scheme)
		_ = authv1beta1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)
		_ = liqov1beta1.AddToScheme(scheme)
		client = fake.NewSimpleDynamicClient(scheme)
	})

	newUnstructured := func(gvr schema.GroupVersionResource, namespace, name string, labels map[string]string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gvr.Group,
			Version: gvr.Version,
			Kind:    "Dummy",
		})
		obj.SetNamespace(namespace)
		obj.SetName(name)
		if len(labels) > 0 {
			obj.SetLabels(labels)
		}
		return obj
	}

	Context("NamespaceOffloadings", func() {
		It("should delete existing NamespaceOffloadings", func() {
			obj := newUnstructured(offloadingv1beta1.NamespaceOffloadingGroupVersionResource, "test-ns", "offloading", nil)
			_, err := client.Resource(offloadingv1beta1.NamespaceOffloadingGroupVersionResource).Namespace("test-ns").Create(ctx, obj, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(uninstaller.DeleteNamespaceOffloadings(ctx, client)).To(Succeed())

			_, err = client.Resource(offloadingv1beta1.NamespaceOffloadingGroupVersionResource).
				Namespace("test-ns").
				Get(ctx, "offloading", metav1.GetOptions{})
			Expect(k8serrors.IsNotFound(err)).To(BeTrue(), "NamespaceOffloading should be deleted")
		})
	})

	Context("VirtualNodes", func() {
		It("should delete existing VirtualNodes", func() {
			obj := newUnstructured(offloadingv1beta1.VirtualNodeGroupVersionResource, "liqo", "vn", nil)
			_, err := client.Resource(offloadingv1beta1.VirtualNodeGroupVersionResource).Namespace("liqo").Create(ctx, obj, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(uninstaller.DeleteVirtualNodes(ctx, client)).To(Succeed())

			_, err = client.Resource(offloadingv1beta1.VirtualNodeGroupVersionResource).Namespace("liqo").Get(ctx, "vn", metav1.GetOptions{})
			Expect(k8serrors.IsNotFound(err)).To(BeTrue(), "VirtualNode should be deleted")
		})
	})

	Context("ResourceSlices", func() {
		It("should delete existing ResourceSlices", func() {
			obj := newUnstructured(authv1beta1.ResourceSliceGroupVersionResource, "liqo", "rs", nil)
			_, err := client.Resource(authv1beta1.ResourceSliceGroupVersionResource).Namespace("liqo").Create(ctx, obj, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(uninstaller.DeleteResourceSlices(ctx, client)).To(Succeed())

			_, err = client.Resource(authv1beta1.ResourceSliceGroupVersionResource).Namespace("liqo").Get(ctx, "rs", metav1.GetOptions{})
			Expect(k8serrors.IsNotFound(err)).To(BeTrue(), "ResourceSlice should be deleted")
		})
	})

	Context("ForeignClusters", func() {
		It("should delete existing ForeignClusters", func() {
			obj := newUnstructured(liqov1beta1.ForeignClusterGroupVersionResource, "", "fc", nil)
			_, err := client.Resource(liqov1beta1.ForeignClusterGroupVersionResource).Create(ctx, obj, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(uninstaller.DeleteAllForeignClusters(ctx, client)).To(Succeed())

			_, err = client.Resource(liqov1beta1.ForeignClusterGroupVersionResource).Get(ctx, "fc", metav1.GetOptions{})
			Expect(k8serrors.IsNotFound(err)).To(BeTrue(), "ForeignCluster should be deleted")
		})
	})

	Context("Tenant namespaces", func() {
		It("should delete tenant namespaces and preserve regular namespaces", func() {
			tenantNs := &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{
					Name:   "liqo-tenant-foo",
					Labels: map[string]string{consts.TenantNamespaceLabel: "true"},
				},
			}
			regularNs := &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{
					Name:   "regular",
					Labels: map[string]string{"app": "foo"},
				},
			}

			unstrTenant, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tenantNs)
			Expect(err).ToNot(HaveOccurred())
			unstrRegular, err := runtime.DefaultUnstructuredConverter.ToUnstructured(regularNs)
			Expect(err).ToNot(HaveOccurred())

			nsGVR := corev1.SchemeGroupVersion.WithResource("namespaces")
			_, err = client.Resource(nsGVR).Create(ctx, &unstructured.Unstructured{Object: unstrTenant}, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = client.Resource(nsGVR).Create(ctx, &unstructured.Unstructured{Object: unstrRegular}, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(uninstaller.DeleteTenantNamespaces(ctx, client)).To(Succeed())

			_, err = client.Resource(nsGVR).Get(ctx, "liqo-tenant-foo", metav1.GetOptions{})
			Expect(k8serrors.IsNotFound(err)).To(BeTrue(), "Tenant namespace should be deleted")

			_, err = client.Resource(nsGVR).Get(ctx, "regular", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

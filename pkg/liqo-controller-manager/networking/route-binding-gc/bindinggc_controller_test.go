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

package routebindinggc

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/route"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestRouteBindingGC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Route Binding GC Suite")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
	Expect(networkingv1beta1.AddToScheme(scheme.Scheme)).To(Succeed())
})

func newGCReconciler(objs ...client.Object) *BindingGCReconciler {
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme).
		WithStatusSubresource(&networkingv1beta1.RouteConfigurationBinding{})
	if len(objs) > 0 {
		cb = cb.WithObjects(objs...)
	}
	return NewBindingGCReconciler(cb.Build(), 0)
}

func newRouteBindingForGC(name string, mutate func(b *networkingv1beta1.RouteConfigurationBinding)) *networkingv1beta1.RouteConfigurationBinding {
	b := &networkingv1beta1.RouteConfigurationBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "liqo",
		},
	}
	if mutate != nil {
		mutate(b)
	}
	return b
}

var _ = Describe("BindingGCReconciler", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("requeues linked bindings", func() {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}}
		binding := newRouteBindingForGC("binding-linked", func(b *networkingv1beta1.RouteConfigurationBinding) {
			b.Spec.TargetRef = networkingv1beta1.RouteBindingTargetReference{
				APIVersion: route.TargetAPIVersionV1,
				Kind:       route.TargetKindNode,
				Name:       "node1",
			}
		})
		r := newGCReconciler(node, binding)

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "binding-linked", Namespace: "liqo"}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(Equal(DefaultBindingGCPeriod))
	})

	It("deletes orphaned bindings and removes their finalizer", func() {
		binding := newRouteBindingForGC("binding-orphan", func(b *networkingv1beta1.RouteConfigurationBinding) {
			b.Finalizers = []string{route.RouteConfigurationBindingControllerFinalizer}
			b.Spec.TargetRef = networkingv1beta1.RouteBindingTargetReference{
				APIVersion: route.TargetAPIVersionV1,
				Kind:       route.TargetKindPod,
				Name:       "gw",
				Namespace:  "liqo",
			}
		})
		r := newGCReconciler(binding)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "binding-orphan", Namespace: "liqo"}})
		Expect(err).ToNot(HaveOccurred())

		// Removing the last finalizer on a deletion-timestamped object triggers the fake client's GC.
		got := &networkingv1beta1.RouteConfigurationBinding{}
		Expect(r.Client.Get(ctx, types.NamespacedName{Name: "binding-orphan", Namespace: "liqo"}, got)).ToNot(Succeed())
	})

	It("returns false for an empty target ref", func() {
		linked, err := (&BindingGCReconciler{}).isTargetLinked(ctx, networkingv1beta1.RouteBindingTargetReference{})
		Expect(err).ToNot(HaveOccurred())
		Expect(linked).To(BeFalse())
	})

})

var _ = Describe("newUnstructuredForTargetRef", func() {
	It("sets the correct GVK", func() {
		obj, err := newUnstructuredForTargetRef(networkingv1beta1.RouteBindingTargetReference{
			APIVersion: "networking.liqo.io/v1beta1",
			Kind:       "InternalNode",
			Name:       "node1",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(obj.GetAPIVersion()).To(Equal("networking.liqo.io/v1beta1"))
		Expect(obj.GetKind()).To(Equal("InternalNode"))
	})
})

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

package route

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const (
	routeBindingTestNamespace       = "liqo-tenant"
	routeBindingDelName             = "route-del"
	routeBindingNoFinName           = "route-nofin"
	routeBindingForeignFinalizer    = "other.liqo.io/finalizer"
	routeBindingTestTableName       = "liqo-route-table"
	routeBindingTestRouteConfigName = "route-cfg"
)

// newRouteBindingReconciler builds a reconciler with a fake client. Netlink branches are not exercised.
//
//nolint:unparam // recorder is returned for tests that need it; others discard it.
func newRouteBindingReconciler(objs ...client.Object) (*RouteConfigurationBindingReconciler, *events.FakeRecorder) {
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme).
		WithStatusSubresource(&networkingv1beta1.RouteConfigurationBinding{})
	if len(objs) > 0 {
		cb = cb.WithObjects(objs...)
	}
	rec := events.NewFakeRecorder(10)
	return &RouteConfigurationBindingReconciler{
		Client:         cb.Build(),
		Scheme:         scheme.Scheme,
		EventsRecorder: rec,
	}, rec
}

func newRouteBinding(name string, mutate func(a *networkingv1beta1.RouteConfigurationBinding)) *networkingv1beta1.RouteConfigurationBinding {
	a := &networkingv1beta1.RouteConfigurationBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: routeBindingTestNamespace,
		},
	}
	if mutate != nil {
		mutate(a)
	}
	return a
}

func newRouteConfiguration(name string) *networkingv1beta1.RouteConfiguration {
	dev := "lo"
	return &networkingv1beta1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: routeBindingTestNamespace,
		},
		Spec: networkingv1beta1.RouteConfigurationSpec{
			Table: networkingv1beta1.Table{
				Name: routeBindingTestTableName,
				Rules: []networkingv1beta1.Rule{
					{
						Routes: []networkingv1beta1.Route{
							{
								Dst: func() *networkingv1beta1.CIDR {
									c := networkingv1beta1.CIDR("127.192.0.0/24")
									return &c
								}(),
								Dev: &dev,
							},
						},
					},
				},
			},
		},
	}
}

var _ = Describe("RouteConfigurationBindingReconciler.Reconcile (non-netlink branches)", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("returns nil when the RouteConfigurationBinding is not found", func() {
		r, _ := newRouteBindingReconciler()
		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing", Namespace: routeBindingTestNamespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))
	})

	It("removes the finalizer on deletion when no table name is cached (no netlink calls)", func() {
		now := metav1.Now()
		a := newRouteBinding(routeBindingDelName, func(a *networkingv1beta1.RouteConfigurationBinding) {
			a.DeletionTimestamp = &now
			a.Finalizers = []string{routeConfigurationBindingControllerFinalizer}
			// Status.TableName intentionally empty: the controller must skip netlink calls.
		})
		r, _ := newRouteBindingReconciler(a)

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: routeBindingDelName, Namespace: routeBindingTestNamespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		// The object should have been deleted from the fake client because removing the
		// last finalizer on a deletion-timestamped object triggers the fake's GC.
		var got networkingv1beta1.RouteConfigurationBinding
		Expect(r.Client.Get(ctx, types.NamespacedName{Name: routeBindingDelName, Namespace: routeBindingTestNamespace}, &got)).ToNot(Succeed())
	})

	It("adds the finalizer on first reconcile", func() {
		routeCfg := newRouteConfiguration(routeBindingTestRouteConfigName)
		a := newRouteBinding("route-first", func(a *networkingv1beta1.RouteConfigurationBinding) {
			a.Spec.RouteConfigurationRef = corev1.LocalObjectReference{Name: routeBindingTestRouteConfigName}
		})
		r, _ := newRouteBindingReconciler(routeCfg, a)

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "route-first", Namespace: routeBindingTestNamespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		got := &networkingv1beta1.RouteConfigurationBinding{}
		Expect(r.Client.Get(ctx, types.NamespacedName{Name: "route-first", Namespace: routeBindingTestNamespace}, got)).To(Succeed())
		Expect(got.Finalizers).To(ContainElement(routeConfigurationBindingControllerFinalizer))
	})

	It("reports Applied=True when the referenced RouteConfiguration exists", func() {
		routeCfg := newRouteConfiguration(routeBindingTestRouteConfigName)
		a := newRouteBinding("route-applied", func(a *networkingv1beta1.RouteConfigurationBinding) {
			a.Spec.RouteConfigurationRef = corev1.LocalObjectReference{Name: routeBindingTestRouteConfigName}
			a.Finalizers = []string{routeConfigurationBindingControllerFinalizer}
		})
		r, _ := newRouteBindingReconciler(routeCfg, a)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "route-applied", Namespace: routeBindingTestNamespace}})
		Expect(err).ToNot(HaveOccurred())

		got := &networkingv1beta1.RouteConfigurationBinding{}
		Expect(r.Client.Get(ctx, types.NamespacedName{Name: "route-applied", Namespace: routeBindingTestNamespace}, got)).To(Succeed())
		cond := apimeta.FindStatusCondition(got.Status.Conditions, string(networkingv1beta1.RouteConfigurationBindingConditionTypeApplied))
		Expect(cond).ToNot(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionTrue))
	})

	It("returns an error and reports Applied=False when the referenced RouteConfiguration is missing", func() {
		a := newRouteBinding("route-missing-ref", func(a *networkingv1beta1.RouteConfigurationBinding) {
			a.Spec.RouteConfigurationRef = corev1.LocalObjectReference{Name: "does-not-exist"}
			a.Finalizers = []string{routeConfigurationBindingControllerFinalizer}
		})
		r, _ := newRouteBindingReconciler(a)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "route-missing-ref", Namespace: routeBindingTestNamespace}})
		Expect(err).To(HaveOccurred())

		got := &networkingv1beta1.RouteConfigurationBinding{}
		Expect(r.Client.Get(ctx, types.NamespacedName{Name: "route-missing-ref", Namespace: routeBindingTestNamespace}, got)).To(Succeed())
		cond := apimeta.FindStatusCondition(got.Status.Conditions, string(networkingv1beta1.RouteConfigurationBindingConditionTypeApplied))
		Expect(cond).ToNot(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
	})

})

var _ = Describe("MatchesRouteTargetRef", func() {
	It("matches when all fields are equal", func() {
		ref := &networkingv1beta1.RouteBindingTargetReference{
			APIVersion: TargetAPIVersionV1,
			Kind:       TargetKindPod,
			Name:       "gw",
			Namespace:  "liqo",
		}
		target := networkingv1beta1.RouteBindingTargetReference{
			APIVersion: TargetAPIVersionV1,
			Kind:       TargetKindPod,
			Name:       "gw",
			Namespace:  "liqo",
		}
		Expect(MatchesRouteTargetRef(ref, target)).To(BeTrue())
	})

	It("does not match when namespace differs", func() {
		ref := &networkingv1beta1.RouteBindingTargetReference{
			APIVersion: TargetAPIVersionV1,
			Kind:       TargetKindPod,
			Name:       "gw",
			Namespace:  "liqo",
		}
		target := networkingv1beta1.RouteBindingTargetReference{
			APIVersion: TargetAPIVersionV1,
			Kind:       TargetKindPod,
			Name:       "gw",
			Namespace:  "other",
		}
		Expect(MatchesRouteTargetRef(ref, target)).To(BeFalse())
	})
})

var _ = Describe("BindingResourceName", func() {
	It("returns the concatenated name when short enough", func() {
		Expect(BindingResourceName("cfg", "node1")).To(Equal("cfg-node1"))
	})

	It("falls back to a hashed name when the combined name is too long", func() {
		long := make([]byte, 300)
		for i := range long {
			long[i] = 'a'
		}
		name := BindingResourceName("cfg", string(long))
		Expect(len(name)).To(BeNumerically("<=", 253))
		Expect(name).To(HavePrefix("rb-"))
	})
})

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

package firewall

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const attachTestNamespace = "liqo-tenant"

// newAttachReconciler builds a reconciler with a nil nftables connection. Callers
// must only exercise reconcile branches that do not touch the connection.
func newAttachReconciler(objs ...client.Object) (*FirewallConfigurationAttachReconciler, *record.FakeRecorder) {
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme).
		WithStatusSubresource(&networkingv1beta1.FirewallConfigurationAttach{})
	if len(objs) > 0 {
		cb = cb.WithObjects(objs...)
	}
	rec := record.NewFakeRecorder(10)
	return &FirewallConfigurationAttachReconciler{
		NodeName:       "test-node",
		NftConnection:  nil, // unused in tested branches
		Client:         cb.Build(),
		Scheme:         scheme.Scheme,
		EventsRecorder: rec,
	}, rec
}

func newAttach(name string, mutate func(a *networkingv1beta1.FirewallConfigurationAttach)) *networkingv1beta1.FirewallConfigurationAttach {
	a := &networkingv1beta1.FirewallConfigurationAttach{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: attachTestNamespace,
		},
	}
	if mutate != nil {
		mutate(a)
	}
	return a
}

var _ = Describe("FirewallConfigurationAttachReconciler.Reconcile (non-nft branches)", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("returns nil when the FirewallConfigurationAttach is not found", func() {
		r, _ := newAttachReconciler()
		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing", Namespace: attachTestNamespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))
	})

	It("removes the finalizer on deletion when no table name is cached (no nft calls)", func() {
		now := metav1.Now()
		a := newAttach("att-del", func(a *networkingv1beta1.FirewallConfigurationAttach) {
			a.DeletionTimestamp = &now
			a.Finalizers = []string{firewallConfigurationAttachControllerFinalizer}
			// Status.TableName intentionally empty: the controller must skip nft calls.
		})
		r, _ := newAttachReconciler(a)

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "att-del", Namespace: attachTestNamespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		// The object should have been deleted from the fake client because removing the
		// last finalizer on a deletion-timestamped object triggers the fake's GC.
		var got networkingv1beta1.FirewallConfigurationAttach
		err = r.Get(ctx, types.NamespacedName{Name: "att-del", Namespace: attachTestNamespace}, &got)
		Expect(err).To(HaveOccurred())
	})

	It("is a no-op when the deletion timestamp is set but our finalizer is absent", func() {
		now := metav1.Now()
		a := newAttach("att-nofin", func(a *networkingv1beta1.FirewallConfigurationAttach) {
			a.DeletionTimestamp = &now
			// Keep at least one finalizer so the fake client accepts the deletion-timestamp object,
			// but it is NOT ours.
			a.Finalizers = []string{"other.liqo.io/finalizer"}
		})
		r, _ := newAttachReconciler(a)

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "att-nofin", Namespace: attachTestNamespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		// Object still exists and the foreign finalizer was not touched.
		var got networkingv1beta1.FirewallConfigurationAttach
		Expect(r.Get(ctx, types.NamespacedName{Name: "att-nofin", Namespace: attachTestNamespace}, &got)).To(Succeed())
		Expect(got.Finalizers).To(ContainElement("other.liqo.io/finalizer"))
	})

	It("requeues after 5s and returns an error when the referenced FirewallConfiguration is missing", func() {
		a := newAttach("att-orphan", func(a *networkingv1beta1.FirewallConfigurationAttach) {
			a.Spec.FirewallConfigurationRef = corev1.LocalObjectReference{Name: "missing-fwcfg"}
		})
		r, _ := newAttachReconciler(a)

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "att-orphan", Namespace: attachTestNamespace}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`referenced firewallconfiguration "missing-fwcfg" not found`))
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))
	})
})

var _ = Describe("FirewallConfigurationAttachReconciler finalizer helpers", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("ensureAttachFinalizerPresence adds our finalizer", func() {
		a := newAttach("a", nil)
		r, _ := newAttachReconciler(a)

		Expect(r.ensureAttachFinalizerPresence(ctx, a)).To(Succeed())

		var got networkingv1beta1.FirewallConfigurationAttach
		Expect(r.Get(ctx, types.NamespacedName{Name: "a", Namespace: attachTestNamespace}, &got)).To(Succeed())
		Expect(got.Finalizers).To(ContainElement(firewallConfigurationAttachControllerFinalizer))
	})

	It("ensureAttachFinalizerAbsence removes our finalizer and leaves others alone", func() {
		a := newAttach("a", func(a *networkingv1beta1.FirewallConfigurationAttach) {
			a.Finalizers = []string{firewallConfigurationAttachControllerFinalizer, "other.liqo.io/finalizer"}
		})
		r, _ := newAttachReconciler(a)

		Expect(r.ensureAttachFinalizerAbsence(ctx, a)).To(Succeed())

		var got networkingv1beta1.FirewallConfigurationAttach
		Expect(r.Get(ctx, types.NamespacedName{Name: "a", Namespace: attachTestNamespace}, &got)).To(Succeed())
		Expect(got.Finalizers).ToNot(ContainElement(firewallConfigurationAttachControllerFinalizer))
		Expect(got.Finalizers).To(ContainElement("other.liqo.io/finalizer"))
	})

	It("exposes the same finalizer string under both internal and external names", func() {
		Expect(FirewallConfigurationAttachControllerFinalizer).To(Equal(firewallConfigurationAttachControllerFinalizer))
	})
})

var _ = Describe("updateStatus", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("sets ConditionTrue, Applied type, and emits an event on success", func() {
		a := newAttach("a", nil)
		r, rec := newAttachReconciler(a)

		Expect(r.updateStatus(ctx, a, nil)).To(Succeed())

		var got networkingv1beta1.FirewallConfigurationAttach
		Expect(r.Get(ctx, types.NamespacedName{Name: "a", Namespace: attachTestNamespace}, &got)).To(Succeed())
		Expect(got.Status.Type).To(Equal(networkingv1beta1.FirewallConfigurationAttachConditionTypeApplied))
		Expect(got.Status.Status).To(Equal(metav1.ConditionTrue))
		Expect(got.Status.LastTransitionTime.IsZero()).To(BeFalse())
		Eventually(rec.Events).Should(Receive(ContainSubstring("FirewallConfigurationAttachUpdate")))
	})

	It("sets ConditionFalse and returns the original reconcile error", func() {
		a := newAttach("a", nil)
		r, _ := newAttachReconciler(a)
		recErr := errors.New("boom")

		err := r.updateStatus(ctx, a, recErr)
		Expect(err).To(MatchError(recErr))

		var got networkingv1beta1.FirewallConfigurationAttach
		Expect(r.Get(ctx, types.NamespacedName{Name: "a", Namespace: attachTestNamespace}, &got)).To(Succeed())
		Expect(got.Status.Status).To(Equal(metav1.ConditionFalse))
	})

	It("is a no-op when the new status equals the existing one", func() {
		a := newAttach("a", func(a *networkingv1beta1.FirewallConfigurationAttach) {
			a.Status.Status = metav1.ConditionTrue
			a.Status.Type = networkingv1beta1.FirewallConfigurationAttachConditionTypeApplied
			a.Status.LastTransitionTime = metav1.Now()
		})
		r, rec := newAttachReconciler(a)

		// Capture the time before; if no update is issued it should remain unchanged.
		original := a.Status.LastTransitionTime
		Expect(r.updateStatus(ctx, a, nil)).To(Succeed())
		Expect(a.Status.LastTransitionTime).To(Equal(original))
		// No event should be emitted.
		Consistently(rec.Events).ShouldNot(Receive())
	})
})

var _ = Describe("forgeLabelsPredicate", func() {
	makeObj := func(lbls map[string]string) client.Object {
		return &networkingv1beta1.FirewallConfigurationAttach{
			ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: attachTestNamespace, Labels: lbls},
		}
	}

	It("matches objects carrying any of the configured label sets", func() {
		p, err := forgeLabelsPredicate([]labels.Set{
			{"role": "fabric"},
			{"role": "gateway"},
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(p.Create(event.CreateEvent{Object: makeObj(map[string]string{"role": "fabric"})})).To(BeTrue())
		Expect(p.Create(event.CreateEvent{Object: makeObj(map[string]string{"role": "gateway"})})).To(BeTrue())
		Expect(p.Create(event.CreateEvent{Object: makeObj(map[string]string{"role": "other"})})).To(BeFalse())
		Expect(p.Create(event.CreateEvent{Object: makeObj(nil)})).To(BeFalse())
	})

	It("returns a predicate that never matches when no label sets are provided", func() {
		p, err := forgeLabelsPredicate(nil)
		Expect(err).ToNot(HaveOccurred())
		// predicate.Or() over an empty list returns false for every event.
		Expect(p.Create(event.CreateEvent{Object: makeObj(map[string]string{"any": "thing"})})).To(BeFalse())
	})
})

// Ensure the scheme import is exercised (silences future unused-import lint if branches change).
var _ = runtime.Object(&networkingv1beta1.FirewallConfigurationAttach{})

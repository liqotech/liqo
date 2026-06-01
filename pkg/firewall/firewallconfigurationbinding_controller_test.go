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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const (
	bindingTestNamespace            = "liqo-tenant"
	bindingDelName                  = "att-del"
	bindingNoFinName                = "att-nofin"
	firewallBindingForeignFinalizer = "other.liqo.io/finalizer"
)

// newBindingReconciler builds a reconciler with a nil nftables connection. Callers
// must only exercise reconcile branches that do not touch the connection.
func newBindingReconciler(objs ...client.Object) (*FirewallConfigurationBindingReconciler, *events.FakeRecorder) {
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme).
		WithStatusSubresource(&networkingv1beta1.FirewallConfigurationBinding{})
	if len(objs) > 0 {
		cb = cb.WithObjects(objs...)
	}
	rec := events.NewFakeRecorder(10)
	return &FirewallConfigurationBindingReconciler{
		NodeName:       "test-node",
		NftConnection:  nil, // unused in tested branches
		Client:         cb.Build(),
		Scheme:         scheme.Scheme,
		EventsRecorder: rec,
	}, rec
}

func newBinding(name string, mutate func(a *networkingv1beta1.FirewallConfigurationBinding)) *networkingv1beta1.FirewallConfigurationBinding {
	a := &networkingv1beta1.FirewallConfigurationBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: bindingTestNamespace,
		},
	}
	if mutate != nil {
		mutate(a)
	}
	return a
}

var _ = Describe("FirewallConfigurationBindingReconciler.Reconcile (non-nft branches)", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("returns nil when the FirewallConfigurationBinding is not found", func() {
		r, _ := newBindingReconciler()
		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing", Namespace: bindingTestNamespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))
	})

	It("removes the finalizer on deletion when no table name is cached (no nft calls)", func() {
		now := metav1.Now()
		a := newBinding(bindingDelName, func(a *networkingv1beta1.FirewallConfigurationBinding) {
			a.DeletionTimestamp = &now
			a.Finalizers = []string{firewallConfigurationBindingControllerFinalizer}
			// Status.TableName intentionally empty: the controller must skip nft calls.
		})
		r, _ := newBindingReconciler(a)

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: bindingDelName, Namespace: bindingTestNamespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		// The object should have been deleted from the fake client because removing the
		// last finalizer on a deletion-timestamped object triggers the fake's GC.
		var got networkingv1beta1.FirewallConfigurationBinding
		err = r.Get(ctx, types.NamespacedName{Name: bindingDelName, Namespace: bindingTestNamespace}, &got)
		Expect(err).To(HaveOccurred())
	})

	It("is a no-op when the deletion timestamp is set but our finalizer is absent", func() {
		now := metav1.Now()
		a := newBinding(bindingNoFinName, func(a *networkingv1beta1.FirewallConfigurationBinding) {
			a.DeletionTimestamp = &now
			// Keep at least one finalizer so the fake client accepts the deletion-timestamp object,
			// but it is NOT ours.
			a.Finalizers = []string{firewallBindingForeignFinalizer}
		})
		r, _ := newBindingReconciler(a)

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: bindingNoFinName, Namespace: bindingTestNamespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		// Object still exists and the foreign finalizer was not touched.
		var got networkingv1beta1.FirewallConfigurationBinding
		Expect(r.Get(ctx, types.NamespacedName{Name: bindingNoFinName, Namespace: bindingTestNamespace}, &got)).To(Succeed())
		Expect(got.Finalizers).To(ContainElement(firewallBindingForeignFinalizer))
	})

	It("requeues after 5s and returns an error when the referenced FirewallConfiguration is missing", func() {
		a := newBinding("att-orphan", func(a *networkingv1beta1.FirewallConfigurationBinding) {
			a.Spec.FirewallConfigurationRef = corev1.LocalObjectReference{Name: "missing-fwcfg"}
		})
		r, _ := newBindingReconciler(a)

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "att-orphan", Namespace: bindingTestNamespace}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`referenced firewallconfiguration "missing-fwcfg" not found`))
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))
	})
})

var _ = Describe("FirewallConfigurationBindingReconciler finalizer helpers", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("ensureBindingFinalizerPresence adds our finalizer", func() {
		a := newBinding("a", nil)
		r, _ := newBindingReconciler(a)

		Expect(r.ensureBindingFinalizerPresence(ctx, a)).To(Succeed())

		var got networkingv1beta1.FirewallConfigurationBinding
		Expect(r.Get(ctx, types.NamespacedName{Name: "a", Namespace: bindingTestNamespace}, &got)).To(Succeed())
		Expect(got.Finalizers).To(ContainElement(firewallConfigurationBindingControllerFinalizer))
	})

	It("ensureBindingFinalizerAbsence removes our finalizer and leaves others alone", func() {
		a := newBinding("a", func(a *networkingv1beta1.FirewallConfigurationBinding) {
			a.Finalizers = []string{firewallConfigurationBindingControllerFinalizer, firewallBindingForeignFinalizer}
		})
		r, _ := newBindingReconciler(a)

		Expect(r.ensureBindingFinalizerAbsence(ctx, a)).To(Succeed())

		var got networkingv1beta1.FirewallConfigurationBinding
		Expect(r.Get(ctx, types.NamespacedName{Name: "a", Namespace: bindingTestNamespace}, &got)).To(Succeed())
		Expect(got.Finalizers).ToNot(ContainElement(firewallConfigurationBindingControllerFinalizer))
		Expect(got.Finalizers).To(ContainElement(firewallBindingForeignFinalizer))
	})

	It("exposes the same finalizer string under both internal and external names", func() {
		Expect(FirewallConfigurationBindingControllerFinalizer).To(Equal(firewallConfigurationBindingControllerFinalizer))
	})
})

var _ = Describe("updateStatus", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("sets ConditionTrue, Applied type, and emits an event on success", func() {
		a := newBinding("a", nil)
		r, rec := newBindingReconciler(a)

		Expect(r.updateStatus(ctx, a, nil)).To(Succeed())

		var got networkingv1beta1.FirewallConfigurationBinding
		Expect(r.Get(ctx, types.NamespacedName{Name: "a", Namespace: bindingTestNamespace}, &got)).To(Succeed())
		cond := apimeta.FindStatusCondition(got.Status.Conditions, string(networkingv1beta1.FirewallConfigurationBindingConditionTypeApplied))
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		Expect(cond.LastTransitionTime.IsZero()).To(BeFalse())
		Eventually(rec.Events).Should(Receive(ContainSubstring("FirewallConfigurationBindingUpdate")))
	})

	It("sets ConditionFalse and returns the original reconcile error", func() {
		a := newBinding("a", nil)
		r, _ := newBindingReconciler(a)
		recErr := errors.New("boom")

		err := r.updateStatus(ctx, a, recErr)
		Expect(err).To(MatchError(recErr))

		var got networkingv1beta1.FirewallConfigurationBinding
		Expect(r.Get(ctx, types.NamespacedName{Name: "a", Namespace: bindingTestNamespace}, &got)).To(Succeed())
		cond := apimeta.FindStatusCondition(got.Status.Conditions, string(networkingv1beta1.FirewallConfigurationBindingConditionTypeApplied))
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
	})

	It("is a no-op when the new status equals the existing one", func() {
		a := newBinding("a", func(a *networkingv1beta1.FirewallConfigurationBinding) {
			apimeta.SetStatusCondition(&a.Status.Conditions, metav1.Condition{
				Type:    string(networkingv1beta1.FirewallConfigurationBindingConditionTypeApplied),
				Status:  metav1.ConditionTrue,
				Reason:  "Applied",
				Message: string(metav1.ConditionTrue),
			})
		})
		r, rec := newBindingReconciler(a)

		// Capture the time before; if no update is issued it should remain unchanged.
		original := apimeta.FindStatusCondition(a.Status.Conditions,
			string(networkingv1beta1.FirewallConfigurationBindingConditionTypeApplied)).LastTransitionTime
		Expect(r.updateStatus(ctx, a, nil)).To(Succeed())
		Expect(apimeta.FindStatusCondition(a.Status.Conditions,
			string(networkingv1beta1.FirewallConfigurationBindingConditionTypeApplied)).LastTransitionTime).To(Equal(original))
		// No event should be emitted.
		Consistently(rec.Events).ShouldNot(Receive())
	})
})

var _ = Describe("forgeTargetIDPredicate", func() {
	makeObj := func(targetID string) client.Object {
		return &networkingv1beta1.FirewallConfigurationBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: bindingTestNamespace},
			Spec:       networkingv1beta1.FirewallConfigurationBindingSpec{TargetID: targetID},
		}
	}

	It("matches objects whose spec.targetID equals the configured value", func() {
		p := forgeTargetIDPredicate("my-node")

		Expect(p.Create(event.CreateEvent{Object: makeObj("my-node")})).To(BeTrue())
		Expect(p.Update(event.UpdateEvent{ObjectNew: makeObj("my-node")})).To(BeTrue())
		Expect(p.Delete(event.DeleteEvent{Object: makeObj("my-node")})).To(BeTrue())
	})

	It("does not match objects with a different spec.targetID", func() {
		p := forgeTargetIDPredicate("my-node")

		Expect(p.Create(event.CreateEvent{Object: makeObj("other-node")})).To(BeFalse())
		Expect(p.Create(event.CreateEvent{Object: makeObj("")})).To(BeFalse())
	})

	It("does not match objects of a different type", func() {
		p := forgeTargetIDPredicate("my-node")

		Expect(p.Create(event.CreateEvent{Object: &networkingv1beta1.FirewallConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "x"},
		}})).To(BeFalse())
	})
})

// Ensure the scheme import is exercised (silences future unused-import lint if branches change).
var _ = runtime.Object(&networkingv1beta1.FirewallConfigurationBinding{})

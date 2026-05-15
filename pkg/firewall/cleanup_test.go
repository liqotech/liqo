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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const (
	appLabelKey      = "app"
	foreignFinalizer = "other.liqo.io/finalizer"
)

func newBindingWith(name string, targetRef networkingv1beta1.TargetReference,
	fins []string) *networkingv1beta1.FirewallConfigurationBinding {
	return &networkingv1beta1.FirewallConfigurationBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  bindingTestNamespace,
			Finalizers: fins,
		},
		Spec: networkingv1beta1.FirewallConfigurationBindingSpec{
			TargetRef: targetRef,
		},
	}
}

func fabricTargetRef() networkingv1beta1.TargetReference {
	return networkingv1beta1.TargetReference{
		APIVersion: "v1",
		Kind:       "Node",
		Name:       "fabric-node",
	}
}

func otherTargetRef(name string) networkingv1beta1.TargetReference {
	return networkingv1beta1.TargetReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       name,
		Namespace:  "other-ns",
	}
}

func getBinding(c client.Client, name string) *networkingv1beta1.FirewallConfigurationBinding {
	var a networkingv1beta1.FirewallConfigurationBinding
	err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: bindingTestNamespace}, &a)
	if err != nil {
		return nil
	}
	return &a
}

var _ = Describe("CleanupFirewallConfigurationBindings", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("removes our finalizer from a targetRef-matched binding", func() {
		a := newBindingWith("match", fabricTargetRef(),
			[]string{firewallConfigurationBindingControllerFinalizer})
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(a).Build()

		CleanupFirewallConfigurationBindings(ctx, cl,
			"v1", "Node", "fabric-node", "", false)

		got := getBinding(cl, "match")
		Expect(got).ToNot(BeNil())
		Expect(got.Finalizers).ToNot(ContainElement(firewallConfigurationBindingControllerFinalizer))
	})

	It("does NOT touch bindings with a different spec.targetRef", func() {
		a := newBindingWith("no-match", otherTargetRef("other-node"),
			[]string{firewallConfigurationBindingControllerFinalizer})
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(a).Build()

		CleanupFirewallConfigurationBindings(ctx, cl,
			"v1", "Node", "fabric-node", "", false)

		got := getBinding(cl, "no-match")
		Expect(got).ToNot(BeNil())
		Expect(got.Finalizers).To(ContainElement(firewallConfigurationBindingControllerFinalizer))
	})

	It("skips bindings that do NOT carry our finalizer", func() {
		a := newBindingWith("foreign-fin", fabricTargetRef(),
			[]string{foreignFinalizer})
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(a).Build()

		CleanupFirewallConfigurationBindings(ctx, cl,
			"v1", "Node", "fabric-node", "", false)

		got := getBinding(cl, "foreign-fin")
		Expect(got).ToNot(BeNil())
		Expect(got.Finalizers).To(ContainElement(foreignFinalizer))
	})

	It("processes multiple bindings with the matching targetRef", func() {
		a := newBindingWith("a", fabricTargetRef(),
			[]string{firewallConfigurationBindingControllerFinalizer})
		b := newBindingWith("b", fabricTargetRef(),
			[]string{firewallConfigurationBindingControllerFinalizer})
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(a, b).Build()

		CleanupFirewallConfigurationBindings(ctx, cl,
			"v1", "Node", "fabric-node", "", false)

		gotA := getBinding(cl, "a")
		Expect(gotA).ToNot(BeNil())
		Expect(gotA.Finalizers).ToNot(ContainElement(firewallConfigurationBindingControllerFinalizer))
		gotB := getBinding(cl, "b")
		Expect(gotB).ToNot(BeNil())
		Expect(gotB.Finalizers).ToNot(ContainElement(firewallConfigurationBindingControllerFinalizer))
	})

	It("does not panic when given an empty targetRef name", func() {
		Expect(func() {
			CleanupFirewallConfigurationBindings(ctx, fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				"v1", "Node", "", "", false)
		}).ToNot(Panic())
	})
})

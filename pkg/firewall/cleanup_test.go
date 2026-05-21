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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const (
	appLabelKey      = "app"
	fabricLabelVal   = "fabric"
	otherLabelVal    = "other"
	gatewayLabelVal  = "gateway"
	foreignFinalizer = "other.liqo.io/finalizer"
)

func newAttachWith(name string, lbls map[string]string, fins []string, deleting bool) *networkingv1beta1.FirewallConfigurationAttach {
	a := &networkingv1beta1.FirewallConfigurationAttach{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  attachTestNamespace,
			Labels:     lbls,
			Finalizers: fins,
		},
	}
	if deleting {
		now := metav1.Now()
		a.DeletionTimestamp = &now
	}
	return a
}

func getAttach(c client.Client, name string) *networkingv1beta1.FirewallConfigurationAttach {
	var a networkingv1beta1.FirewallConfigurationAttach
	err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: attachTestNamespace}, &a)
	if err != nil {
		return nil
	}
	return &a
}

var _ = Describe("CleanupPendingAttachFinalizers", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("removes our finalizer from a deletion-pending, label-matched attach", func() {
		a := newAttachWith("match", map[string]string{appLabelKey: fabricLabelVal},
			[]string{firewallConfigurationAttachControllerFinalizer},
			true /* deleting */)
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(a).Build()

		CleanupPendingAttachFinalizers(ctx, cl, []labels.Set{{appLabelKey: fabricLabelVal}})

		// Removing the only finalizer on a deletion-timestamped object triggers fake-client GC,
		// so the object should be gone.
		Expect(getAttach(cl, "match")).To(BeNil())
	})

	It("does NOT touch attaches whose labels do not match any set", func() {
		a := newAttachWith("no-label", map[string]string{appLabelKey: otherLabelVal},
			[]string{firewallConfigurationAttachControllerFinalizer},
			true)
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(a).Build()

		CleanupPendingAttachFinalizers(ctx, cl, []labels.Set{{appLabelKey: fabricLabelVal}})

		got := getAttach(cl, "no-label")
		Expect(got).ToNot(BeNil())
		Expect(got.Finalizers).To(ContainElement(firewallConfigurationAttachControllerFinalizer))
	})

	It("skips attaches that are NOT pending deletion", func() {
		a := newAttachWith("alive", map[string]string{appLabelKey: fabricLabelVal},
			[]string{firewallConfigurationAttachControllerFinalizer},
			false /* not deleting */)
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(a).Build()

		CleanupPendingAttachFinalizers(ctx, cl, []labels.Set{{appLabelKey: fabricLabelVal}})

		got := getAttach(cl, "alive")
		Expect(got).ToNot(BeNil())
		Expect(got.Finalizers).To(ContainElement(firewallConfigurationAttachControllerFinalizer))
	})

	It("skips attaches that do NOT carry our finalizer", func() {
		a := newAttachWith("foreign-fin", map[string]string{appLabelKey: fabricLabelVal},
			[]string{foreignFinalizer},
			true)
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(a).Build()

		CleanupPendingAttachFinalizers(ctx, cl, []labels.Set{{appLabelKey: fabricLabelVal}})

		got := getAttach(cl, "foreign-fin")
		Expect(got).ToNot(BeNil())
		Expect(got.Finalizers).To(ContainElement(foreignFinalizer))
	})

	It("processes all provided label sets", func() {
		a := newAttachWith("a", map[string]string{appLabelKey: fabricLabelVal},
			[]string{firewallConfigurationAttachControllerFinalizer}, true)
		b := newAttachWith("b", map[string]string{appLabelKey: gatewayLabelVal},
			[]string{firewallConfigurationAttachControllerFinalizer}, true)
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(a, b).Build()

		CleanupPendingAttachFinalizers(ctx, cl,
			[]labels.Set{{appLabelKey: fabricLabelVal}, {appLabelKey: gatewayLabelVal}})

		Expect(getAttach(cl, "a")).To(BeNil())
		Expect(getAttach(cl, "b")).To(BeNil())
	})

	It("does not panic when given an empty label set list", func() {
		Expect(func() {
			CleanupPendingAttachFinalizers(ctx, fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(), nil)
		}).ToNot(Panic())
	})
})

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

package sharedgatewayctrl

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Shared Gateway route annotation", func() {
	const namespace = "tenant"

	var (
		ctx        context.Context
		cl         client.Client
		reconciler *RouteReconciler
		route      *gwv1.HTTPRoute
		gateway    *gwv1.Gateway
	)

	shared := types.NamespacedName{Namespace: "infra", Name: "public"}
	key := types.NamespacedName{Namespace: namespace, Name: "route"}

	newGateway := func(programmed metav1.ConditionStatus, addresses ...string) *gwv1.Gateway {
		gw := &gwv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: shared.Name, Namespace: shared.Namespace,
				Labels: map[string]string{consts.SharedGatewayLabel: consts.SharedGatewayLabelValue}},
			Status: gwv1.GatewayStatus{Conditions: []metav1.Condition{{
				Type: string(gwv1.GatewayConditionProgrammed), Status: programmed, Reason: "Reason", LastTransitionTime: metav1.Now(),
			}}},
		}
		for _, address := range addresses {
			gw.Status.Addresses = append(gw.Status.Addresses, gwv1.GatewayStatusAddress{Type: ptr.To(gwv1.IPAddressType), Value: address})
		}
		return gw
	}

	newRoute := func(annotations map[string]string, parents ...gwv1.ParentReference) *gwv1.HTTPRoute {
		return &gwv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Annotations: annotations,
				Labels: map[string]string{forge.LiqoOriginClusterIDKey: "consumer"}},
			Spec: gwv1.HTTPRouteSpec{CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: parents}},
		}
	}

	sharedParent := gwv1.ParentReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")}

	annotations := func() map[string]string {
		var current gwv1.HTTPRoute
		Expect(cl.Get(ctx, key, &current)).To(Succeed())
		return current.GetAnnotations()
	}

	BeforeEach(func() {
		ctx = context.Background()
		gateway = newGateway(metav1.ConditionTrue, "10.0.0.1", "10.0.0.2")
		route = newRoute(map[string]string{"foo": "bar"}, sharedParent)
	})

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(gwv1.Install(scheme)).To(Succeed())
		cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(gateway, route).
			WithStatusSubresource(&gwv1.Gateway{}).Build()
		reconciler = &RouteReconciler{Client: cl, Kind: offloadingv1beta1.HTTPRouteKind}

		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).ToNot(HaveOccurred())
	})

	When("the route is attached to a programmed shared Gateway", func() {
		It("should annotate the route with its addresses, preserving the other annotations", func() {
			Expect(annotations()).To(HaveKeyWithValue(consts.SharedGatewayAddressesAnnotation,
				`[{"type":"IPAddress","value":"10.0.0.1"},{"type":"IPAddress","value":"10.0.0.2"}]`))
			Expect(annotations()).To(HaveKeyWithValue("foo", "bar"))
		})

		It("should enqueue the route when the shared Gateway changes", func() {
			Expect(reconciler.attachedRoutes(ctx, gateway)).To(ConsistOf(ctrl.Request{NamespacedName: key}))
		})
	})

	When("the shared Gateway is not programmed", func() {
		BeforeEach(func() {
			gateway = newGateway(metav1.ConditionFalse, "10.0.0.1")
			route = newRoute(map[string]string{consts.SharedGatewayAddressesAnnotation: "stale"}, sharedParent)
		})

		It("should remove the stale annotation", func() {
			Expect(annotations()).ToNot(HaveKey(consts.SharedGatewayAddressesAnnotation))
		})
	})

	When("the Gateway is no longer labeled as shared", func() {
		BeforeEach(func() {
			gateway.Labels = nil
			route = newRoute(map[string]string{consts.SharedGatewayAddressesAnnotation: "stale"}, sharedParent)
		})

		It("should remove the stale annotation", func() {
			Expect(annotations()).ToNot(HaveKey(consts.SharedGatewayAddressesAnnotation))
		})
	})

	When("the route is not attached to a shared Gateway", func() {
		BeforeEach(func() {
			route = newRoute(map[string]string{consts.SharedGatewayAddressesAnnotation: "stale"}, gwv1.ParentReference{Name: "other"})
		})

		It("should remove the stale annotation", func() {
			Expect(annotations()).ToNot(HaveKey(consts.SharedGatewayAddressesAnnotation))
		})

		It("should not enqueue the route when the shared Gateway changes", func() {
			Expect(reconciler.attachedRoutes(ctx, gateway)).To(BeEmpty())
		})
	})
})

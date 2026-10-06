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

package gatewayapi

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Validation", func() {
	const namespace = "offloaded"

	var (
		cl  client.Client
		res admission.Response
	)

	namespaceObj := func(name, clusterID string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{consts.RemoteClusterID: clusterID}}}
	}
	gateway := func(name, label string) *gwv1.Gateway {
		gw := &gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "infra"}}
		if label != "" {
			gw.Labels = map[string]string{consts.SharedGatewayLabel: label}
		}
		return gw
	}
	route := func(parents ...gwv1.ParentReference) *gwv1.HTTPRoute {
		return &gwv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "route", Namespace: namespace},
			Spec:       gwv1.HTTPRouteSpec{CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: parents}},
		}
	}
	service := func(namespace, name string) gwv1.ParentReference {
		return gwv1.ParentReference{Name: gwv1.ObjectName(name), Namespace: ptr.To(gwv1.Namespace(namespace)),
			Group: ptr.To[gwv1.Group](""), Kind: ptr.To[gwv1.Kind]("Service")}
	}
	gatewayRef := func(namespace, name string) gwv1.ParentReference {
		return gwv1.ParentReference{Name: gwv1.ObjectName(name), Namespace: ptr.To(gwv1.Namespace(namespace))}
	}

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(gwv1.Install(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			gateway("public", consts.SharedGatewayLabelValue), gateway("private", ""),
			namespaceObj(namespace, "consumer"), namespaceObj("sibling", "consumer"), namespaceObj("stranger", "other"),
		).Build()
	})

	handle := func(kind string, op admissionv1.Operation, obj, old client.Object) {
		res = NewValidator(cl, []string{"envoy"}).Handle(context.Background(), request(kind, op, obj, old))
	}

	DescribeTable("the routes",
		func(allowed bool, parents ...gwv1.ParentReference) {
			handle(httpRouteKind, admissionv1.Create, route(parents...), nil)
			Expect(res.Allowed).To(Equal(allowed), res.Result.Message)
		},
		Entry("attached to a Gateway in the same namespace", true, gwv1.ParentReference{Name: "liqo"}),
		Entry("attached to a shared Gateway", true, gatewayRef("infra", "public")),
		Entry("attached to a Gateway not shared", false, gatewayRef("infra", "private")),
		Entry("attached to a non-existing Gateway", false, gatewayRef("infra", "missing")),
		Entry("attached to a Service in the same namespace", true, gwv1.ParentReference{Name: "svc", Group: ptr.To[gwv1.Group](""),
			Kind: ptr.To[gwv1.Kind]("Service")}),
		Entry("attached to a Service offloaded by the same cluster", true, service("sibling", "svc")),
		Entry("attached to a Service offloaded by another cluster", false, service("stranger", "svc")),
		Entry("attached to a Service in a namespace not offloaded", false, service("kube-system", "svc")),
		Entry("attached to an unsupported kind", false, gwv1.ParentReference{Name: "set", Kind: ptr.To[gwv1.Kind]("ListenerSet")}),
	)

	It("should deny the routes referring to extension filters", func() {
		obj := route(gwv1.ParentReference{Name: "liqo"})
		obj.Spec.Rules = []gwv1.HTTPRouteRule{{Filters: []gwv1.HTTPRouteFilter{{
			Type: gwv1.HTTPRouteFilterExtensionRef, ExtensionRef: &gwv1.LocalObjectReference{Name: "auth", Kind: "Policy"},
		}}}}
		handle(httpRouteKind, admissionv1.Create, obj, nil)
		Expect(res.Allowed).To(BeFalse())
		Expect(res.Result.Message).To(ContainSubstring("extension filters"))
	})

	It("should validate only the parents added by updates", func() {
		// The route was attached to a Gateway shared at that time, and it is now updated (e.g., to annotate it).
		old := route(gatewayRef("infra", "private"))
		updated := route(gatewayRef("infra", "private"))
		updated.SetAnnotations(map[string]string{"foo": "bar"})
		handle(httpRouteKind, admissionv1.Update, updated, old)
		Expect(res.Allowed).To(BeTrue())

		updated.Spec.ParentRefs = append(updated.Spec.ParentRefs, gatewayRef("infra", "other"))
		handle(httpRouteKind, admissionv1.Update, updated, old)
		Expect(res.Allowed).To(BeFalse())
	})

	DescribeTable("the Gateways",
		func(op admissionv1.Operation, class, oldClass string, allowed bool) {
			gw := &gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: namespace},
				Spec: gwv1.GatewaySpec{GatewayClassName: gwv1.ObjectName(class)}}
			var old client.Object
			if oldClass != "" {
				old = &gwv1.Gateway{ObjectMeta: gw.ObjectMeta, Spec: gwv1.GatewaySpec{GatewayClassName: gwv1.ObjectName(oldClass)}}
			}
			handle(gatewayKind, op, gw, old)
			Expect(res.Allowed).To(Equal(allowed))
		},
		Entry("created with an offered GatewayClass", admissionv1.Create, "envoy", "", true),
		Entry("created with a GatewayClass not offered", admissionv1.Create, "istio", "", false),
		Entry("updated to a GatewayClass not offered", admissionv1.Update, "istio", "envoy", false),
		Entry("updated without changing a GatewayClass no longer offered", admissionv1.Update, "istio", "istio", true),
	)
})

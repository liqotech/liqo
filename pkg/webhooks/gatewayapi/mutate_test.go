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
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Route mutation", func() {
	const namespace = "offloaded"

	var (
		route  *gwv1.HTTPRoute
		shared *types.NamespacedName
	)

	public := gwv1.ParentReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")}
	reflected := gwv1.ParentReference{Name: "liqo", SectionName: ptr.To[gwv1.SectionName]("https")}

	newRoute := func(annotations map[string]string, parents ...gwv1.ParentReference) *gwv1.HTTPRoute {
		return &gwv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "route", Namespace: namespace, Annotations: annotations},
			Spec:       gwv1.HTTPRouteSpec{CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: parents}},
		}
	}

	Describe("the MutateRoute function", func() {
		var modified bool

		BeforeEach(func() { shared = &types.NamespacedName{Namespace: "infra", Name: "public"} })
		JustBeforeEach(func() { modified = MutateRoute(route, &route.Spec.CommonRouteSpec, shared) })

		When("the route refers to the placeholder", func() {
			BeforeEach(func() {
				route = newRoute(map[string]string{"foo": "bar"}, reflected, forge.SharedGatewayPlaceholderRef(), public)
			})

			It("should replace it with the shared Gateway, without duplicates", func() {
				Expect(modified).To(BeTrue())
				Expect(route.Spec.ParentRefs).To(Equal([]gwv1.ParentReference{reflected, public}))
			})
			It("should annotate the route with the shared Gateway", func() {
				Expect(route.Annotations).To(HaveKeyWithValue(consts.SharedGatewayAnnotation, "infra/public"))
				Expect(route.Annotations).To(HaveKeyWithValue("foo", "bar"))
			})

			When("no Gateway is shared", func() {
				BeforeEach(func() {
					shared = nil
					route = newRoute(nil, reflected, forge.SharedGatewayPlaceholderRef())
				})

				It("should remove the placeholder", func() {
					Expect(modified).To(BeTrue())
					Expect(route.Spec.ParentRefs).To(Equal([]gwv1.ParentReference{reflected}))
					Expect(route.Annotations).ToNot(HaveKey(consts.SharedGatewayAnnotation))
				})
			})
		})

		When("the route no longer refers to the shared Gateway it is annotated with", func() {
			BeforeEach(func() { route = newRoute(map[string]string{consts.SharedGatewayAnnotation: "infra/public"}, reflected) })

			It("should remove the annotation", func() {
				Expect(modified).To(BeTrue())
				Expect(route.Annotations).ToNot(HaveKey(consts.SharedGatewayAnnotation))
			})
		})

		When("the route has already been mutated", func() {
			BeforeEach(func() {
				route = newRoute(map[string]string{consts.SharedGatewayAnnotation: "infra/public"}, reflected, public)
			})

			It("should not modify it", func() {
				Expect(modified).To(BeFalse())
				Expect(route.Spec.ParentRefs).To(Equal([]gwv1.ParentReference{reflected, public}))
			})
		})
	})

	Describe("the mutating webhook", func() {
		var (
			cl       client.Client
			response func() []jsonpatch.JsonPatchOperation
		)

		gateway := func(name, label string) *gwv1.Gateway {
			gw := &gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "infra"}}
			if label != "" {
				gw.Labels = map[string]string{consts.SharedGatewayLabel: label}
			}
			return gw
		}

		BeforeEach(func() {
			scheme := runtime.NewScheme()
			Expect(gwv1.Install(scheme)).To(Succeed())
			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				gateway("internal", consts.SharedGatewayLabelValue), gateway("public", consts.SharedGatewayLabelDefaultValue),
				gateway("private", ""),
			).Build()
			route = newRoute(nil, forge.SharedGatewayPlaceholderRef())

			response = func() []jsonpatch.JsonPatchOperation {
				res := NewRouteMutator(cl).Handle(context.Background(), request(httpRouteKind, admissionv1.Create, route, nil))
				Expect(res.Allowed).To(BeTrue())
				return res.Patches
			}
		})

		It("should replace the placeholder with the default shared Gateway", func() {
			Expect(response()).To(ContainElements(
				And(HaveField("Path", "/spec/parentRefs/0/name"), HaveField("Value", "public")),
				And(HaveField("Path", "/spec/parentRefs/0/namespace"), HaveField("Value", "infra")),
				And(HaveField("Path", "/metadata/annotations"), HaveField("Value", HaveKeyWithValue(consts.SharedGatewayAnnotation, "infra/public"))),
			))
		})

		It("should not modify the routes not referring to the placeholder", func() {
			route = newRoute(nil, reflected)
			Expect(response()).To(BeEmpty())
		})
	})
})

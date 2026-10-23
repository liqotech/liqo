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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("ListenerSets", func() {
	const namespace = "offloaded"

	var (
		cl          client.Client
		listenerSet *gwv1.ListenerSet
		res         admission.Response
	)

	gateway := func(name, label string, ports ...gwv1.PortNumber) *gwv1.Gateway {
		gw := &gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "infra"}}
		if label != "" {
			gw.Labels = map[string]string{consts.SharedGatewayLabel: label}
		}
		for _, port := range ports {
			gw.Spec.Listeners = append(gw.Spec.Listeners, gwv1.Listener{Name: gwv1.SectionName(fmt.Sprint(port)), Port: port})
		}
		return gw
	}
	namespaceObj := func(name, clusterID string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{consts.RemoteClusterID: clusterID}}}
	}
	listener := func(name string, port gwv1.PortNumber, certNamespace *gwv1.Namespace) gwv1.ListenerEntry {
		return gwv1.ListenerEntry{Name: gwv1.SectionName(name), Port: port, Protocol: gwv1.HTTPSProtocolType,
			TLS: &gwv1.ListenerTLSConfig{CertificateRefs: []gwv1.SecretObjectReference{{Name: "tls", Namespace: certNamespace}}}}
	}
	newListenerSet := func(parent gwv1.ParentGatewayReference, listeners ...gwv1.ListenerEntry) *gwv1.ListenerSet {
		return &gwv1.ListenerSet{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
			Spec:       gwv1.ListenerSetSpec{ParentRef: parent, Listeners: listeners},
		}
	}
	public := gwv1.ParentGatewayReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")}

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(gwv1.Install(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			gateway("public", consts.SharedGatewayLabelDefaultValue, 80, 443), gateway("private", "", 443),
			namespaceObj(namespace, "consumer"), namespaceObj("sibling", "consumer"), namespaceObj("stranger", "other"),
		).Build()
	})

	Describe("the MutateListenerSet function", func() {
		var (
			shared   *types.NamespacedName
			modified bool
		)

		BeforeEach(func() {
			shared = &types.NamespacedName{Namespace: "infra", Name: "public"}
			listenerSet = newListenerSet(forge.SharedGatewayPlaceholderParentRef(), listener("https", 443, nil))
		})
		JustBeforeEach(func() { modified = MutateListenerSet(listenerSet, shared) })

		When("the ListenerSet refers to the placeholder", func() {
			It("should replace it with the shared Gateway, and annotate the ListenerSet", func() {
				Expect(modified).To(BeTrue())
				Expect(listenerSet.Spec.ParentRef).To(Equal(public))
				Expect(listenerSet.Annotations).To(HaveKeyWithValue(consts.SharedGatewayAnnotation, "infra/public"))
			})
		})

		When("the placeholder is applied again, preserving the namespace set by the previous mutation", func() {
			BeforeEach(func() {
				// The parent is a structure, hence the server-side apply of the placeholder (without namespace) by the
				// consumer cluster preserves the namespace set when the placeholder was replaced the first time.
				parent := forge.SharedGatewayPlaceholderParentRef()
				parent.Namespace = ptr.To[gwv1.Namespace]("infra")
				listenerSet = newListenerSet(parent, listener("https", 443, nil))
				listenerSet.Annotations = map[string]string{consts.SharedGatewayAnnotation: "infra/public"}
			})

			It("should replace it with the shared Gateway again", func() {
				Expect(modified).To(BeTrue())
				Expect(listenerSet.Spec.ParentRef).To(Equal(public))
			})
		})

		When("the ListenerSet has already been mutated", func() {
			BeforeEach(func() {
				listenerSet = newListenerSet(public, listener("https", 443, nil))
				listenerSet.Annotations = map[string]string{consts.SharedGatewayAnnotation: "infra/public"}
			})

			It("should not modify it", func() { Expect(modified).To(BeFalse()) })
		})

		When("the ListenerSet no longer refers to the shared Gateway it is annotated with", func() {
			BeforeEach(func() {
				listenerSet = newListenerSet(gwv1.ParentGatewayReference{Name: "liqo"}, listener("https", 443, nil))
				listenerSet.Annotations = map[string]string{consts.SharedGatewayAnnotation: "infra/public"}
			})

			It("should remove the annotation", func() {
				Expect(modified).To(BeTrue())
				Expect(listenerSet.Annotations).ToNot(HaveKey(consts.SharedGatewayAnnotation))
			})
		})
	})

	Describe("the mutating webhook", func() {
		handle := func() {
			res = NewListenerSetMutator(cl).Handle(context.Background(), request(listenerSetKind, admissionv1.Create, listenerSet, nil))
		}

		It("should replace the placeholder with the default shared Gateway", func() {
			listenerSet = newListenerSet(forge.SharedGatewayPlaceholderParentRef(), listener("https", 443, nil))
			handle()
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(ContainElements(
				And(HaveField("Path", "/spec/parentRef/name"), HaveField("Value", "public")),
				And(HaveField("Path", "/spec/parentRef/namespace"), HaveField("Value", "infra")),
			))
		})

		It("should reject the ListenerSets referring to the placeholder, if no Gateway is shared", func() {
			Expect(cl.Delete(context.Background(), gateway("public", ""))).To(Succeed())
			listenerSet = newListenerSet(forge.SharedGatewayPlaceholderParentRef(), listener("https", 443, nil))
			handle()
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Message).To(ContainSubstring("no shared Gateway is offered"))
		})

		It("should not modify the ListenerSets not referring to the placeholder", func() {
			listenerSet = newListenerSet(gwv1.ParentGatewayReference{Name: "liqo"}, listener("https", 443, nil))
			handle()
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(BeEmpty())
		})
	})

	Describe("the validating webhook", func() {
		handle := func(op admissionv1.Operation, obj, old *gwv1.ListenerSet) {
			var oldObj metav1.Object
			if old != nil {
				oldObj = old
			}
			res = NewValidator(cl, []string{"envoy"}).Handle(context.Background(), request(listenerSetKind, op, obj, oldObj))
		}

		DescribeTable("the ListenerSets",
			func(allowed bool, parent gwv1.ParentGatewayReference, listeners ...gwv1.ListenerEntry) {
				handle(admissionv1.Create, newListenerSet(parent, listeners...), nil)
				Expect(res.Allowed).To(Equal(allowed), res.Result.Message)
			},
			Entry("attached to a Gateway in the same namespace, using any port", true,
				gwv1.ParentGatewayReference{Name: "liqo"}, listener("https", 8443, nil)),
			Entry("attached to a shared Gateway, using its ports", true, public, listener("https", 443, nil), listener("http", 80, nil)),
			Entry("attached to a shared Gateway, using other ports", false, public, listener("https", 8443, nil)),
			Entry("attached to a Gateway not shared", false,
				gwv1.ParentGatewayReference{Name: "private", Namespace: ptr.To[gwv1.Namespace]("infra")}, listener("https", 443, nil)),
			Entry("attached to a non-existing Gateway", false,
				gwv1.ParentGatewayReference{Name: "missing", Namespace: ptr.To[gwv1.Namespace]("infra")}, listener("https", 443, nil)),
			Entry("attached to an unsupported kind", false,
				gwv1.ParentGatewayReference{Name: "other", Kind: ptr.To[gwv1.Kind]("Other")}, listener("https", 443, nil)),
			Entry("referring to certificates offloaded by the same cluster", true,
				public, listener("https", 443, ptr.To[gwv1.Namespace]("sibling"))),
			Entry("referring to certificates offloaded by another cluster", false,
				public, listener("https", 443, ptr.To[gwv1.Namespace]("stranger"))),
			Entry("referring to certificates in a namespace not offloaded", false,
				public, listener("https", 443, ptr.To[gwv1.Namespace]("kube-system"))),
		)

		It("should validate only the parents and ports changed by updates", func() {
			// The ListenerSet was attached to a Gateway shared at that time, and it is now updated (e.g., to annotate it).
			private := gwv1.ParentGatewayReference{Name: "private", Namespace: ptr.To[gwv1.Namespace]("infra")}
			old := newListenerSet(private, listener("https", 8443, nil))
			updated := newListenerSet(private, listener("https", 8443, nil))
			updated.SetAnnotations(map[string]string{"foo": "bar"})
			handle(admissionv1.Update, updated, old)
			Expect(res.Allowed).To(BeTrue(), res.Result.Message)

			// A listener using a port not offered by the Gateway is added.
			updated.Spec.Listeners = append(updated.Spec.Listeners, listener("other", 9443, nil))
			handle(admissionv1.Update, updated, old)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Message).To(ContainSubstring(`port 9443 of listener "other" not allowed`))

			// The parent is changed to a Gateway not shared.
			updated = newListenerSet(gwv1.ParentGatewayReference{Name: "missing", Namespace: ptr.To[gwv1.Namespace]("infra")},
				listener("https", 8443, nil))
			handle(admissionv1.Update, updated, old)
			Expect(res.Allowed).To(BeFalse())
		})
	})
})

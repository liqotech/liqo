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

package forge_test

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwv1apply "sigs.k8s.io/gateway-api/applyconfiguration/apis/v1"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Gateway API routes forging", func() {
	const localNamespace = "local"

	var (
		opts     forge.GatewayAPIForgingOpts
		warnings []string
		err      error
	)

	mapper := func(local string) (string, bool) {
		switch local {
		case localNamespace:
			return "local-remote", true
		case "offloaded":
			return "offloaded-remote", true
		default:
			return "", false
		}
	}

	BeforeEach(func() {
		opts = forge.GatewayAPIForgingOpts{Mapper: mapper, SharedGateway: &types.NamespacedName{Namespace: "infra", Name: "public"}}
	})

	Describe("the RemoteRouteParentRefs function", func() {
		var (
			spec    gwv1.CommonRouteSpec
			parents []gwv1.ParentReference
		)

		JustBeforeEach(func() { parents, warnings, err = forge.RemoteRouteParentRefs(localNamespace, &spec, &opts) })

		When("the route refers to Gateways", func() {
			BeforeEach(func() {
				spec = gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{
					{Name: "edge", Namespace: ptr.To[gwv1.Namespace]("infra"), SectionName: ptr.To[gwv1.SectionName]("http")},
					{Name: "internal", Kind: ptr.To[gwv1.Kind]("Gateway"), Group: ptr.To[gwv1.Group](gwv1.GroupName)},
				}}
			})

			It("should succeed without warnings", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
			It("should replace them with a single reference to the shared gateway", func() {
				Expect(parents).To(ConsistOf(gwv1.ParentReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")}))
			})

			When("the remote cluster does not offer a shared gateway", func() {
				BeforeEach(func() { opts.SharedGateway = nil })

				It("should return a not reflectable error", func() {
					var target *forge.ErrNotReflectable
					Expect(err).To(BeAssignableToTypeOf(target))
				})
				It("should report the dropped references", func() {
					Expect(warnings).To(HaveLen(2))
					Expect(warnings[0]).To(ContainSubstring("Gateway.gateway.networking.k8s.io infra/edge"))
					Expect(warnings[1]).To(ContainSubstring("Gateway.gateway.networking.k8s.io local/internal"))
				})

				When("the route uses the default gateways", func() {
					BeforeEach(func() { spec.UseDefaultGateways = gwv1.GatewayDefaultScopeAll })

					It("should succeed with no parents", func() {
						Expect(err).ToNot(HaveOccurred())
						Expect(parents).To(BeEmpty())
					})
				})
			})
		})

		When("the route refers to reflected Gateways", func() {
			BeforeEach(func() {
				opts.IsReflectedGateway = func(namespace, name string) bool { return name == "liqo" }
				spec = gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{
					{Name: "liqo", SectionName: ptr.To[gwv1.SectionName]("https")},
					{Name: "liqo", Namespace: ptr.To[gwv1.Namespace]("offloaded"), Port: ptr.To[gwv1.PortNumber](443)},
					{Name: "liqo", Namespace: ptr.To[gwv1.Namespace]("other")},
					{Name: "edge", Namespace: ptr.To[gwv1.Namespace]("infra")},
				}}
			})

			It("should succeed without warnings", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
			It("should attach the route to the remote copies of the reflected Gateways, and to the shared one otherwise", func() {
				Expect(parents).To(ConsistOf(
					// Same namespace: the section name is preserved, and the namespace is implicitly translated.
					gwv1.ParentReference{Name: "liqo", SectionName: ptr.To[gwv1.SectionName]("https")},
					// Offloaded namespace: the namespace is translated.
					gwv1.ParentReference{Name: "liqo", Namespace: ptr.To[gwv1.Namespace]("offloaded-remote"), Port: ptr.To[gwv1.PortNumber](443)},
					// Namespace not offloaded, and Gateway of another class: replaced by the shared Gateway.
					gwv1.ParentReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")},
				))
			})
		})

		When("the route refers to Services (i.e., mesh routes)", func() {
			BeforeEach(func() {
				spec = gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{
					{Name: "same", Kind: ptr.To[gwv1.Kind]("Service"), Group: ptr.To[gwv1.Group]("")},
					{Name: "mapped", Kind: ptr.To[gwv1.Kind]("Service"), Group: ptr.To[gwv1.Group](""), Namespace: ptr.To[gwv1.Namespace]("offloaded")},
					{Name: "unmapped", Kind: ptr.To[gwv1.Kind]("Service"), Group: ptr.To[gwv1.Group](""), Namespace: ptr.To[gwv1.Namespace]("other")},
				}}
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should translate the namespaces of the references", func() {
				Expect(parents).To(ConsistOf(
					gwv1.ParentReference{Name: "same", Kind: ptr.To[gwv1.Kind]("Service"), Group: ptr.To[gwv1.Group]("")},
					gwv1.ParentReference{Name: "mapped", Kind: ptr.To[gwv1.Kind]("Service"), Group: ptr.To[gwv1.Group](""),
						Namespace: ptr.To[gwv1.Namespace]("offloaded-remote")},
				))
			})
			It("should report the references to namespaces not offloaded", func() {
				Expect(warnings).To(ConsistOf(ContainSubstring(`Service other/unmapped dropped: namespace "other" not offloaded`)))
			})
		})

		When("the route refers to unsupported kinds", func() {
			BeforeEach(func() {
				spec = gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{
					{Name: "set", Kind: ptr.To[gwv1.Kind]("ListenerSet")},
					{Name: "edge"},
				}}
			})

			It("should drop them", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(parents).To(ConsistOf(gwv1.ParentReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")}))
				Expect(warnings).To(ConsistOf(ContainSubstring("unsupported parent kind")))
			})
		})
	})

	Describe("the LocalRouteParentStatuses function", func() {
		var (
			localParents []gwv1.ParentReference
			remote       []gwv1.RouteParentStatus
			local        []gwv1.RouteParentStatus
		)

		accepted := func(status metav1.ConditionStatus) []metav1.Condition {
			return []metav1.Condition{{Type: string(gwv1.RouteConditionAccepted), Status: status, Reason: "Reason"}}
		}

		BeforeEach(func() {
			opts.IsReflectedGateway = func(_, name string) bool { return name == "liqo" }
			localParents = []gwv1.ParentReference{
				{Name: "liqo", SectionName: ptr.To[gwv1.SectionName]("https")},
				{Name: "edge", Namespace: ptr.To[gwv1.Namespace]("infra")},
				{Name: "internal", Namespace: ptr.To[gwv1.Namespace]("infra")},
				{Name: "svc", Kind: ptr.To[gwv1.Kind]("Service"), Group: ptr.To[gwv1.Group](""), Namespace: ptr.To[gwv1.Namespace]("other")},
			}
			remote = []gwv1.RouteParentStatus{
				// The reference to the reflected Gateway, as specified in the remote route.
				{ParentRef: gwv1.ParentReference{Name: "liqo", SectionName: ptr.To[gwv1.SectionName]("https")},
					ControllerName: "example.com/controller", Conditions: accepted(metav1.ConditionTrue)},
				// The reference to the shared Gateway, with the default values explicitly set by the remote controller.
				{ParentRef: gwv1.ParentReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra"),
					Group: ptr.To[gwv1.Group](gwv1.GroupName), Kind: ptr.To[gwv1.Kind]("Gateway")},
					ControllerName: "example.com/controller", Conditions: accepted(metav1.ConditionFalse)},
				// A reference not derived from any local parent.
				{ParentRef: gwv1.ParentReference{Name: "unknown"}, ControllerName: "example.com/controller", Conditions: accepted(metav1.ConditionTrue)},
			}
		})

		JustBeforeEach(func() {
			local = forge.LocalRouteParentStatuses(localNamespace, "remote", localParents, remote, &opts)
		})

		It("should translate the remote parents to the local ones", func() {
			Expect(local).To(ConsistOf(
				gwv1.RouteParentStatus{ParentRef: localParents[0], ControllerName: "example.com/controller", Conditions: accepted(metav1.ConditionTrue)},
				// Both local parents replaced by the shared Gateway inherit its status.
				gwv1.RouteParentStatus{ParentRef: localParents[1], ControllerName: "example.com/controller", Conditions: accepted(metav1.ConditionFalse)},
				gwv1.RouteParentStatus{ParentRef: localParents[2], ControllerName: "example.com/controller", Conditions: accepted(metav1.ConditionFalse)},
			))
		})
	})

	Describe("the RemoteBackendObjectReference function", func() {
		var (
			ref     gwv1.BackendObjectReference
			warning string
		)

		JustBeforeEach(func() { warning = forge.RemoteBackendObjectReference(localNamespace, &ref, mapper) })

		When("the reference has no namespace", func() {
			BeforeEach(func() { ref = gwv1.BackendObjectReference{Name: "svc"} })

			It("should keep it unchanged", func() {
				Expect(warning).To(BeEmpty())
				Expect(ref).To(Equal(gwv1.BackendObjectReference{Name: "svc"}))
			})
		})

		When("the reference points to an offloaded namespace", func() {
			BeforeEach(func() { ref = gwv1.BackendObjectReference{Name: "svc", Namespace: ptr.To[gwv1.Namespace]("offloaded")} })

			It("should translate the namespace", func() {
				Expect(warning).To(BeEmpty())
				Expect(ref.Namespace).To(PointTo(BeEquivalentTo("offloaded-remote")))
			})
		})

		When("the reference points to a namespace not offloaded", func() {
			BeforeEach(func() { ref = gwv1.BackendObjectReference{Name: "svc", Namespace: ptr.To[gwv1.Namespace]("other")} })

			It("should return a warning", func() { Expect(warning).To(ContainSubstring(`namespace "other" not offloaded`)) })
		})

		When("the reference points to an unsupported kind", func() {
			BeforeEach(func() {
				ref = gwv1.BackendObjectReference{Name: "svc", Kind: ptr.To[gwv1.Kind]("ServiceImport"), Group: ptr.To[gwv1.Group]("multicluster.x-k8s.io")}
			})

			It("should return a warning", func() {
				Expect(warning).To(ContainSubstring("ServiceImport.multicluster.x-k8s.io local/svc dropped: unsupported backend kind"))
			})
		})
	})

	Describe("the RemoteHTTPRoute function", func() {
		var (
			local    gwv1.HTTPRoute
			backends []gwv1.HTTPBackendRef
			filters  []gwv1.HTTPRouteFilter
			remote   *gwv1apply.HTTPRouteApplyConfiguration
		)

		backend := func(name string, namespace *gwv1.Namespace, weight int32) gwv1.HTTPBackendRef {
			return gwv1.HTTPBackendRef{BackendRef: gwv1.BackendRef{
				BackendObjectReference: gwv1.BackendObjectReference{Name: gwv1.ObjectName(name), Namespace: namespace, Port: ptr.To[gwv1.PortNumber](80)},
				Weight:                 ptr.To(weight),
			}}
		}

		remoteSpec := func() gwv1.HTTPRouteSpec {
			var spec gwv1.HTTPRouteSpec
			data, errMarshal := json.Marshal(remote.Spec)
			Expect(errMarshal).ToNot(HaveOccurred())
			Expect(json.Unmarshal(data, &spec)).To(Succeed())
			return spec
		}

		BeforeEach(func() {
			backends = []gwv1.HTTPBackendRef{backend("svc", nil, 90), backend("other", ptr.To[gwv1.Namespace]("other"), 10)}
			filters = []gwv1.HTTPRouteFilter{{
				Type:                  gwv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gwv1.HTTPHeaderFilter{Set: []gwv1.HTTPHeader{{Name: "x-liqo", Value: "true"}}},
			}}
		})

		JustBeforeEach(func() {
			local = gwv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Name: "route", Namespace: localNamespace,
					Labels: map[string]string{"foo": "bar"}, Annotations: map[string]string{"bar": "baz"}},
				Spec: gwv1.HTTPRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{{Name: "edge", Namespace: ptr.To[gwv1.Namespace]("infra")}}},
					Hostnames:       []gwv1.Hostname{"shop.example.com"},
					Rules: []gwv1.HTTPRouteRule{{
						Matches:     []gwv1.HTTPRouteMatch{{Path: &gwv1.HTTPPathMatch{Value: ptr.To("/api")}}},
						Filters:     filters,
						BackendRefs: backends,
					}},
				},
			}
			forgingOpts := forge.NewEmptyForgingOpts()
			remote, warnings, err = forge.RemoteHTTPRoute(&local, "remote", &opts, &forgingOpts)
		})

		When("the route can be translated", func() {
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should report the dropped backend", func() {
				Expect(warnings).To(ConsistOf(ContainSubstring("backendRef Service other/other dropped")))
			})
			It("should correctly forge the metadata", func() {
				Expect(remote.Name).To(PointTo(Equal("route")))
				Expect(remote.Namespace).To(PointTo(Equal("remote")))
				Expect(remote.Labels).To(HaveKeyWithValue("foo", "bar"))
				Expect(remote.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
				Expect(remote.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
				Expect(remote.Annotations).To(HaveKeyWithValue("bar", "baz"))
			})
			It("should correctly forge the spec", func() {
				spec := remoteSpec()
				Expect(spec.ParentRefs).To(ConsistOf(gwv1.ParentReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")}))
				Expect(spec.Hostnames).To(ConsistOf(gwv1.Hostname("shop.example.com")))
				Expect(spec.Rules).To(HaveLen(1))
				Expect(spec.Rules[0].Matches).To(Equal(local.Spec.Rules[0].Matches))
				Expect(spec.Rules[0].Filters).To(Equal(filters))
				Expect(spec.Rules[0].BackendRefs).To(ConsistOf(backend("svc", nil, 90)))
			})
			It("should not mutate the local route", func() {
				Expect(local.Spec.Rules[0].BackendRefs).To(HaveLen(2))
				Expect(local.Spec.ParentRefs[0].Name).To(BeEquivalentTo("edge"))
			})
		})

		When("the route has an extension filter", func() {
			BeforeEach(func() {
				filters = append(filters, gwv1.HTTPRouteFilter{Type: gwv1.HTTPRouteFilterExtensionRef,
					ExtensionRef: &gwv1.LocalObjectReference{Group: "example.com", Kind: "Policy", Name: "auth"}})
			})

			It("should not be reflectable", func() {
				var target *forge.ErrNotReflectable
				Expect(errors.As(err, &target)).To(BeTrue())
				Expect(err).To(MatchError(ContainSubstring(`unsupported ExtensionRef filter "auth"`)))
			})
		})

		When("the route has an external authentication filter", func() {
			var namespace string

			JustBeforeEach(func() {
				local.Spec.Rules[0].Filters = []gwv1.HTTPRouteFilter{{Type: gwv1.HTTPRouteFilterExternalAuth, ExternalAuth: &gwv1.HTTPExternalAuthFilter{
					ExternalAuthProtocol: gwv1.HTTPRouteExternalAuthHTTPProtocol,
					BackendRef:           gwv1.BackendObjectReference{Name: "auth", Namespace: ptr.To(gwv1.Namespace(namespace))},
				}}}
				forgingOpts := forge.NewEmptyForgingOpts()
				remote, warnings, err = forge.RemoteHTTPRoute(&local, "remote", &opts, &forgingOpts)
			})

			When("the authentication service is in an offloaded namespace", func() {
				BeforeEach(func() { namespace = "offloaded" })

				It("should translate the filter", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(remoteSpec().Rules[0].Filters[0].ExternalAuth.BackendRef.Namespace).To(PointTo(BeEquivalentTo("offloaded-remote")))
				})
			})

			When("the authentication service is in a namespace not offloaded", func() {
				BeforeEach(func() { namespace = "other" })

				It("should not be reflectable, as the route would be exposed without authentication", func() {
					var target *forge.ErrNotReflectable
					Expect(errors.As(err, &target)).To(BeTrue())
					Expect(err).To(MatchError(ContainSubstring("cannot translate the ExternalAuth filter")))
				})
			})
		})

		When("the route has a mirror filter towards a namespace not offloaded", func() {
			BeforeEach(func() {
				filters = append(filters, gwv1.HTTPRouteFilter{Type: gwv1.HTTPRouteFilterRequestMirror, RequestMirror: &gwv1.HTTPRequestMirrorFilter{
					BackendRef: gwv1.BackendObjectReference{Name: "mirror", Namespace: ptr.To[gwv1.Namespace]("other")},
				}})
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should drop the filter", func() {
				Expect(remoteSpec().Rules[0].Filters).To(HaveLen(1))
				Expect(warnings).To(ContainElement(ContainSubstring("RequestMirror filter dropped")))
			})
		})
	})

	Describe("the RemoteGateway function", func() {
		var (
			local  gwv1.Gateway
			remote *gwv1apply.GatewayApplyConfiguration
		)

		listener := func(name string, from gwv1.FromNamespaces, certNamespace *gwv1.Namespace) gwv1.Listener {
			l := gwv1.Listener{
				Name: gwv1.SectionName(name), Port: 443, Protocol: gwv1.HTTPSProtocolType,
				TLS: &gwv1.ListenerTLSConfig{Mode: ptr.To(gwv1.TLSModeTerminate),
					CertificateRefs: []gwv1.SecretObjectReference{{Name: "cert", Namespace: certNamespace}}},
				AllowedRoutes: &gwv1.AllowedRoutes{Namespaces: &gwv1.RouteNamespaces{From: ptr.To(from)}},
			}
			if from == gwv1.NamespacesFromSelector {
				l.AllowedRoutes.Namespaces.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"team": "shop"}}
			}
			return l
		}

		remoteSpec := func() gwv1.GatewaySpec {
			var spec gwv1.GatewaySpec
			data, errMarshal := json.Marshal(remote.Spec)
			Expect(errMarshal).ToNot(HaveOccurred())
			Expect(json.Unmarshal(data, &spec)).To(Succeed())
			return spec
		}

		BeforeEach(func() {
			opts.VirtualGatewayClass = "liqo"
			opts.RemoteGatewayClass = "envoy"
			local = gwv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: localNamespace, Labels: map[string]string{"foo": "bar"}},
				Spec: gwv1.GatewaySpec{
					GatewayClassName: "liqo",
					Listeners: []gwv1.Listener{
						listener("same", gwv1.NamespacesFromSame, nil),
						listener("all", gwv1.NamespacesFromAll, ptr.To[gwv1.Namespace]("offloaded")),
						listener("selector", gwv1.NamespacesFromSelector, nil),
					},
				},
			}
		})

		JustBeforeEach(func() {
			forgingOpts := forge.NewEmptyForgingOpts()
			remote, warnings, err = forge.RemoteGateway(&local, "remote", &opts, &forgingOpts)
		})

		When("the Gateway belongs to the virtual class", func() {
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should correctly forge the metadata", func() {
				Expect(remote.Name).To(PointTo(Equal("web")))
				Expect(remote.Namespace).To(PointTo(Equal("remote")))
				Expect(remote.Labels).To(HaveKeyWithValue("foo", "bar"))
				Expect(remote.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			})
			It("should use the GatewayClass offered by the remote cluster", func() {
				Expect(remoteSpec().GatewayClassName).To(BeEquivalentTo("envoy"))
			})
			It("should translate the namespaces of the certificates", func() {
				spec := remoteSpec()
				Expect(spec.Listeners[0].TLS.CertificateRefs[0].Namespace).To(BeNil())
				Expect(spec.Listeners[1].TLS.CertificateRefs[0].Namespace).To(PointTo(BeEquivalentTo("offloaded-remote")))
			})
			It("should restrict the routes attached from all namespaces to the remote namespaces of the local cluster", func() {
				namespaces := remoteSpec().Listeners[1].AllowedRoutes.Namespaces
				Expect(namespaces.From).To(PointTo(Equal(gwv1.NamespacesFromSelector)))
				Expect(namespaces.Selector.MatchLabels).To(Equal(map[string]string{consts.RemoteClusterID: string(LocalClusterID)}))
			})
			It("should restrict the routes attached through label selectors to the same namespace", func() {
				namespaces := remoteSpec().Listeners[2].AllowedRoutes.Namespaces
				Expect(namespaces.From).To(PointTo(Equal(gwv1.NamespacesFromSame)))
				Expect(namespaces.Selector).To(BeNil())
				Expect(warnings).To(ConsistOf(ContainSubstring(`listener "selector" restricted to routes in the same namespace`)))
			})
		})

		When("the Gateway belongs to another class", func() {
			BeforeEach(func() { local.Spec.GatewayClassName = "envoy" })

			It("should return a not managed error", func() { Expect(err).To(MatchError(forge.ErrNotManaged)) })
		})

		When("the remote cluster does not offer any GatewayClass", func() {
			BeforeEach(func() { opts.RemoteGatewayClass = "" })

			It("should not be reflectable", func() {
				Expect(err).To(MatchError(ContainSubstring("no GatewayClass offered by the remote cluster")))
			})
		})

		When("the Gateway has a TLS configuration", func() {
			BeforeEach(func() { local.Spec.TLS = &gwv1.GatewayTLSConfig{Frontend: &gwv1.FrontendTLSConfig{}} })

			It("should not be reflectable, as it might enforce the authentication of clients", func() {
				var target *forge.ErrNotReflectable
				Expect(errors.As(err, &target)).To(BeTrue())
			})
		})

		When("the Gateway has fields referring to the local cluster", func() {
			BeforeEach(func() {
				local.Spec.Addresses = []gwv1.GatewaySpecAddress{{Value: "10.0.0.1"}}
				local.Spec.Infrastructure = &gwv1.GatewayInfrastructure{
					Labels:        map[gwv1.LabelKey]gwv1.LabelValue{"foo": "bar"},
					ParametersRef: &gwv1.LocalParametersReference{Group: "example.com", Kind: "Config", Name: "params"},
				}
				local.Spec.DefaultScope = gwv1.GatewayDefaultScopeAll
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should drop them", func() {
				spec := remoteSpec()
				Expect(spec.Addresses).To(BeEmpty())
				Expect(spec.Infrastructure.ParametersRef).To(BeNil())
				Expect(spec.Infrastructure.Labels).To(HaveKeyWithValue(gwv1.LabelKey("foo"), gwv1.LabelValue("bar")))
				Expect(spec.DefaultScope).To(BeEmpty())
			})
			It("should report the dropped fields", func() {
				Expect(warnings).To(ContainElements(ContainSubstring("addresses dropped"),
					ContainSubstring("parametersRef dropped"), ContainSubstring("defaultScope dropped")))
			})
		})

		When("a listener refers to certificates in namespaces not offloaded", func() {
			BeforeEach(func() {
				local.Spec.Listeners = append(local.Spec.Listeners, listener("other", gwv1.NamespacesFromSame, ptr.To[gwv1.Namespace]("other")))
			})

			It("should drop the listener", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(remoteSpec().Listeners).To(HaveLen(3))
				Expect(warnings).To(ContainElement(ContainSubstring(`listener "other" dropped: certificateRef Secret other/cert refers to namespace "other"`)))
			})

			When("no listener can be translated", func() {
				BeforeEach(func() { local.Spec.Listeners = local.Spec.Listeners[3:] })

				It("should not be reflectable", func() {
					Expect(err).To(MatchError(ContainSubstring("no listener can be translated")))
				})
			})
		})
	})

	Describe("the RemoteReferenceGrant function", func() {
		var (
			local  gwv1.ReferenceGrant
			remote *gwv1apply.ReferenceGrantApplyConfiguration
		)

		BeforeEach(func() {
			local = gwv1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{Name: "grant", Namespace: localNamespace},
				Spec: gwv1.ReferenceGrantSpec{
					From: []gwv1.ReferenceGrantFrom{
						{Group: gwv1.GroupName, Kind: "HTTPRoute", Namespace: "offloaded"},
						{Group: gwv1.GroupName, Kind: "HTTPRoute", Namespace: "other"},
					},
					To: []gwv1.ReferenceGrantTo{{Kind: "Service", Name: ptr.To[gwv1.ObjectName]("svc")}},
				},
			}
		})

		JustBeforeEach(func() {
			forgingOpts := forge.NewEmptyForgingOpts()
			remote, warnings, err = forge.RemoteReferenceGrant(&local, "remote", &opts, &forgingOpts)
		})

		It("should succeed without warnings", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
		It("should translate the namespaces offloaded, and drop the others", func() {
			var spec gwv1.ReferenceGrantSpec
			data, errMarshal := json.Marshal(remote.Spec)
			Expect(errMarshal).ToNot(HaveOccurred())
			Expect(json.Unmarshal(data, &spec)).To(Succeed())

			Expect(spec.From).To(ConsistOf(gwv1.ReferenceGrantFrom{Group: gwv1.GroupName, Kind: "HTTPRoute", Namespace: "offloaded-remote"}))
			Expect(spec.To).To(Equal(local.Spec.To))
		})

		When("none of the namespaces is offloaded", func() {
			BeforeEach(func() { local.Spec.From = local.Spec.From[1:] })

			It("should return a not managed error", func() { Expect(err).To(MatchError(forge.ErrNotManaged)) })
		})
	})

	Describe("the RemoteGRPCRoute function", func() {
		var (
			local  gwv1.GRPCRoute
			remote *gwv1apply.GRPCRouteApplyConfiguration
		)

		BeforeEach(func() {
			local = gwv1.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{Name: "route", Namespace: localNamespace},
				Spec: gwv1.GRPCRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{{Name: "edge"}}},
					Rules: []gwv1.GRPCRouteRule{{
						BackendRefs: []gwv1.GRPCBackendRef{{BackendRef: gwv1.BackendRef{BackendObjectReference: gwv1.BackendObjectReference{
							Name: "svc", Namespace: ptr.To[gwv1.Namespace]("offloaded"), Port: ptr.To[gwv1.PortNumber](9000)}}}},
					}},
				},
			}
		})

		JustBeforeEach(func() {
			forgingOpts := forge.NewEmptyForgingOpts()
			remote, warnings, err = forge.RemoteGRPCRoute(&local, "remote", &opts, &forgingOpts)
		})

		It("should succeed without warnings", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
		It("should correctly forge the spec", func() {
			var spec gwv1.GRPCRouteSpec
			data, errMarshal := json.Marshal(remote.Spec)
			Expect(errMarshal).ToNot(HaveOccurred())
			Expect(json.Unmarshal(data, &spec)).To(Succeed())

			Expect(spec.ParentRefs).To(ConsistOf(gwv1.ParentReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")}))
			Expect(spec.Rules[0].BackendRefs[0].Namespace).To(PointTo(BeEquivalentTo("offloaded-remote")))
		})
	})
})

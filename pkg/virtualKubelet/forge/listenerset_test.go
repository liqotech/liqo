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

var _ = Describe("Gateway API ListenerSets forging", func() {
	const localNamespace = "local"

	var (
		opts     forge.GatewayAPIForgingOpts
		gateways map[types.NamespacedName]*gwv1.Gateway
		local    gwv1.ListenerSet
		remote   *gwv1apply.ListenerSetApplyConfiguration
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

	gateway := func(namespace, name, class string, from *gwv1.FromNamespaces) *gwv1.Gateway {
		gw := &gwv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       gwv1.GatewaySpec{GatewayClassName: gwv1.ObjectName(class)},
		}
		if from != nil {
			gw.Spec.AllowedListeners = &gwv1.AllowedListeners{Namespaces: &gwv1.ListenerNamespaces{From: from}}
		}
		return gw
	}

	listener := func(name string, certNamespace *gwv1.Namespace, from gwv1.FromNamespaces) gwv1.ListenerEntry {
		return gwv1.ListenerEntry{
			Name: gwv1.SectionName(name), Hostname: ptr.To[gwv1.Hostname]("app.example.com"), Port: 443, Protocol: gwv1.HTTPSProtocolType,
			TLS: &gwv1.ListenerTLSConfig{Mode: ptr.To(gwv1.TLSModeTerminate),
				CertificateRefs: []gwv1.SecretObjectReference{{Name: "app-tls", Namespace: certNamespace}}},
			AllowedRoutes: &gwv1.AllowedRoutes{Namespaces: &gwv1.RouteNamespaces{From: ptr.To(from)}},
		}
	}

	remoteSpec := func() gwv1.ListenerSetSpec {
		var spec gwv1.ListenerSetSpec
		data, errMarshal := json.Marshal(remote.Spec)
		Expect(errMarshal).ToNot(HaveOccurred())
		Expect(json.Unmarshal(data, &spec)).To(Succeed())
		return spec
	}

	BeforeEach(func() {
		gateways = map[types.NamespacedName]*gwv1.Gateway{}
		add := func(gw *gwv1.Gateway) { gateways[types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name}] = gw }
		add(gateway(localNamespace, "web", "liqo", ptr.To(gwv1.NamespacesFromSame)))
		add(gateway(localNamespace, "closed", "liqo", nil))
		add(gateway(localNamespace, "envoy", "envoy", ptr.To(gwv1.NamespacesFromAll)))
		add(gateway("offloaded", "web", "liqo", ptr.To(gwv1.NamespacesFromAll)))
		add(gateway("offloaded", "selector", "liqo", ptr.To(gwv1.NamespacesFromSelector)))
		add(gateway("other", "web", "liqo", ptr.To(gwv1.NamespacesFromAll)))

		opts = forge.GatewayAPIForgingOpts{
			Mapper: mapper, SharedGatewayEnabled: true, VirtualGatewayClass: "liqo", RemoteGatewayClass: "envoy",
			LocalGateway: func(namespace, name string) *gwv1.Gateway {
				return gateways[types.NamespacedName{Namespace: namespace, Name: name}]
			},
		}

		local = gwv1.ListenerSet{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: localNamespace, Labels: map[string]string{"foo": "bar"},
				Annotations: map[string]string{
					"cert-manager.io/cluster-issuer": "letsencrypt",
					consts.SharedGatewayAnnotation:   "infra/public",
				}},
			Spec: gwv1.ListenerSetSpec{
				ParentRef: gwv1.ParentGatewayReference{Name: "web"},
				Listeners: []gwv1.ListenerEntry{
					listener("same", nil, gwv1.NamespacesFromSame),
					listener("all", ptr.To[gwv1.Namespace]("offloaded"), gwv1.NamespacesFromAll),
					listener("selector", nil, gwv1.NamespacesFromSelector),
				},
			},
		}
	})

	JustBeforeEach(func() {
		forgingOpts := forge.NewEmptyForgingOpts()
		remote, warnings, err = forge.RemoteListenerSet(&local, "remote", &opts, &forgingOpts)
	})

	When("the ListenerSet is attached to a reflected Gateway", func() {
		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should be mapped as its Gateway", func() { Expect(forge.MapListenerSet(&local, &opts)).To(Equal(forge.GatewayReflected)) })
		It("should correctly forge the metadata", func() {
			Expect(remote.Name).To(PointTo(Equal("app")))
			Expect(remote.Namespace).To(PointTo(Equal("remote")))
			Expect(remote.Labels).To(HaveKeyWithValue("foo", "bar"))
			Expect(remote.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
		})
		It("should preserve the annotations, except for the ones set by the remote cluster", func() {
			Expect(remote.Annotations).To(HaveKeyWithValue("cert-manager.io/cluster-issuer", "letsencrypt"))
			Expect(remote.Annotations).ToNot(HaveKey(consts.SharedGatewayAnnotation))
		})
		It("should preserve the parent, implicitly translating its namespace", func() {
			Expect(remoteSpec().ParentRef).To(Equal(gwv1.ParentGatewayReference{Name: "web"}))
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
			Expect(remoteSpec().Listeners[2].AllowedRoutes.Namespaces.From).To(PointTo(Equal(gwv1.NamespacesFromSame)))
			Expect(warnings).To(ConsistOf(ContainSubstring(`listener "selector" restricted to routes in the same namespace`)))
		})

		When("the Gateway is in another offloaded namespace", func() {
			BeforeEach(func() { local.Spec.ParentRef.Namespace = ptr.To[gwv1.Namespace]("offloaded") })

			It("should translate the namespace of the parent", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(remoteSpec().ParentRef.Namespace).To(PointTo(BeEquivalentTo("offloaded-remote")))
			})
		})

		When("the Gateway is in a namespace not offloaded", func() {
			BeforeEach(func() { local.Spec.ParentRef.Namespace = ptr.To[gwv1.Namespace]("other") })

			It("should not be reflectable", func() {
				var target *forge.ErrNotReflectable
				Expect(err).To(BeAssignableToTypeOf(target))
				Expect(err).To(MatchError(ContainSubstring(`the namespace "other" of the parent Gateway is not offloaded`)))
			})
		})
	})

	When("the ListenerSet is attached to a Gateway mapped to the shared one", func() {
		BeforeEach(func() { opts.RemoteGatewayClass = "" })

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should be mapped as its Gateway", func() { Expect(forge.MapListenerSet(&local, &opts)).To(Equal(forge.GatewayShared)) })
		It("should be attached to the placeholder of the shared Gateway", func() {
			Expect(remoteSpec().ParentRef).To(Equal(forge.SharedGatewayPlaceholderParentRef()))
			Expect(forge.IsSharedGatewayPlaceholderParent(ptr.To(remoteSpec().ParentRef))).To(BeTrue())
		})

		When("the remote cluster does not offer a shared Gateway either", func() {
			BeforeEach(func() { opts.SharedGatewayEnabled = false })

			It("should not be reflectable", func() {
				Expect(err).To(MatchError(ContainSubstring("the parent Gateway is neither reflected nor mapped to the shared Gateway")))
			})
		})
	})

	DescribeTable("the ListenerSets not managed by the reflection",
		func(parent gwv1.ParentGatewayReference) {
			local.Spec.ParentRef = parent
			forgingOpts := forge.NewEmptyForgingOpts()
			_, _, err = forge.RemoteListenerSet(&local, "remote", &opts, &forgingOpts)
			Expect(err).To(MatchError(forge.ErrNotManaged))
			Expect(forge.MapListenerSet(&local, &opts)).To(Equal(forge.GatewayNotReflected))
		},
		Entry("attached to a Gateway of another class", gwv1.ParentGatewayReference{Name: "envoy"}),
		Entry("attached to a Gateway not existing", gwv1.ParentGatewayReference{Name: "missing"}),
		Entry("attached to a Gateway not allowing ListenerSets", gwv1.ParentGatewayReference{Name: "closed"}),
		Entry("attached to a Gateway allowing ListenerSets through label selectors, from another namespace",
			gwv1.ParentGatewayReference{Name: "selector", Namespace: ptr.To[gwv1.Namespace]("offloaded")}),
		Entry("attached to an object other than a Gateway",
			gwv1.ParentGatewayReference{Name: "web", Kind: ptr.To[gwv1.Kind]("Other"), Group: ptr.To[gwv1.Group]("example.com")}),
	)

	When("a listener refers to certificates in namespaces not offloaded", func() {
		BeforeEach(func() {
			local.Spec.Listeners = append(local.Spec.Listeners, listener("other", ptr.To[gwv1.Namespace]("other"), gwv1.NamespacesFromSame))
		})

		It("should drop the listener", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(remoteSpec().Listeners).To(HaveLen(3))
			Expect(warnings).To(ContainElement(ContainSubstring(`listener "other" dropped: certificateRef Secret other/app-tls refers to namespace "other"`)))
		})

		When("no listener can be translated", func() {
			BeforeEach(func() { local.Spec.Listeners = local.Spec.Listeners[3:] })

			It("should not be reflectable", func() {
				Expect(err).To(MatchError(ContainSubstring("no listener can be translated")))
			})
		})
	})
})

var _ = Describe("The ListenerSetAllowed function", func() {
	gateway := func(allowed *gwv1.AllowedListeners) *gwv1.Gateway {
		return &gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "shop"}, Spec: gwv1.GatewaySpec{AllowedListeners: allowed}}
	}
	from := func(value gwv1.FromNamespaces) *gwv1.AllowedListeners {
		return &gwv1.AllowedListeners{Namespaces: &gwv1.ListenerNamespaces{From: ptr.To(value)}}
	}

	DescribeTable("should return whether the Gateway allows the ListenerSets in the given namespace",
		func(allowed *gwv1.AllowedListeners, namespace string, expected bool) {
			Expect(forge.ListenerSetAllowed(gateway(allowed), namespace)).To(Equal(expected))
		},
		Entry("default (none)", nil, "shop", false),
		Entry("none", from(gwv1.NamespacesFromNone), "shop", false),
		Entry("same namespace, from the same namespace", from(gwv1.NamespacesFromSame), "shop", true),
		Entry("same namespace, from another namespace", from(gwv1.NamespacesFromSame), "other", false),
		Entry("all namespaces", from(gwv1.NamespacesFromAll), "other", true),
		Entry("label selector, from the same namespace", from(gwv1.NamespacesFromSelector), "shop", true),
		Entry("label selector, from another namespace (conservatively denied)", from(gwv1.NamespacesFromSelector), "other", false),
	)
})

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

package gatewayapi_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/fake"
	k8stesting "k8s.io/client-go/testing"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/liqotech/liqo/pkg/utils/gatewayapi"
)

var _ = Describe("Gateway API resources discovery", func() {
	var (
		client       *fake.FakeDiscovery
		availability gatewayapi.Availability
		err          error
	)

	BeforeEach(func() { client = &fake.FakeDiscovery{Fake: &k8stesting.Fake{}} })
	JustBeforeEach(func() { availability, err = gatewayapi.Detect(client) })

	When("the Gateway API group version is not served", func() {
		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should report all the resources as unavailable", func() {
			for _, gvr := range gatewayapi.Resources {
				Expect(availability.Has(gvr)).To(BeFalse(), gvr.String())
			}
		})
	})

	When("only some Gateway API resources are served", func() {
		BeforeEach(func() {
			client.Resources = []*metav1.APIResourceList{{
				GroupVersion: gwv1.GroupVersion.String(),
				APIResources: []metav1.APIResource{{Name: "gateways"}, {Name: "gateways/status"}, {Name: "httproutes"}},
			}}
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should report the served resources as available", func() {
			Expect(availability.Has(gatewayapi.GatewaysGVR)).To(BeTrue())
			Expect(availability.Has(gatewayapi.HTTPRoutesGVR)).To(BeTrue())
		})
		It("should report the missing resources as unavailable", func() {
			Expect(availability.Has(gatewayapi.GRPCRoutesGVR)).To(BeFalse())
			Expect(availability.Has(gatewayapi.ReferenceGrantsGVR)).To(BeFalse())
		})
	})

	When("the discovery fails", func() {
		BeforeEach(func() {
			client.AddReactor("get", "resource", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("connection refused")
			})
		})

		It("should return an error", func() { Expect(err).To(MatchError(ContainSubstring("connection refused"))) })
	})
})

var _ = Describe("Unsupported Gateway API routes discovery", func() {
	var (
		client *fake.FakeDiscovery
		routes []gatewayapi.UnsupportedRoute
		err    error
	)

	BeforeEach(func() { client = &fake.FakeDiscovery{Fake: &k8stesting.Fake{}} })
	JustBeforeEach(func() { routes, err = gatewayapi.DetectUnsupportedRoutes(client) })

	When("the Gateway API group is not served", func() {
		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should return no routes", func() { Expect(routes).To(BeEmpty()) })
	})

	When("the routes are served in different versions", func() {
		BeforeEach(func() {
			// The fake discovery returns the group versions in order, the first one being the preferred.
			client.Resources = []*metav1.APIResourceList{{
				GroupVersion: gwv1.GroupVersion.String(),
				APIResources: []metav1.APIResource{{Name: "httproutes", Kind: "HTTPRoute"}, {Name: "tlsroutes", Kind: "TLSRoute"}},
			}, {
				GroupVersion: "gateway.networking.k8s.io/v1alpha2",
				APIResources: []metav1.APIResource{
					{Name: "tcproutes", Kind: "TCPRoute"}, {Name: "tcproutes/status", Kind: "TCPRoute"}, {Name: "tlsroutes", Kind: "TLSRoute"},
				},
			}}
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should return the unsupported routes only, in the preferred version", func() {
			Expect(routes).To(ConsistOf(
				gatewayapi.UnsupportedRoute{GVR: schema.GroupVersionResource{Group: gwv1.GroupName, Version: "v1", Resource: "tlsroutes"}, Kind: "TLSRoute"},
				gatewayapi.UnsupportedRoute{GVR: schema.GroupVersionResource{Group: gwv1.GroupName, Version: "v1alpha2", Resource: "tcproutes"},
					Kind: "TCPRoute"},
			))
		})
	})

	When("the discovery fails", func() {
		BeforeEach(func() {
			client.AddReactor("get", "group", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("connection refused")
			})
		})

		It("should return an error", func() { Expect(err).To(MatchError(ContainSubstring("connection refused"))) })
	})
})

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

package mapper

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/fake"
	k8stesting "k8s.io/client-go/testing"
)

var _ = Describe("The addGroup function", func() {
	var (
		client *fake.FakeDiscovery
		mapper *meta.DefaultRESTMapper
	)

	gv := schema.GroupVersion{Group: "gateway.networking.k8s.io", Version: "v1"}

	BeforeEach(func() {
		client = &fake.FakeDiscovery{Fake: &k8stesting.Fake{}}
		client.Resources = []*metav1.APIResourceList{{
			GroupVersion: gv.String(),
			APIResources: []metav1.APIResource{
				{Name: "gateways", SingularName: "gateway", Kind: "Gateway", Namespaced: true},
				{Name: "gateways/status", Kind: "Gateway", Namespaced: true},
				{Name: "gatewayclasses", SingularName: "gatewayclass", Kind: "GatewayClass"},
			},
		}}
		mapper = meta.NewDefaultRESTMapper(nil)
	})

	It("should map the kinds to the resource names retrieved from the discovery", func() {
		Expect(addGroup(client, gv, mapper, GroupRequired)).To(Succeed())

		// The resource name guessed from the kind would be "gatewaies".
		mapping, err := mapper.RESTMapping(gv.WithKind("Gateway").GroupKind(), gv.Version)
		Expect(err).ToNot(HaveOccurred())
		Expect(mapping.Resource).To(Equal(gv.WithResource("gateways")))
		Expect(mapping.Scope.Name()).To(Equal(meta.RESTScopeNameNamespace))

		mapping, err = mapper.RESTMapping(gv.WithKind("GatewayClass").GroupKind(), gv.Version)
		Expect(err).ToNot(HaveOccurred())
		Expect(mapping.Resource).To(Equal(gv.WithResource("gatewayclasses")))
		Expect(mapping.Scope.Name()).To(Equal(meta.RESTScopeNameRoot))

		kind, err := mapper.KindFor(gv.WithResource("gateway"))
		Expect(err).ToNot(HaveOccurred())
		Expect(kind).To(Equal(gv.WithKind("Gateway")))
	})

	It("should ignore optional groups not served by the cluster", func() {
		missing := schema.GroupVersion{Group: "missing.example.com", Version: "v1"}
		Expect(addGroup(client, missing, mapper, GroupOptional)).To(Succeed())
		_, err := mapper.RESTMapping(missing.WithKind("Missing").GroupKind(), missing.Version)
		Expect(meta.IsNoMatchError(err)).To(BeTrue())
	})

	It("should fail for required groups not served by the cluster", func() {
		missing := schema.GroupVersion{Group: "missing.example.com", Version: "v1"}
		Expect(addGroup(client, missing, mapper, GroupRequired)).ToNot(Succeed())
	})
})

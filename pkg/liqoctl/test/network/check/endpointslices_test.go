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

package check

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	testclient "github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/setup"
)

var _ = Describe("Pod targets on dual-stack clusters", func() {
	forgeSlice := func(name string, addrType discoveryv1.AddressType, addrs ...string) *discoveryv1.EndpointSlice {
		eps := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: setup.NamespaceName,
				Labels:    map[string]string{discoveryv1.LabelServiceName: setup.DeploymentName},
			},
			AddressType: addrType,
		}
		for _, addr := range addrs {
			eps.Endpoints = append(eps.Endpoints, discoveryv1.Endpoint{Addresses: []string{addr}})
		}
		return eps
	}

	var (
		ctx context.Context
		cl  client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		utilruntime.Must(discoveryv1.AddToScheme(scheme))
		cl = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	It("should only target the addresses of the IPv4 EndpointSlices of the consumer", func() {
		Expect(cl.Create(ctx, forgeSlice("v4", discoveryv1.AddressTypeIPv4, "10.0.0.1", "10.0.0.2"))).To(Succeed())
		Expect(cl.Create(ctx, forgeSlice("v6", discoveryv1.AddressTypeIPv6, "fd00::1", "fd00::2"))).To(Succeed())

		target := Targets{}
		err := ForgePodTargetForConsumer(ctx, &testclient.Client{Consumer: cl, ConsumerName: "consumer"}, 2, target)
		Expect(err).ToNot(HaveOccurred())
		Expect(target["consumer"]).To(ConsistOf("10.0.0.1", "10.0.0.2"))
	})

	It("should only target the addresses of the IPv4 EndpointSlices of the provider", func() {
		Expect(cl.Create(ctx, forgeSlice("v4-a", discoveryv1.AddressTypeIPv4, "10.0.0.1"))).To(Succeed())
		Expect(cl.Create(ctx, forgeSlice("v4-b", discoveryv1.AddressTypeIPv4, "10.0.0.2"))).To(Succeed())
		Expect(cl.Create(ctx, forgeSlice("v6", discoveryv1.AddressTypeIPv6, "fd00::1", "fd00::2"))).To(Succeed())

		target := Targets{}
		err := ForgePodTargetForProvider(ctx, &testclient.Client{Providers: map[string]client.Client{"provider": cl}}, "provider", 2, target)
		Expect(err).ToNot(HaveOccurred())
		Expect(target["provider"]).To(ConsistOf("10.0.0.1", "10.0.0.2"))
	})

	It("should return the same targets when only IPv4 EndpointSlices exist", func() {
		Expect(cl.Create(ctx, forgeSlice("v4", discoveryv1.AddressTypeIPv4, "10.0.0.1", "10.0.0.2"))).To(Succeed())

		target := Targets{}
		err := ForgePodTargetForConsumer(ctx, &testclient.Client{Consumer: cl, ConsumerName: "consumer"}, 2, target)
		Expect(err).ToNot(HaveOccurred())
		Expect(target["consumer"]).To(Equal([]string{"10.0.0.1", "10.0.0.2"}))
	})
})

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

package ipmapping

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("EndpointSlice IP mapping on dual-stack and IPv6 clusters", func() {
	const (
		ipNamespace = "liqo"
		ipv4Slice   = "kubernetes-v4"
		ipv6Slice   = "kubernetes-v6"
		apiServerV4 = "10.96.0.1"
		apiServerV6 = "fd00:96::1"
	)

	var (
		ctx        context.Context
		cl         client.Client
		reconciler *EndpointSliceIPReconciler
	)

	forgeSlice := func(name string, addrType discoveryv1.AddressType, addrs ...string) *discoveryv1.EndpointSlice {
		eps := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: kubernetesServiceNamespace,
				Labels:    map[string]string{discoveryv1.LabelServiceName: kubernetesServiceName},
			},
			AddressType: addrType,
		}
		for _, addr := range addrs {
			eps.Endpoints = append(eps.Endpoints, discoveryv1.Endpoint{Addresses: []string{addr}})
		}
		return eps
	}

	reconcile := func(name string) {
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: kubernetesServiceNamespace}})
		Expect(err).ToNot(HaveOccurred())
	}

	listIPs := func() []ipamv1alpha1.IP {
		var ips ipamv1alpha1.IPList
		Expect(cl.List(ctx, &ips, client.InNamespace(ipNamespace))).To(Succeed())
		return ips.Items
	}

	ipNames := func() []string {
		names := []string{}
		for _, ip := range listIPs() {
			names = append(names, ip.Name)
		}
		return names
	}

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		utilruntime.Must(discoveryv1.AddToScheme(scheme))
		utilruntime.Must(ipamv1alpha1.AddToScheme(scheme))
		utilruntime.Must(networkingv1beta1.AddToScheme(scheme))
		cl = fake.NewClientBuilder().WithScheme(scheme).Build()
		reconciler = &EndpointSliceIPReconciler{Client: cl, Scheme: scheme, ipNamespace: ipNamespace}
	})

	It("should create the IP resources of an IPv4 EndpointSlice", func() {
		eps := forgeSlice(ipv4Slice, discoveryv1.AddressTypeIPv4, apiServerV4)
		Expect(cl.Create(ctx, eps)).To(Succeed())

		reconcile(ipv4Slice)

		ips := listIPs()
		Expect(ips).To(HaveLen(1))
		Expect(ips[0].Name).To(Equal(forgeIPname(eps, apiServerV4)))
		Expect(ips[0].Spec.IP).To(Equal(networkingv1beta1.IP(apiServerV4)))
		Expect(ips[0].Labels).To(HaveKeyWithValue(consts.IPEndpointSliceNameLabelKey, ipv4Slice))
		Expect(ips[0].Labels).To(HaveKeyWithValue(consts.IPTypeLabelKey, consts.IPTypeAPIServer))
	})

	It("should create no IP resources for an IPv6 EndpointSlice", func() {
		Expect(cl.Create(ctx, forgeSlice(ipv6Slice, discoveryv1.AddressTypeIPv6, apiServerV6))).To(Succeed())

		reconcile(ipv6Slice)

		Expect(listIPs()).To(BeEmpty())
	})

	It("should not delete the IP resources of the IPv4 EndpointSlice when reconciling the IPv6 one", func() {
		epsV4 := forgeSlice(ipv4Slice, discoveryv1.AddressTypeIPv4, apiServerV4)
		Expect(cl.Create(ctx, epsV4)).To(Succeed())
		Expect(cl.Create(ctx, forgeSlice(ipv6Slice, discoveryv1.AddressTypeIPv6, apiServerV6))).To(Succeed())

		reconcile(ipv4Slice)
		Expect(ipNames()).To(ConsistOf(forgeIPname(epsV4, apiServerV4)))

		reconcile(ipv6Slice)
		Expect(ipNames()).To(ConsistOf(forgeIPname(epsV4, apiServerV4)))
	})

	It("should still delete the stale IP resources of an IPv4 EndpointSlice", func() {
		epsV4 := forgeSlice(ipv4Slice, discoveryv1.AddressTypeIPv4, apiServerV4, "10.96.0.2")
		Expect(cl.Create(ctx, epsV4)).To(Succeed())
		reconcile(ipv4Slice)
		Expect(listIPs()).To(HaveLen(2))

		epsV4.Endpoints = epsV4.Endpoints[:1]
		Expect(cl.Update(ctx, epsV4)).To(Succeed())
		reconcile(ipv4Slice)
		Expect(ipNames()).To(ConsistOf(forgeIPname(epsV4, apiServerV4)))
	})
})

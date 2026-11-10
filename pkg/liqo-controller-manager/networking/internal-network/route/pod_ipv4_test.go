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

package route

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

var _ = Describe("Pod route reconciliation on dual-stack and IPv6 clusters", func() {
	const (
		liqoNamespace = "liqo"
		podNamespace  = "default"
		nodeName      = "node-2"
		podIPv4       = "10.20.57.90"
		podIPv6       = "fd00:10:20::5a"
	)

	var (
		ctx        context.Context
		cl         client.Client
		reconciler *PodReconciler
	)

	forgePod := func(name string, uid types.UID, hostNetwork bool, ips ...string) *corev1.Pod {
		status := corev1.PodStatus{Phase: corev1.PodRunning}
		if len(ips) > 0 {
			status.PodIP = ips[0]
		}
		for _, ip := range ips {
			status.PodIPs = append(status.PodIPs, corev1.PodIP{IP: ip})
		}
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: podNamespace, UID: uid},
			Spec:       corev1.PodSpec{NodeName: nodeName, HostNetwork: hostNetwork},
			Status:     status,
		}
	}

	reconcile := func(name string) {
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: podNamespace}})
		Expect(err).ToNot(HaveOccurred())
	}

	getRouteConfig := func() (*networkingv1beta1.RouteConfiguration, error) {
		routecfg := &networkingv1beta1.RouteConfiguration{}
		err := cl.Get(ctx, types.NamespacedName{Name: generatePodRouteConfigurationName(nodeName), Namespace: liqoNamespace}, routecfg)
		return routecfg, err
	}

	BeforeEach(func() {
		ctx = context.Background()
		checkLeftoverRoutesOnce.Do(func() {})
		podKeyToNode.Range(func(key, _ any) bool {
			podKeyToNode.Delete(key)
			return true
		})

		scheme := runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(scheme))
		utilruntime.Must(networkingv1beta1.AddToScheme(scheme))

		internalNode := &networkingv1beta1.InternalNode{
			ObjectMeta: metav1.ObjectMeta{Name: nodeName},
			Spec: networkingv1beta1.InternalNodeSpec{
				Interface: networkingv1beta1.InternalNodeSpecInterface{
					Gateway: networkingv1beta1.InternalNodeSpecInterfaceGateway{Name: "liqo-node-2"},
					Node:    networkingv1beta1.InternalNodeSpecInterfaceNode{IP: "10.80.0.3"},
				},
			},
		}
		cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(internalNode).Build()
		reconciler = NewPodReconciler(cl, scheme, record.NewFakeRecorder(10), &Options{Namespace: liqoNamespace})
	})

	It("should use the IPv4 address of an IPv6-primary dual-stack pod", func() {
		Expect(cl.Create(ctx, forgePod("dual", "u1", false, podIPv6, podIPv4))).To(Succeed())
		reconcile("dual")

		routecfg, err := getRouteConfig()
		Expect(err).ToNot(HaveOccurred())
		Expect(routecfg.Spec.Table.Rules).To(HaveLen(2))
		routes := routecfg.Spec.Table.Rules[1].Routes
		Expect(routes).To(HaveLen(1))
		Expect(routes[0].Dst.String()).To(Equal(podIPv4 + "/32"))
		Expect(routes[0].TargetRef.UID).To(Equal(types.UID("u1")))
	})

	It("should adopt by IPv4 the route of a previous pod on an IPv6-primary dual-stack cluster", func() {
		Expect(cl.Create(ctx, forgePod("old", "u-old", false, podIPv6, podIPv4))).To(Succeed())
		reconcile("old")
		Expect(cl.Create(ctx, forgePod("new", "u-new", false, "fd00:10:20::ffff", podIPv4))).To(Succeed())
		reconcile("new")

		routecfg, err := getRouteConfig()
		Expect(err).ToNot(HaveOccurred())
		routes := routecfg.Spec.Table.Rules[1].Routes
		Expect(routes).To(HaveLen(1))
		Expect(routes[0].Dst.String()).To(Equal(podIPv4 + "/32"))
		Expect(routes[0].TargetRef.UID).To(Equal(types.UID("u-new")))
	})

	It("should not create a pod route for an IPv6-only pod", func() {
		Expect(cl.Create(ctx, forgePod("v6", "u6", false, podIPv6))).To(Succeed())
		reconcile("v6")

		_, err := getRouteConfig()
		Expect(client.IgnoreNotFound(err)).To(Succeed())
		Expect(err).To(HaveOccurred())
	})

	It("should still enforce the node route for an IPv6-only host network pod", func() {
		Expect(cl.Create(ctx, forgePod("hostv6", "uh", true, podIPv6))).To(Succeed())
		reconcile("hostv6")

		routecfg, err := getRouteConfig()
		Expect(err).ToNot(HaveOccurred())
		Expect(routecfg.Spec.Table.Rules).To(HaveLen(1))
		Expect(routecfg.Spec.Table.Rules[0].Routes).To(HaveLen(1))
		Expect(routecfg.Spec.Table.Rules[0].Routes[0].Dst.String()).To(Equal("10.80.0.3/32"))
	})

	It("should wait for pods without any IP", func() {
		Expect(cl.Create(ctx, forgePod("noip", "un", false))).To(Succeed())
		reconcile("noip")

		_, err := getRouteConfig()
		Expect(err).To(HaveOccurred())
	})
})

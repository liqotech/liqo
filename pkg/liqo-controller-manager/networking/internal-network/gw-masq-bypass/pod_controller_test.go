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

package gwmasqbypass

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

var _ = Describe("Gateway masquerade bypass on dual-stack and IPv6 clusters", func() {
	const (
		liqoNamespace = "liqo"
		podNamespace  = "liqo-tenant"
		nodeName      = "node-1"
		genevePort    = uint16(6091)
		podIPv4       = "10.20.57.90"
		podIPv6       = "fd00:10:20::5a"
	)

	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		cl         client.Client
		reconciler *PodReconciler
	)

	forgePod := func(name string, scheduled bool, ips ...string) *corev1.Pod {
		status := corev1.PodStatus{Phase: corev1.PodRunning}
		if len(ips) > 0 {
			status.PodIP = ips[0]
		}
		for _, ip := range ips {
			status.PodIPs = append(status.PodIPs, corev1.PodIP{IP: ip})
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: podNamespace, UID: types.UID("uid-" + name)},
			Status:     status,
		}
		if scheduled {
			pod.Spec.NodeName = nodeName
		}
		return pod
	}

	reconcile := func(name string) ctrl.Result {
		res, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: podNamespace}})
		Expect(err).ToNot(HaveOccurred())
		return res
	}

	getFirewallConfiguration := func() (*networkingv1beta1.FirewallConfiguration, error) {
		fwcfg := &networkingv1beta1.FirewallConfiguration{}
		err := cl.Get(ctx, types.NamespacedName{Name: generateFirewallConfigurationName(nodeName), Namespace: liqoNamespace}, fwcfg)
		return fwcfg, err
	}

	BeforeEach(func() {
		ctx = context.Background()
		checkLeftoverRulesOnce.Do(func() {})
		podKeyToNode.Range(func(key, _ any) bool {
			podKeyToNode.Delete(key)
			return true
		})

		scheme = runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(scheme))
		utilruntime.Must(networkingv1beta1.AddToScheme(scheme))

		internalNode := &networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
		cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(internalNode).Build()
		reconciler = NewPodReconciler(cl, scheme, record.NewFakeRecorder(10), &Options{Namespace: liqoNamespace, GenevePort: genevePort})
	})

	It("should forge the source NAT rule with the IPv4 address of an IPv6-primary dual-stack pod", func() {
		Expect(cl.Create(ctx, forgePod("dual", true, podIPv6, podIPv4))).To(Succeed())
		Expect(reconcile("dual")).To(Equal(ctrl.Result{}))

		fwcfg, err := getFirewallConfiguration()
		Expect(err).ToNot(HaveOccurred())
		Expect(fwcfg.Spec.Table.Family).To(HaveValue(Equal(firewall.TableFamilyIPv4)))
		Expect(fwcfg.Spec.Table.Chains).To(HaveLen(1))
		rules := fwcfg.Spec.Table.Chains[0].Rules.NatRules
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Name).To(HaveValue(Equal("dual")))
		Expect(rules[0].NatType).To(Equal(firewall.NatTypeSource))
		Expect(rules[0].To).To(HaveValue(Equal(podIPv4)))
		Expect(rules[0].Match).To(HaveLen(1))
		Expect(rules[0].Match[0].IP.Value).To(Equal(podIPv4))
		Expect(rules[0].Match[0].IP.Position).To(Equal(firewall.MatchPositionSrc))
	})

	It("should update the rule of an existing pod to its IPv4 address", func() {
		// The Kind is set explicitly, as objects read through the manager cache carry it, while the ones of the fake client do not.
		pod := forgePod("dual", true, podIPv6, podIPv4)
		pod.Kind = "Pod"
		internalNode := &networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
		fwcfg := &networkingv1beta1.FirewallConfiguration{}

		Expect(forgeFirewallPodUpdateFunction(internalNode, fwcfg, pod, podIPv4, scheme, genevePort)()).To(Succeed())
		Expect(forgeFirewallPodUpdateFunction(internalNode, fwcfg, pod, "10.20.57.91", scheme, genevePort)()).To(Succeed())

		rules := fwcfg.Spec.Table.Chains[0].Rules.NatRules
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].To).To(HaveValue(Equal("10.20.57.91")))
		Expect(rules[0].Match[0].IP.Value).To(Equal("10.20.57.91"))
	})

	It("should keep forging the rule for IPv4 single-stack pods", func() {
		Expect(cl.Create(ctx, forgePod("single", true, podIPv4))).To(Succeed())
		reconcile("single")

		fwcfg, err := getFirewallConfiguration()
		Expect(err).ToNot(HaveOccurred())
		rules := fwcfg.Spec.Table.Chains[0].Rules.NatRules
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].To).To(HaveValue(Equal(podIPv4)))
		Expect(rules[0].Match[0].IP.Value).To(Equal(podIPv4))
	})

	It("should skip an IPv6-only pod without requeuing nor failing", func() {
		Expect(cl.Create(ctx, forgePod("v6only", true, podIPv6))).To(Succeed())
		Expect(reconcile("v6only")).To(Equal(ctrl.Result{}))

		_, err := getFirewallConfiguration()
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should wait for a scheduled pod without IP address", func() {
		Expect(cl.Create(ctx, forgePod("noip", true))).To(Succeed())
		Expect(reconcile("noip")).To(Equal(ctrl.Result{}))

		_, err := getFirewallConfiguration()
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should wait for a pod that is not scheduled yet", func() {
		Expect(cl.Create(ctx, forgePod("pending", false, podIPv6, podIPv4))).To(Succeed())
		Expect(reconcile("pending")).To(Equal(ctrl.Result{}))

		_, err := getFirewallConfiguration()
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
})

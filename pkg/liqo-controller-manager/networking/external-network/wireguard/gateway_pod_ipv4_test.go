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

package wireguard

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/gateway"
	podutils "github.com/liqotech/liqo/pkg/utils/pod"
)

var _ = Describe("Active gateway pod IP on dual-stack and IPv6 clusters", func() {
	const (
		namespace = "liqo-tenant"
		nodeName  = "node-1"
		podIPv4   = "10.20.57.90"
		podIPv6   = "fd00:10:20::5a"
	)

	var (
		ctx    context.Context
		scheme *runtime.Scheme
		cl     client.Client
	)

	forgeGatewayPod := func(ips ...string) *corev1.Pod {
		status := corev1.PodStatus{Phase: corev1.PodRunning}
		if len(ips) > 0 {
			status.PodIP = ips[0]
		}
		for _, ip := range ips {
			status.PodIPs = append(status.PodIPs, corev1.PodIP{IP: ip})
		}
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "gw-pod", Namespace: namespace, Labels: gateway.ForgeActiveGatewayPodLabels()},
			Spec:       corev1.PodSpec{NodeName: nodeName},
			Status:     status,
		}
	}

	forgeDeployment := func() *appsv1.Deployment {
		return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: namespace}}
	}

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(scheme))
		utilruntime.Must(appsv1.AddToScheme(scheme))
		utilruntime.Must(networkingv1beta1.AddToScheme(scheme))
		cl = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	Describe("listActiveGatewayPod", func() {
		It("should return the pod of an IPv6-primary dual-stack gateway", func() {
			Expect(cl.Create(ctx, forgeGatewayPod(podIPv6, podIPv4))).To(Succeed())

			pod, err := listActiveGatewayPod(ctx, cl, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(pod.Name).To(Equal("gw-pod"))
		})

		It("should return the pod of an IPv4 single-stack gateway", func() {
			Expect(cl.Create(ctx, forgeGatewayPod(podIPv4))).To(Succeed())

			pod, err := listActiveGatewayPod(ctx, cl, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(pod.Name).To(Equal("gw-pod"))
		})

		It("should return an error wrapping ErrNoIPv4Address for an IPv6-only gateway", func() {
			Expect(cl.Create(ctx, forgeGatewayPod(podIPv6))).To(Succeed())

			pod, err := listActiveGatewayPod(ctx, cl, namespace)
			Expect(pod).To(BeNil())
			Expect(errors.Is(err, podutils.ErrNoIPv4Address)).To(BeTrue())
		})

		It("should ask to retry later when the gateway has no IP yet", func() {
			Expect(cl.Create(ctx, forgeGatewayPod())).To(Succeed())

			pod, err := listActiveGatewayPod(ctx, cl, namespace)
			Expect(pod).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("does not have IP assigned yet. Retry later")))
			Expect(errors.Is(err, podutils.ErrNoIPv4Address)).To(BeFalse())
		})
	})

	Describe("handleInternalEndpointStatus", func() {
		It("should set the IPv4 address of an IPv6-primary dual-stack gateway in the WgGatewayServer status", func() {
			Expect(cl.Create(ctx, forgeGatewayPod(podIPv6, podIPv4))).To(Succeed())
			reconciler := &WgGatewayServerReconciler{Client: cl, Scheme: scheme}
			wgServer := &networkingv1beta1.WgGatewayServer{}

			Expect(reconciler.handleInternalEndpointStatus(ctx, wgServer, forgeDeployment())).To(Succeed())
			Expect(wgServer.Status.InternalEndpoint).ToNot(BeNil())
			Expect(wgServer.Status.InternalEndpoint.IP).ToNot(BeNil())
			Expect(string(*wgServer.Status.InternalEndpoint.IP)).To(Equal(podIPv4))
			Expect(wgServer.Status.InternalEndpoint.Node).To(HaveValue(Equal(nodeName)))
		})

		It("should set the IPv4 address of an IPv6-primary dual-stack gateway in the WgGatewayClient status", func() {
			Expect(cl.Create(ctx, forgeGatewayPod(podIPv6, podIPv4))).To(Succeed())
			reconciler := &WgGatewayClientReconciler{Client: cl, Scheme: scheme}
			wgClient := &networkingv1beta1.WgGatewayClient{}

			Expect(reconciler.handleInternalEndpointStatus(ctx, wgClient, forgeDeployment())).To(Succeed())
			Expect(wgClient.Status.InternalEndpoint).ToNot(BeNil())
			Expect(wgClient.Status.InternalEndpoint.IP).ToNot(BeNil())
			Expect(string(*wgClient.Status.InternalEndpoint.IP)).To(Equal(podIPv4))
			Expect(wgClient.Status.InternalEndpoint.Node).To(HaveValue(Equal(nodeName)))
		})

		It("should fail and leave the status untouched for an IPv6-only gateway", func() {
			Expect(cl.Create(ctx, forgeGatewayPod(podIPv6))).To(Succeed())
			server := &WgGatewayServerReconciler{Client: cl, Scheme: scheme}
			wgServer := &networkingv1beta1.WgGatewayServer{}
			err := server.handleInternalEndpointStatus(ctx, wgServer, forgeDeployment())
			Expect(errors.Is(err, podutils.ErrNoIPv4Address)).To(BeTrue())
			Expect(wgServer.Status.InternalEndpoint).To(BeNil())

			clientRec := &WgGatewayClientReconciler{Client: cl, Scheme: scheme}
			wgClient := &networkingv1beta1.WgGatewayClient{}
			err = clientRec.handleInternalEndpointStatus(ctx, wgClient, forgeDeployment())
			Expect(errors.Is(err, podutils.ErrNoIPv4Address)).To(BeTrue())
			Expect(wgClient.Status.InternalEndpoint).To(BeNil())
		})
	})
})

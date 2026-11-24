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
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	podutils "github.com/liqotech/liqo/pkg/utils/pod"
)

var _ = Describe("Pod IP mapping on dual-stack and IPv6 clusters", func() {
	const (
		podNamespace = "default"
		podIPv4      = "10.20.57.90"
		podIPv6      = "fd00:10:20::5a"
	)

	var (
		ctx    context.Context
		cl     client.Client
		scheme *runtime.Scheme
	)

	forgePod := func(name string, ips ...string) *corev1.Pod {
		status := corev1.PodStatus{Phase: corev1.PodRunning}
		if len(ips) > 0 {
			status.PodIP = ips[0]
		}
		for _, ip := range ips {
			status.PodIPs = append(status.PodIPs, corev1.PodIP{IP: ip})
		}
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: podNamespace, UID: types.UID("uid-" + name)},
			Status:     status,
		}
	}

	getIP := func(name string) (*ipamv1alpha1.IP, error) {
		ip := &ipamv1alpha1.IP{}
		err := cl.Get(ctx, types.NamespacedName{Name: name, Namespace: podNamespace}, ip)
		return ip, err
	}

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(scheme))
		utilruntime.Must(ipamv1alpha1.AddToScheme(scheme))
		utilruntime.Must(networkingv1beta1.AddToScheme(scheme))
		cl = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	Describe("CreateOrUpdateIP", func() {
		It("should set the IPv4 address of an IPv6-primary dual-stack pod", func() {
			pod := forgePod("dual", podIPv6, podIPv4)
			Expect(cl.Create(ctx, pod)).To(Succeed())

			op, err := CreateOrUpdateIP(ctx, cl, scheme, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(op).To(Equal(controllerutil.OperationResultCreated))

			ip, err := getIP("dual")
			Expect(err).ToNot(HaveOccurred())
			Expect(ip.Spec.IP).To(Equal(networkingv1beta1.IP(podIPv4)))
			Expect(ip.Labels).To(HaveKeyWithValue(offloadedPodNameLabelKey, "dual"))
		})

		It("should keep the pod address on IPv4 single-stack pods", func() {
			pod := forgePod("single", podIPv4)
			Expect(cl.Create(ctx, pod)).To(Succeed())

			_, err := CreateOrUpdateIP(ctx, cl, scheme, pod)
			Expect(err).ToNot(HaveOccurred())

			ip, err := getIP("single")
			Expect(err).ToNot(HaveOccurred())
			Expect(ip.Spec.IP).To(Equal(networkingv1beta1.IP(podIPv4)))
		})

		It("should update the IP resource to the IPv4 address", func() {
			pod := forgePod("update", podIPv6, podIPv4)
			Expect(cl.Create(ctx, pod)).To(Succeed())
			_, err := CreateOrUpdateIP(ctx, cl, scheme, pod)
			Expect(err).ToNot(HaveOccurred())

			pod.Status.PodIPs = []corev1.PodIP{{IP: podIPv6}, {IP: "10.20.57.91"}}
			op, err := CreateOrUpdateIP(ctx, cl, scheme, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(op).To(Equal(controllerutil.OperationResultUpdated))

			ip, err := getIP("update")
			Expect(err).ToNot(HaveOccurred())
			Expect(ip.Spec.IP).To(Equal(networkingv1beta1.IP("10.20.57.91")))
		})

		It("should return ErrNoIPAddress for a pod without IP and create no IP resource", func() {
			pod := forgePod("noip")
			Expect(cl.Create(ctx, pod)).To(Succeed())

			op, err := CreateOrUpdateIP(ctx, cl, scheme, pod)
			Expect(errors.Is(err, podutils.ErrNoIPAddress)).To(BeTrue())
			Expect(op).To(Equal(controllerutil.OperationResultNone))

			_, err = getIP("noip")
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should return ErrNoIPv4Address for an IPv6-only pod and create no IP resource", func() {
			pod := forgePod("v6only", podIPv6)
			Expect(cl.Create(ctx, pod)).To(Succeed())

			op, err := CreateOrUpdateIP(ctx, cl, scheme, pod)
			Expect(errors.Is(err, podutils.ErrNoIPv4Address)).To(BeTrue())
			Expect(op).To(Equal(controllerutil.OperationResultNone))

			_, err = getIP("v6only")
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	})

	Describe("OffloadedPodReconciler", func() {
		var reconciler *OffloadedPodReconciler

		reconcile := func(name string) (ctrl.Result, error) {
			return reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: podNamespace}})
		}

		BeforeEach(func() {
			reconciler = NewOffloadedPodReconciler(cl, scheme, nil)
		})

		It("should create the IP resource with the IPv4 address of an IPv6-primary dual-stack pod", func() {
			Expect(cl.Create(ctx, forgePod("dual", podIPv6, podIPv4))).To(Succeed())

			res, err := reconcile("dual")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))

			ip, err := getIP("dual")
			Expect(err).ToNot(HaveOccurred())
			Expect(ip.Spec.IP).To(Equal(networkingv1beta1.IP(podIPv4)))
		})

		It("should requeue when the pod has no IP address yet", func() {
			Expect(cl.Create(ctx, forgePod("noip"))).To(Succeed())

			res, err := reconcile("noip")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{RequeueAfter: time.Second}))

			_, err = getIP("noip")
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should neither requeue nor fail for an IPv6-only pod, and create no IP resource", func() {
			Expect(cl.Create(ctx, forgePod("v6only", podIPv6))).To(Succeed())

			res, err := reconcile("v6only")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))

			_, err = getIP("v6only")
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should ignore a missing pod", func() {
			res, err := reconcile("missing")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))
		})
	})
})

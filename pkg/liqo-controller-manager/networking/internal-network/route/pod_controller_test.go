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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

var _ = Describe("Pod route reconciliation", func() {
	const (
		liqoNamespace = "liqo"
		podNamespace  = "default"
		nodeName      = "node-1"
		podIP         = "10.20.57.88"
		podDst        = podIP + "/32"
	)

	var (
		ctx        context.Context
		cl         client.Client
		reconciler *PodReconciler
	)

	forgePod := func(name string, uid types.UID) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: podNamespace, UID: uid},
			Spec:       corev1.PodSpec{NodeName: nodeName},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning, PodIP: podIP},
		}
	}

	reconcile := func(name string) {
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: podNamespace}})
		Expect(err).ToNot(HaveOccurred())
	}

	setPhase := func(name string, phase corev1.PodPhase) {
		pod := &corev1.Pod{}
		Expect(cl.Get(ctx, types.NamespacedName{Name: name, Namespace: podNamespace}, pod)).To(Succeed())
		pod.Status.Phase = phase
		Expect(cl.Status().Update(ctx, pod)).To(Succeed())
	}

	// expectRouteOwner checks that the route towards the pod IP exists and references the given pod.
	expectRouteOwner := func(name string, uid types.UID) {
		routecfg := &networkingv1beta1.RouteConfiguration{}
		Expect(cl.Get(ctx, types.NamespacedName{
			Name: generatePodRouteConfigurationName(nodeName), Namespace: liqoNamespace,
		}, routecfg)).To(Succeed())
		Expect(routecfg.Spec.Table.Rules).To(HaveLen(2))
		routes := routecfg.Spec.Table.Rules[1].Routes
		Expect(routes).To(HaveLen(1))
		Expect(routes[0].Dst.String()).To(Equal(podDst))
		Expect(routes[0].TargetRef).ToNot(BeNil())
		Expect(routes[0].TargetRef.Name).To(Equal(name))
		Expect(routes[0].TargetRef.UID).To(Equal(uid))
	}

	BeforeEach(func() {
		ctx = context.Background()

		// Skip the startup leftover check, which is not relevant here.
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
					Gateway: networkingv1beta1.InternalNodeSpecInterfaceGateway{Name: "liqo-node-1"},
					Node:    networkingv1beta1.InternalNodeSpecInterfaceNode{IP: "10.80.0.2"},
				},
			},
		}

		cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(internalNode, forgePod("completed-job", "old-uid")).Build()
		reconciler = NewPodReconciler(cl, scheme, record.NewFakeRecorder(10), &Options{Namespace: liqoNamespace})

		// A job pod gets the IP and the corresponding route.
		reconcile("completed-job")
		expectRouteOwner("completed-job", "old-uid")

		// The job completes, its IP is released and reassigned to a new pod on the same node.
		setPhase("completed-job", corev1.PodSucceeded)
		reconcile("completed-job")
		Expect(cl.Create(ctx, forgePod("new-pod", "new-uid"))).To(Succeed())
		reconcile("new-pod")
	})

	It("should transfer the route ownership to the pod currently using the IP", func() {
		expectRouteOwner("new-pod", "new-uid")
	})

	It("should not let the terminated pod claim back the route", func() {
		// E.g., periodic resync or controller restart.
		reconcile("completed-job")
		expectRouteOwner("new-pod", "new-uid")
	})

	It("should remove the old route when a pod with the same name is recreated with a different IP", func() {
		stateful := forgePod("stateful", "st-1")
		stateful.Status.PodIP = "10.20.57.60"
		Expect(cl.Create(ctx, stateful)).To(Succeed())
		reconcile("stateful")

		// The deletion of the old pod object removes its route, which is matched by TargetRef.
		Expect(cl.Delete(ctx, stateful)).To(Succeed())
		reconcile("stateful")

		recreated := forgePod("stateful", "st-2")
		recreated.Status.PodIP = "10.20.57.61"
		Expect(cl.Create(ctx, recreated)).To(Succeed())
		reconcile("stateful")

		routecfg := &networkingv1beta1.RouteConfiguration{}
		Expect(cl.Get(ctx, types.NamespacedName{
			Name: generatePodRouteConfigurationName(nodeName), Namespace: liqoNamespace,
		}, routecfg)).To(Succeed())
		routes := routecfg.Spec.Table.Rules[1].Routes
		Expect(routes).To(HaveLen(2))
		Expect(routes).To(ContainElement(HaveField("TargetRef.UID", types.UID("st-2"))))
		Expect(routes).To(ContainElement(HaveField("Dst", ptr.To(networkingv1beta1.CIDR("10.20.57.61/32")))))
		Expect(routes).ToNot(ContainElement(HaveField("Dst", ptr.To(networkingv1beta1.CIDR("10.20.57.60/32")))))
	})

	It("should keep the route when the terminated pod with the same IP is deleted", func() {
		reconcile("completed-job")
		Expect(cl.Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "completed-job", Namespace: podNamespace}})).To(Succeed())
		reconcile("completed-job")
		expectRouteOwner("new-pod", "new-uid")

		// Deleting the terminated pod must not remove the route even if the deletion
		// had been triggered with the full pod object (which still holds the old IP).
		deleted := forgePod("completed-job", "old-uid")
		PopulatePodKeyToNodeMap(deleted)
		Expect(enforceRoutePodAbsence(ctx, cl, reconciler.Options, deleted)).To(Succeed())
		expectRouteOwner("new-pod", "new-uid")
		DeletePodKeyFromMap(client.ObjectKeyFromObject(deleted))
	})
})

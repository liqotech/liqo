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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/forge"
)

const (
	wgNamespace = "tenant-1"
	wgName      = "wg-client-1"
	clusterRole = "liqo-gateway-role"
)

// wgClientPending builds a WgGatewayClient already in a deletion state and carrying
// the ClusterRoleBindingFinalizer, which is the precondition for the new deletion
// sequencing logic.
func wgClientPending(saName string) *networkingv1beta1.WgGatewayClient {
	now := metav1.Now()
	o := &networkingv1beta1.WgGatewayClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:              wgName,
			Namespace:         wgNamespace,
			DeletionTimestamp: &now,
			Finalizers:        []string{consts.ClusterRoleBindingFinalizer},
		},
	}
	o.Spec.Deployment.Spec.Template.Spec.ServiceAccountName = saName
	return o
}

func wgClientReconciler(objs ...client.Object) *WgGatewayClientReconciler {
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme).
		WithStatusSubresource(&networkingv1beta1.WgGatewayClient{}).
		WithObjects(objs...)
	return NewWgGatewayClientReconciler(cb.Build(), scheme.Scheme, record.NewFakeRecorder(10), clusterRole)
}

func gwDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forge.GatewayResourceName(wgName),
			Namespace: wgNamespace,
		},
	}
}

func gwPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-pod-1",
			Namespace: wgNamespace,
			Labels: map[string]string{
				consts.GatewayNameLabel:      wgName,
				consts.GatewayNamespaceLabel: wgNamespace,
			},
		},
	}
}

func gwServiceAccount(name string, withFinalizer bool) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: wgNamespace,
		},
	}
	if withFinalizer {
		sa.Finalizers = []string{consts.GatewayServiceAccountFinalizer}
	}
	return sa
}

func gwClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "irrelevant-crb-name",
			Labels: map[string]string{
				consts.GatewayNameLabel:      wgName,
				consts.GatewayNamespaceLabel: wgNamespace,
			},
		},
	}
}

func reqWgClient() ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: wgName, Namespace: wgNamespace}}
}

var _ = Describe("WgGatewayClientReconciler deletion sequencing", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("triggers foreground deletion on the Deployment and requeues while pods still exist via the Deployment", func() {
		wg := wgClientPending("gw-sa")
		deploy := gwDeployment()
		r := wgClientReconciler(wg, deploy)

		res, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))

		// The Deployment must now carry a non-zero DeletionTimestamp (foreground propagation
		// adds a finalizer + DeletionTimestamp in real clusters; the fake client at least sets
		// the timestamp). We assert that the controller hit the Delete path.
		var d appsv1.Deployment
		err = r.Get(ctx, types.NamespacedName{Name: forge.GatewayResourceName(wgName), Namespace: wgNamespace}, &d)
		// The fake client deletes immediately because there are no real finalizers; either way
		// the requeue ensures we will return to the pod-check branch.
		if err == nil {
			Expect(d.DeletionTimestamp.IsZero()).To(BeFalse())
		} else {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}

		// WgGatewayClient finalizer must NOT have been removed yet.
		var got networkingv1beta1.WgGatewayClient
		Expect(r.Get(ctx, reqWgClient().NamespacedName, &got)).To(Succeed())
		Expect(got.Finalizers).To(ContainElement(consts.ClusterRoleBindingFinalizer))
	})

	It("does not re-issue Delete when the Deployment is already being deleted, and still requeues", func() {
		wg := wgClientPending("gw-sa")
		deploy := gwDeployment()
		now := metav1.Now()
		deploy.DeletionTimestamp = &now
		deploy.Finalizers = []string{"keep"} // required so the fake client accepts the deletion timestamp
		r := wgClientReconciler(wg, deploy)

		res, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))

		// CRB removal must NOT have been reached.
		var got networkingv1beta1.WgGatewayClient
		Expect(r.Get(ctx, reqWgClient().NamespacedName, &got)).To(Succeed())
		Expect(got.Finalizers).To(ContainElement(consts.ClusterRoleBindingFinalizer))
	})

	It("requeues while gateway pods still exist after the Deployment is gone", func() {
		wg := wgClientPending("gw-sa")
		r := wgClientReconciler(wg, gwPod())

		res, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))

		var got networkingv1beta1.WgGatewayClient
		Expect(r.Get(ctx, reqWgClient().NamespacedName, &got)).To(Succeed())
		Expect(got.Finalizers).To(ContainElement(consts.ClusterRoleBindingFinalizer))
	})

	It("removes the SA finalizer, deletes the CRB, and removes the wg finalizer once everything is gone", func() {
		wg := wgClientPending("gw-sa")
		sa := gwServiceAccount("gw-sa", true /* withFinalizer */)
		crb := gwClusterRoleBinding()
		r := wgClientReconciler(wg, sa, crb)

		res, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		// ServiceAccount no longer carries the gateway finalizer.
		var gotSA corev1.ServiceAccount
		Expect(r.Get(ctx, types.NamespacedName{Name: "gw-sa", Namespace: wgNamespace}, &gotSA)).To(Succeed())
		Expect(gotSA.Finalizers).ToNot(ContainElement(consts.GatewayServiceAccountFinalizer))

		// CRB was deleted.
		var crbList rbacv1.ClusterRoleBindingList
		Expect(r.List(ctx, &crbList, client.MatchingLabels{
			consts.GatewayNameLabel:      wgName,
			consts.GatewayNamespaceLabel: wgNamespace,
		})).To(Succeed())
		Expect(crbList.Items).To(BeEmpty())

		// WgGatewayClient: ClusterRoleBindingFinalizer is gone (either object removed, or finalizer absent).
		var got networkingv1beta1.WgGatewayClient
		err = r.Get(ctx, reqWgClient().NamespacedName, &got)
		if err == nil {
			Expect(got.Finalizers).ToNot(ContainElement(consts.ClusterRoleBindingFinalizer))
		} else {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
	})

	It("proceeds when the ServiceAccount is missing", func() {
		wg := wgClientPending("gw-sa")
		// No SA object in the client.
		r := wgClientReconciler(wg, gwClusterRoleBinding())

		res, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		// CRB was deleted; wg finalizer removed.
		var crbList rbacv1.ClusterRoleBindingList
		Expect(r.List(ctx, &crbList)).To(Succeed())
		Expect(crbList.Items).To(BeEmpty())
	})

	It("does not require a SA finalizer to exist (leaves a plain SA untouched)", func() {
		wg := wgClientPending("gw-sa")
		sa := gwServiceAccount("gw-sa", false /* no finalizer */)
		r := wgClientReconciler(wg, sa, gwClusterRoleBinding())

		_, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())

		var gotSA corev1.ServiceAccount
		Expect(r.Get(ctx, types.NamespacedName{Name: "gw-sa", Namespace: wgNamespace}, &gotSA)).To(Succeed())
		Expect(gotSA.Finalizers).To(BeEmpty())
	})

	It("defaults the SA name to 'default' when none is specified on the deployment template", func() {
		wg := wgClientPending("" /* empty SA name */)
		defaultSA := gwServiceAccount("default", true)
		r := wgClientReconciler(wg, defaultSA, gwClusterRoleBinding())

		_, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())

		var gotSA corev1.ServiceAccount
		Expect(r.Get(ctx, types.NamespacedName{Name: "default", Namespace: wgNamespace}, &gotSA)).To(Succeed())
		Expect(gotSA.Finalizers).ToNot(ContainElement(consts.GatewayServiceAccountFinalizer))
	})
})

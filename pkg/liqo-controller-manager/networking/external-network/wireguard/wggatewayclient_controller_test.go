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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

const (
	wgNamespace = "tenant-1"
	wgName      = "wg-client-1"
	clusterRole = "liqo-gateway-role"
	gwSAName    = "gw-sa"
)

func wgClientReconciler(objs ...client.Object) *WgGatewayClientReconciler {
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme).
		WithStatusSubresource(&networkingv1beta1.WgGatewayClient{}).
		WithObjects(objs...)
	return NewWgGatewayClientReconciler(cb.Build(), scheme.Scheme, record.NewFakeRecorder(10), clusterRole)
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

	// The WgGatewayClient is already gone (NotFound). The CRB carries the finalizer.
	// The reconciler is triggered by the CRB watch and handles cleanup in the NotFound path.

	It("requeues while gateway pods still exist", func() {
		crb := gwClusterRoleBinding()
		r := wgClientReconciler(crb, gwPod())

		res, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))

		// CRB must still exist (not deleted while pods are running).
		var gotCRB rbacv1.ClusterRoleBinding
		Expect(r.Get(ctx, types.NamespacedName{Name: crb.Name}, &gotCRB)).To(Succeed())
	})

	It("removes the SA finalizer and deletes the CRB once pods are gone", func() {
		sa := gwServiceAccount(gwSAName, true /* withFinalizer */)
		crb := gwClusterRoleBinding()
		// Add the SA as a subject so CleanupClusterRoleBindings can find it.
		crb.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      gwSAName,
			Namespace: wgNamespace,
		}}
		r := wgClientReconciler(sa, crb)

		res, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		// ServiceAccount no longer carries the gateway finalizer.
		var gotSA corev1.ServiceAccount
		Expect(r.Get(ctx, types.NamespacedName{Name: gwSAName, Namespace: wgNamespace}, &gotSA)).To(Succeed())
		Expect(gotSA.Finalizers).ToNot(ContainElement(consts.GatewayServiceAccountFinalizer))

		// CRB was deleted.
		var crbList rbacv1.ClusterRoleBindingList
		Expect(r.List(ctx, &crbList, client.MatchingLabels{
			consts.GatewayNameLabel:      wgName,
			consts.GatewayNamespaceLabel: wgNamespace,
		})).To(Succeed())
		Expect(crbList.Items).To(BeEmpty())
	})

	It("proceeds when the ServiceAccount is missing", func() {
		crb := gwClusterRoleBinding()
		crb.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      gwSAName,
			Namespace: wgNamespace,
		}}
		r := wgClientReconciler(crb)

		res, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		// CRB was deleted.
		var crbList rbacv1.ClusterRoleBindingList
		Expect(r.List(ctx, &crbList)).To(Succeed())
		Expect(crbList.Items).To(BeEmpty())
	})

	It("does not require a SA finalizer to exist (leaves a plain SA untouched)", func() {
		sa := gwServiceAccount(gwSAName, false /* no finalizer */)
		crb := gwClusterRoleBinding()
		crb.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      gwSAName,
			Namespace: wgNamespace,
		}}
		r := wgClientReconciler(sa, crb)

		_, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())

		var gotSA corev1.ServiceAccount
		Expect(r.Get(ctx, types.NamespacedName{Name: gwSAName, Namespace: wgNamespace}, &gotSA)).To(Succeed())
		Expect(gotSA.Finalizers).To(BeEmpty())
	})

	It("is a no-op when no CRBs exist", func() {
		// No CRB, no WgGatewayClient — nothing to do.
		r := wgClientReconciler()

		res, err := r.Reconcile(ctx, reqWgClient())
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))
	})
})

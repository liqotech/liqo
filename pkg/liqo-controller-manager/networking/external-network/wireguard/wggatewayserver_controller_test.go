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
	wgServerName = "wg-server-1"
)

func wgServerReconciler(objs ...client.Object) *WgGatewayServerReconciler {
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme).
		WithStatusSubresource(&networkingv1beta1.WgGatewayServer{}).
		WithObjects(objs...)
	return NewWgGatewayServerReconciler(cb.Build(), scheme.Scheme, record.NewFakeRecorder(10), clusterRole)
}

func gwServerPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-pod-s",
			Namespace: wgNamespace,
			Labels: map[string]string{
				consts.GatewayNameLabel:      wgServerName,
				consts.GatewayNamespaceLabel: wgNamespace,
			},
		},
	}
}

func gwServerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "irrelevant-crb-server",
			Labels: map[string]string{
				consts.GatewayNameLabel:      wgServerName,
				consts.GatewayNamespaceLabel: wgNamespace,
			},
			Finalizers: []string{consts.ClusterRoleBindingFinalizer},
		},
	}
}

func reqWgServer() ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: wgServerName, Namespace: wgNamespace}}
}

var _ = Describe("WgGatewayServerReconciler deletion sequencing", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	// The WgGatewayServer is already gone (NotFound). The CRB carries the finalizer.
	// The reconciler is triggered by the CRB watch and handles cleanup in the NotFound path.

	It("requeues while gateway pods still exist", func() {
		crb := gwServerClusterRoleBinding()
		r := wgServerReconciler(crb, gwServerPod())

		res, err := r.Reconcile(ctx, reqWgServer())
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))

		// CRB finalizer must NOT have been removed yet.
		var gotCRB rbacv1.ClusterRoleBinding
		Expect(r.Get(ctx, types.NamespacedName{Name: crb.Name}, &gotCRB)).To(Succeed())
		Expect(gotCRB.Finalizers).To(ContainElement(consts.ClusterRoleBindingFinalizer))
	})

	It("removes SA finalizer, removes CRB finalizer, and deletes CRB when everything is gone", func() {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:       gwSAName,
				Namespace:  wgNamespace,
				Finalizers: []string{consts.GatewayServiceAccountFinalizer},
			},
		}
		crb := gwServerClusterRoleBinding()
		crb.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      gwSAName,
			Namespace: wgNamespace,
		}}
		r := wgServerReconciler(sa, crb)

		res, err := r.Reconcile(ctx, reqWgServer())
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		var gotSA corev1.ServiceAccount
		Expect(r.Get(ctx, types.NamespacedName{Name: gwSAName, Namespace: wgNamespace}, &gotSA)).To(Succeed())
		Expect(gotSA.Finalizers).ToNot(ContainElement(consts.GatewayServiceAccountFinalizer))

		var crbList rbacv1.ClusterRoleBindingList
		Expect(r.List(ctx, &crbList, client.MatchingLabels{
			consts.GatewayNameLabel:      wgServerName,
			consts.GatewayNamespaceLabel: wgNamespace,
		})).To(Succeed())
		Expect(crbList.Items).To(BeEmpty())
	})

	It("tolerates a missing ServiceAccount", func() {
		crb := gwServerClusterRoleBinding()
		crb.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      gwSAName,
			Namespace: wgNamespace,
		}}
		r := wgServerReconciler(crb)

		res, err := r.Reconcile(ctx, reqWgServer())
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))
	})

	It("is a no-op when no CRBs with the finalizer exist", func() {
		r := wgServerReconciler()

		res, err := r.Reconcile(ctx, reqWgServer())
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))
	})
})

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
	wgServerName = "wg-server-1"
)

func wgServerPending(saName string) *networkingv1beta1.WgGatewayServer {
	now := metav1.Now()
	o := &networkingv1beta1.WgGatewayServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              wgServerName,
			Namespace:         wgNamespace,
			DeletionTimestamp: &now,
			Finalizers:        []string{consts.ClusterRoleBindingFinalizer},
		},
	}
	o.Spec.Deployment.Spec.Template.Spec.ServiceAccountName = saName
	return o
}

func wgServerReconciler(objs ...client.Object) *WgGatewayServerReconciler {
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme).
		WithStatusSubresource(&networkingv1beta1.WgGatewayServer{}).
		WithObjects(objs...)
	return NewWgGatewayServerReconciler(cb.Build(), scheme.Scheme, record.NewFakeRecorder(10), clusterRole)
}

func gwServerDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forge.GatewayResourceName(wgServerName),
			Namespace: wgNamespace,
		},
	}
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

	It("triggers Deployment deletion and requeues while the Deployment still exists", func() {
		wg := wgServerPending(gwSAName)
		r := wgServerReconciler(wg, gwServerDeployment())

		res, err := r.Reconcile(ctx, reqWgServer())
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))

		var got networkingv1beta1.WgGatewayServer
		Expect(r.Get(ctx, reqWgServer().NamespacedName, &got)).To(Succeed())
		Expect(got.Finalizers).To(ContainElement(consts.ClusterRoleBindingFinalizer))
	})

	It("requeues without re-deleting when the Deployment is already deletion-pending", func() {
		wg := wgServerPending(gwSAName)
		d := gwServerDeployment()
		now := metav1.Now()
		d.DeletionTimestamp = &now
		d.Finalizers = []string{keepFinalizer}
		r := wgServerReconciler(wg, d)

		res, err := r.Reconcile(ctx, reqWgServer())
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))
	})

	It("requeues while pods still exist after the Deployment is gone", func() {
		wg := wgServerPending(gwSAName)
		r := wgServerReconciler(wg, gwServerPod())

		res, err := r.Reconcile(ctx, reqWgServer())
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))
	})

	It("removes SA finalizer, deletes CRB, removes wg finalizer when everything is gone", func() {
		wg := wgServerPending(gwSAName)
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:       gwSAName,
				Namespace:  wgNamespace,
				Finalizers: []string{consts.GatewayServiceAccountFinalizer},
			},
		}
		r := wgServerReconciler(wg, sa, gwServerClusterRoleBinding())

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

		var got networkingv1beta1.WgGatewayServer
		err = r.Get(ctx, reqWgServer().NamespacedName, &got)
		if err == nil {
			Expect(got.Finalizers).ToNot(ContainElement(consts.ClusterRoleBindingFinalizer))
		} else {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
	})

	It("tolerates a missing ServiceAccount", func() {
		wg := wgServerPending(gwSAName)
		r := wgServerReconciler(wg, gwServerClusterRoleBinding())

		res, err := r.Reconcile(ctx, reqWgServer())
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))
	})

	It("defaults the SA name to '"+defaultServiceAccountName+"' when none is specified", func() {
		wg := wgServerPending("")
		defaultSA := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:       defaultServiceAccountName,
				Namespace:  wgNamespace,
				Finalizers: []string{consts.GatewayServiceAccountFinalizer},
			},
		}
		r := wgServerReconciler(wg, defaultSA, gwServerClusterRoleBinding())

		_, err := r.Reconcile(ctx, reqWgServer())
		Expect(err).ToNot(HaveOccurred())

		var gotSA corev1.ServiceAccount
		Expect(r.Get(ctx, types.NamespacedName{Name: defaultServiceAccountName, Namespace: wgNamespace}, &gotSA)).To(Succeed())
		Expect(gotSA.Finalizers).ToNot(ContainElement(consts.GatewayServiceAccountFinalizer))
	})
})

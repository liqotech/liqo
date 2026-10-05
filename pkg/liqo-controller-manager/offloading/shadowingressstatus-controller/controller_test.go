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

package shadowingressstatusctrl_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	shadowingressstatusctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/shadowingressstatus-controller"
)

const (
	testIngressName    = "test-ingress"
	testNamespace      = "default"
	testCluster1       = "cluster1"
	deletedIngressName = "deleted-ingress"
	ingressNameLabel   = "liqo.io/ingress-name"
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, offloadingv1beta1.AddToScheme(scheme))
	require.NoError(t, netv1.AddToScheme(scheme))

	ctx := context.Background()

	ingress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testIngressName,
			Namespace: testNamespace,
		},
	}

	shadow1 := &offloadingv1beta1.ShadowIngressStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress-cluster1",
			Namespace: testNamespace,
			Labels:    map[string]string{ingressNameLabel: testIngressName},
		},
		Spec: offloadingv1beta1.ShadowIngressStatusSpec{
			IngressName: testIngressName,
			ClusterID:   testCluster1,
			LoadBalancer: netv1.IngressLoadBalancerStatus{
				Ingress: []netv1.IngressLoadBalancerIngress{{IP: "1.2.3.4"}},
			},
		},
	}

	shadow2 := &offloadingv1beta1.ShadowIngressStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress-cluster2",
			Namespace: testNamespace,
			Labels:    map[string]string{ingressNameLabel: testIngressName},
		},
		Spec: offloadingv1beta1.ShadowIngressStatusSpec{
			IngressName: testIngressName,
			ClusterID:   "cluster2",
			LoadBalancer: netv1.IngressLoadBalancerStatus{
				Ingress: []netv1.IngressLoadBalancerIngress{{IP: "5.6.7.8"}},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(ingress, shadow1, shadow2).
		WithStatusSubresource(&netv1.Ingress{}).
		Build()

	rec := &shadowingressstatusctrl.Reconciler{Client: cl, Scheme: scheme}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: testIngressName, Namespace: testNamespace}})
	require.NoError(t, err)

	var updated netv1.Ingress
	require.NoError(t, cl.Get(ctx, types.NamespacedName{Name: testIngressName, Namespace: testNamespace}, &updated))
	require.Len(t, updated.Status.LoadBalancer.Ingress, 2)
	assert.Equal(t, "1.2.3.4", updated.Status.LoadBalancer.Ingress[0].IP)
	assert.Equal(t, "5.6.7.8", updated.Status.LoadBalancer.Ingress[1].IP)
}

func TestCleanupOrphanShadows(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, offloadingv1beta1.AddToScheme(scheme))
	require.NoError(t, netv1.AddToScheme(scheme))

	ctx := context.Background()

	shadow := &offloadingv1beta1.ShadowIngressStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orphan",
			Namespace: testNamespace,
			Labels:    map[string]string{ingressNameLabel: deletedIngressName},
		},
		Spec: offloadingv1beta1.ShadowIngressStatusSpec{
			IngressName: deletedIngressName,
			ClusterID:   testCluster1,
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(shadow).Build()

	rec := &shadowingressstatusctrl.Reconciler{Client: cl, Scheme: scheme}
	_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: deletedIngressName, Namespace: testNamespace}})
	require.NoError(t, err)

	var shadows offloadingv1beta1.ShadowIngressStatusList
	require.NoError(t, cl.List(ctx, &shadows, client.InNamespace(testNamespace)))
	assert.Empty(t, shadows.Items)
}

// Copyright 2019-2025 The Liqo Authors
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

package tenantcontroller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

func TestTenantController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TenantController Suite")
}

var _ = Describe("ensureForeignCluster", func() {
	var (
		ctx              context.Context
		fakeClient       client.Client
		tenantReconciler *TenantReconciler
		testTenant       *authv1beta1.Tenant
		testClusterID    liqov1beta1.ClusterID
	)

	BeforeEach(func() {
		ctx = context.Background()
		testClusterID = "test-cluster"

		s := runtime.NewScheme()
		Expect(authv1beta1.AddToScheme(s)).To(Succeed())
		Expect(liqov1beta1.AddToScheme(s)).To(Succeed())
		Expect(scheme.AddToScheme(s)).To(Succeed())

		fakeClient = fake.NewClientBuilder().WithScheme(s).Build()

		tenantReconciler = &TenantReconciler{
			Client: fakeClient,
			Scheme: s,
		}

		testTenant = &authv1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      string(testClusterID),
				Namespace: "test-namespace",
			},
			Spec: authv1beta1.TenantSpec{
				ClusterID: testClusterID,
			},
		}
	})

	It("should create a ForeignCluster when it doesn't exist", func() {
		err := tenantReconciler.ensureForeignCluster(ctx, testTenant)
		Expect(err).ToNot(HaveOccurred())

		var fc liqov1beta1.ForeignCluster
		err = fakeClient.Get(ctx, client.ObjectKey{Name: string(testClusterID)}, &fc)
		Expect(err).ToNot(HaveOccurred())
		Expect(fc.Spec.ClusterID).To(Equal(testClusterID))
		Expect(fc.Labels).To(HaveKeyWithValue("liqo.io/remote-cluster-id", string(testClusterID)))
	})

	It("should not error when ForeignCluster already exists", func() {
		existingFC := &liqov1beta1.ForeignCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: string(testClusterID),
			},
			Spec: liqov1beta1.ForeignClusterSpec{
				ClusterID: testClusterID,
			},
		}
		Expect(fakeClient.Create(ctx, existingFC)).To(Succeed())

		err := tenantReconciler.ensureForeignCluster(ctx, testTenant)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should handle multiple calls idempotently", func() {
		// First call
		err := tenantReconciler.ensureForeignCluster(ctx, testTenant)
		Expect(err).ToNot(HaveOccurred())

		// Second call should not error
		err = tenantReconciler.ensureForeignCluster(ctx, testTenant)
		Expect(err).ToNot(HaveOccurred())

		// Verify only one ForeignCluster exists
		var fcList liqov1beta1.ForeignClusterList
		err = fakeClient.List(ctx, &fcList)
		Expect(err).ToNot(HaveOccurred())
		Expect(fcList.Items).To(HaveLen(1))
	})

	It("should create ForeignCluster with correct labels", func() {
		err := tenantReconciler.ensureForeignCluster(ctx, testTenant)
		Expect(err).ToNot(HaveOccurred())

		var fc liqov1beta1.ForeignCluster
		err = fakeClient.Get(ctx, client.ObjectKey{Name: string(testClusterID)}, &fc)
		Expect(err).ToNot(HaveOccurred())
		Expect(fc.Labels).ToNot(BeNil())
		Expect(fc.Labels["liqo.io/remote-cluster-id"]).To(Equal(string(testClusterID)))
	})
})

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
//

package tenant_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	tenantwk "github.com/liqotech/liqo/pkg/webhooks/tenant"
)

var _ = Describe("Validating webhook tests", func() {
	const (
		clusterID = "fake-cluster"
		nsName    = "fake-namespace"
	)

	Describe("Test the Tenant field validations", func() {

		Context("Test the tenantConsistencyChecks", func() {
			It("Should return an error if the Tenant is not created in a tenant namespace", func() {
				fakeNs := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fake-namespace",
					},
				}
				fakeTenant := generateFakeTenant("fake-tenant", "fake-namespace", "fake-cluster")
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).
					WithObjects(fakeNs).
					Build()

				validator := tenantwk.NewValidator(fakeClient)

				res := validator.Handle(context.TODO(), generateAdmissionRequest(fakeTenant, admissionv1.Create))
				Expect(res.Allowed).To(BeFalse(), "Expected request to be denied")
				Expect(res.Result.Message).To(ContainSubstring("must be created in a tenant namespace"))
				Expect(res.Result.Code).To(Equal(int32(http.StatusBadRequest)))
			})

			It("Should return an error if a new Tenant cluster ID does not match the one in the tenant namespace", func() {
				tenantNamespace := testutil.FakeNamespaceWithClusterID(liqov1beta1.ClusterID(clusterID), nsName)
				newTenant := generateFakeTenant("new-tenant", nsName, "something-else")
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).
					WithObjects(tenantNamespace).
					Build()

				validator := tenantwk.NewValidator(fakeClient)

				res := validator.Handle(context.TODO(), generateAdmissionRequest(newTenant, admissionv1.Create))
				Expect(res.Allowed).To(BeFalse(), "Expected request to be denied")
				Expect(res.Result.Message).To(ContainSubstring("Tenant must have the same cluster ID"))
				Expect(res.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})

			It("Should return an error if a new Tenant cluster ID does not match its liqo.io/remote-cluster-id label", func() {
				tenantNamespace := testutil.FakeNamespaceWithClusterID(liqov1beta1.ClusterID(clusterID), nsName)
				newTenant := generateFakeTenant("new-tenant", nsName, clusterID)
				newTenant.Labels[consts.RemoteClusterID] = "something-else"
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).
					WithObjects(tenantNamespace).
					Build()

				validator := tenantwk.NewValidator(fakeClient)

				res := validator.Handle(context.TODO(), generateAdmissionRequest(newTenant, admissionv1.Create))
				Expect(res.Allowed).To(BeFalse(), "Expected request to be denied")
				Expect(res.Result.Message).To(ContainSubstring("label must match"))
				Expect(res.Result.Code).To(Equal(int32(http.StatusBadRequest)))
			})
		})

		Context("Test webhook on creation", func() {
			It("Should allow valid creation", func() {
				tenantNamespace := testutil.FakeNamespaceWithClusterID(liqov1beta1.ClusterID(clusterID), nsName)
				newTenant := generateFakeTenant("my-tenant", nsName, clusterID)
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).
					WithIndex(&authv1beta1.Tenant{}, "metadata.name", tenantwk.NameExtractor).
					WithObjects(tenantNamespace).
					Build()

				validator := tenantwk.NewValidator(fakeClient)

				res := validator.Handle(context.TODO(), generateAdmissionRequest(newTenant, admissionv1.Create))
				Expect(res.Allowed).To(BeTrue(), "Expected request to be accepted")
			})

			It("Should return an error if a new Tenant is created in a tenant namespace, which already contains one", func() {
				tenantNamespace := testutil.FakeNamespaceWithClusterID(liqov1beta1.ClusterID(clusterID), nsName)
				existingTenant := generateFakeTenant("existing-tenant", nsName, clusterID)
				newTenant := generateFakeTenant("new-tenant", nsName, clusterID)
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).
					WithObjects(tenantNamespace, existingTenant).
					Build()

				validator := tenantwk.NewValidator(fakeClient)

				res := validator.Handle(context.TODO(), generateAdmissionRequest(newTenant, admissionv1.Create))
				Expect(res.Allowed).To(BeFalse(), "Expected request to be denied")
				Expect(res.Result.Message).To(ContainSubstring("already exists"))
				Expect(res.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})

			It("Should return an error if a new Tenant with the same name exists in the cluster", func() {
				tenantNamespace := testutil.FakeNamespaceWithClusterID(liqov1beta1.ClusterID(clusterID), nsName)
				existingTenant := generateFakeTenant("same-name", "another-namespace", clusterID)
				newTenant := generateFakeTenant("same-name", nsName, clusterID)
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).
					WithIndex(&authv1beta1.Tenant{}, "metadata.name", tenantwk.NameExtractor).
					WithObjects(tenantNamespace, existingTenant).
					Build()

				validator := tenantwk.NewValidator(fakeClient)

				res := validator.Handle(context.TODO(), generateAdmissionRequest(newTenant, admissionv1.Create))
				Expect(res.Allowed).To(BeFalse(), "Expected request to be denied")
				Expect(res.Result.Message).To(ContainSubstring("unique name across the cluster"))
				Expect(res.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})
		})

		Context("Test webhook on update", func() {
			It("Should allow valid update", func() {
				tenantNamespace := testutil.FakeNamespaceWithClusterID(liqov1beta1.ClusterID(clusterID), nsName)
				newTenant := generateFakeTenant("my-tenant", nsName, clusterID)
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).
					WithIndex(&authv1beta1.Tenant{}, "metadata.name", tenantwk.NameExtractor).
					WithObjects(tenantNamespace, newTenant).
					Build()

				validator := tenantwk.NewValidator(fakeClient)

				res := validator.Handle(context.TODO(), generateAdmissionRequest(newTenant, admissionv1.Update))
				Expect(res.Allowed).To(BeTrue(), "Expected request to be accepted")
			})

			It("Should return an error if a new Tenant with the same name exists in the cluster", func() {
				tenantNamespace := testutil.FakeNamespaceWithClusterID(liqov1beta1.ClusterID(clusterID), nsName)
				existingTenant := generateFakeTenant("same-name", "another-namespace", clusterID)
				newTenant := generateFakeTenant("same-name", nsName, clusterID)
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).
					WithIndex(&authv1beta1.Tenant{}, "metadata.name", tenantwk.NameExtractor).
					WithObjects(tenantNamespace, existingTenant).
					Build()

				validator := tenantwk.NewValidator(fakeClient)

				res := validator.Handle(context.TODO(), generateAdmissionRequest(newTenant, admissionv1.Update))
				Expect(res.Allowed).To(BeFalse(), "Expected request to be denied")
				Expect(res.Result.Message).To(ContainSubstring("unique name across the cluster"))
				Expect(res.Result.Code).To(Equal(int32(http.StatusForbidden)))
			})
		})
	})
})

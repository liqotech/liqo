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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	tenantwk "github.com/liqotech/liqo/pkg/webhooks/tenant"
)

var _ = Describe("Mutation webhook tests", func() {
	const (
		clusterID = "fake-cluster"
		nsName    = "fake-namespace"
	)

	var (
		fakeClient     client.Client
		expectedResult = generateFakeTenant("fake-tenant", nsName, clusterID)
	)

	BeforeEach(func() {
		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			Build()
	})

	DescribeTable("Test liqo.io/remote-cluster-id injection",
		func(operation admissionv1.Operation, expectedPatches bool) {
			tenant := expectedResult.DeepCopy()
			if expectedPatches {
				tenant.Labels = nil
			}

			validator := tenantwk.NewMutator(fakeClient)

			res := validator.Handle(context.TODO(), generateAdmissionRequest(tenant, operation))
			Expect(res.Allowed).To(BeTrue(), "Expected request to be accepted")

			if expectedPatches {
				patch, err := jsonpatch.CreatePatch(tenantToRawExtension(tenant).Raw, tenantToRawExtension(expectedResult).Raw)
				Expect(err).To(BeNil(), "Expected no errors while creating the patch")
				Expect(res.Patches).To(Equal(patch), "Patch is not equal the expected one")
			} else {
				Expect(res.Patches).To(BeNil(), "Expected no patches")
			}
		},
		Entry("Create: Should not change the Tenant resource when the label is already present", admissionv1.Create, false),
		Entry("Create: Should add the liqo.io/remote-cluster-id label when not present", admissionv1.Create, true),
		Entry("Update: Should not change the Tenant resource when the label is already present", admissionv1.Update, false),
		Entry("Update: Should add the liqo.io/remote-cluster-id label when not present", admissionv1.Update, true),
	)
})

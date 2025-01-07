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

package identitymanager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	idManTest "github.com/liqotech/liqo/pkg/identityManager/testUtils"
	"github.com/liqotech/liqo/pkg/utils/csr"
)

var _ = Describe("IdentityManager", func() {
	Context("Remote Manager", Ordered, func() {

		var csrBytes []byte
		var err error
		var stopChan chan struct{}

		BeforeAll(func() {
			_, csrBytes, err = csr.NewKeyAndRequest("foobar")
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			stopChan = make(chan struct{})
			idManTest.StartTestApprover(k8sClient, stopChan)
		})

		AfterEach(func() {
			close(stopChan)
		})

		It("Approve Signing Request", func() {
			opts := &SigningRequestOptions{
				Cluster:         remoteCluster,
				SigningRequest:  csrBytes,
				IdentityType:    authv1beta1.ControlPlaneIdentityType,
				TenantNamespace: namespace.Name,
			}
			certificate, err := identityProvider.ApproveSigningRequest(ctx, opts)
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate.Certificate).To(Equal([]byte(idManTest.FakeCRT)))
		})

		It("Retrieve Remote Certificate", func() {
			opts := &SigningRequestOptions{
				Cluster:         remoteCluster,
				SigningRequest:  csrBytes,
				IdentityType:    authv1beta1.ControlPlaneIdentityType,
				TenantNamespace: namespace.Name,
			}
			certificate, err := identityProvider.GetRemoteCertificate(ctx, opts)
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate.Certificate).To(Equal([]byte(idManTest.FakeCRT)))
		})

	})

	Context("Identity Provider", func() {

		It("Certificate Identity Provider", func() {
			idProvider := NewCertificateIdentityProvider(ctx,
				mgr.GetClient(), cluster.GetClient(), cluster.GetCfg(),
				localCluster, namespaceManager)

			certIDManager, ok := idProvider.(*identityManager)
			Expect(ok).To(BeTrue())

			_, ok = certIDManager.IdentityProvider.(*certificateIdentityProvider)
			Expect(ok).To(BeTrue())
		})

	})

})

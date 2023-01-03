// Copyright 2019-2023 The Liqo Authors
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

package authservice

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	autherrors "github.com/liqotech/liqo/pkg/auth/errors"
	csrutil "github.com/liqotech/liqo/pkg/utils/csr"
)

type tokenManagerMock struct {
	token string
}

func (man *tokenManagerMock) getToken() (string, error) {
	return man.token, nil
}

func (man *tokenManagerMock) createToken() error {
	man.token = "token"
	return nil
}

var _ = Describe("Auth", func() {

	Context("Token", func() {

		It("Create Token", func() {
			err := authService.createToken()
			Expect(err).To(BeNil())
		})

		It("Get Token", func() {
			Eventually(func() int {
				token, _ := authService.getToken()
				return len(token)
			}).Should(Equal(128))
		})

	})

	Context("Credential Validator", func() {

		type credentialValidatorTestcase struct {
			credentials    auth.ServiceAccountIdentityRequest
			authEnabled    bool
			expectedOutput types.GomegaMatcher
		}

		DescribeTable("Credential Validator table",
			func(c credentialValidatorTestcase) {
				err := authService.credentialsValidator.checkCredentials(&c.credentials, &tMan, c.authEnabled)
				Expect(err).To(c.expectedOutput)
			},

			Entry("empty token accepted", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:           "",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: "test", ClusterName: "test"},
				},
				authEnabled:    false,
				expectedOutput: BeNil(),
			}),

			Entry("empty token denied", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:           "",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: "test", ClusterName: "test"},
				},
				authEnabled:    true,
				expectedOutput: HaveOccurred(),
			}),

			Entry("token accepted", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:           "token",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: "test", ClusterName: "test"},
				},
				authEnabled:    true,
				expectedOutput: BeNil(),
			}),

			Entry("token denied", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:           "token-wrong",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: "test", ClusterName: "test"},
				},
				authEnabled:    true,
				expectedOutput: HaveOccurred(),
			}),
		)

	})

	Context("Certificate Identity Creation", func() {

		var (
			oldAuthEnabled bool
		)

		type certificateTestcase struct {
			request          auth.CertificateIdentityRequest
			expectedOutput   types.GomegaMatcher
			expectedResponse func(*auth.CertificateIdentityResponse)
		}

		BeforeEach(func() {
			oldAuthEnabled = authService.authenticationEnabled
			authService.authenticationEnabled = false
		})

		AfterEach(func() {
			authService.authenticationEnabled = oldAuthEnabled
		})

		DescribeTable("Certificate Identity Creation table",
			func(c certificateTestcase) {
				_, req, err := csrutil.NewKeyAndRequest(authService.localCluster.ClusterID)
				Expect(err).To(BeNil())
				c.request.CertificateSigningRequest = base64.StdEncoding.EncodeToString(req)

				response, err := authService.handleIdentity(ctx, c.request)
				Expect(err).To(c.expectedOutput)
				c.expectedResponse(response)
			},

			Entry("first creation", certificateTestcase{
				request: auth.CertificateIdentityRequest{
					ClusterIdentity:           discoveryv1alpha1.ClusterIdentity{ClusterID: "cluster1", ClusterName: "cluster1"},
					CertificateSigningRequest: string(csr),
				},
				expectedOutput: BeNil(),
				expectedResponse: func(resp *auth.CertificateIdentityResponse) {
					Expect(resp).NotTo(BeNil())
				},
			}),

			Entry("second creation", certificateTestcase{
				request: auth.CertificateIdentityRequest{
					ClusterIdentity:           discoveryv1alpha1.ClusterIdentity{ClusterID: "cluster1", ClusterName: "cluster1"},
					CertificateSigningRequest: string(csr),
				},
				expectedOutput: HaveOccurred(),
				expectedResponse: func(resp *auth.CertificateIdentityResponse) {
					Expect(resp).To(BeNil())
				},
			}),

			Entry("create different one", certificateTestcase{
				request: auth.CertificateIdentityRequest{
					ClusterIdentity:           discoveryv1alpha1.ClusterIdentity{ClusterID: "cluster2", ClusterName: "cluster2"},
					CertificateSigningRequest: string(csr),
				},
				expectedOutput: BeNil(),
				expectedResponse: func(resp *auth.CertificateIdentityResponse) {
					Expect(resp).NotTo(BeNil())
				},
			}),
		)
	})

	Context("errorHandler", func() {

		type errorHandlerTestcase struct {
			err  error
			body []byte
			code int
		}

		DescribeTable("errorHandler table",
			func(c errorHandlerTestcase) {
				recorder := httptest.NewRecorder()
				authService.handleError(recorder, c.err)

				recorder.Flush()

				body, err := io.ReadAll(recorder.Body)
				Expect(err).To(Succeed())
				Expect(string(body)).To(ContainSubstring(string(c.body)))
				Expect(recorder.Code).To(Equal(c.code))
			},

			Entry("generic error", errorHandlerTestcase{
				err:  fmt.Errorf("generic error"),
				body: []byte("generic error"),
				code: http.StatusInternalServerError,
			}),

			Entry("status error", errorHandlerTestcase{
				err:  kerrors.NewForbidden(discoveryv1alpha1.ForeignClusterGroupResource, "test", fmt.Errorf("")),
				body: []byte("forbidden"),
				code: http.StatusForbidden,
			}),

			Entry("client error", errorHandlerTestcase{
				err: &autherrors.ClientError{
					Reason: "client error",
				},
				body: []byte("client error"),
				code: http.StatusBadRequest,
			}),

			Entry("authentication error", errorHandlerTestcase{
				err: &autherrors.AuthenticationFailedError{
					Reason: "invalid token",
				},
				body: []byte("invalid token"),
				code: http.StatusUnauthorized,
			}),
		)

	})

})

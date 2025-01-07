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

package csr

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

var _ = Describe("Watcher functions", func() {
	const (
		MatchingLabel    = "matching"
		NotMatchingLabel = "not-matching"

		MatchingName    = "foo"
		NotMatchingName = "bar"
	)

	Describe("The csr.Watcher functions", func() {
		var (
			watcherCancel context.CancelFunc

			client   kubernetes.Interface
			selector labels.Selector

			name, label string
			input       *certv1.CertificateSigningRequest

			watcher Watcher
		)

		CSRForger := func(name, label string) *certv1.CertificateSigningRequest {
			_, req, err := NewKeyAndRequest("foobar")
			Expect(err).ToNot(HaveOccurred())

			return &certv1.CertificateSigningRequest{
				ObjectMeta: v1.ObjectMeta{Name: name, Labels: map[string]string{label: "whatever"}},
				Spec: certv1.CertificateSigningRequestSpec{
					SignerName: "foo.com/bar",
					Usages:     []certv1.KeyUsage{certv1.UsageAny},
					Request:    req,
				},
			}
		}

		BeforeEach(func() {
			client = kubernetes.NewForConfigOrDie(cluster.GetCfg())

			req, err := labels.NewRequirement(MatchingLabel, selection.Exists, []string{})
			Expect(err).ToNot(HaveOccurred())

			selector = labels.NewSelector().Add(*req)
			watcher = NewWatcher(client, 0, selector, fields.Everything())
		})

		JustBeforeEach(func() {
			var watcherCtx context.Context
			watcherCtx, watcherCancel = context.WithCancel(ctx)
			watcher.Start(watcherCtx)

			var err error
			input = CSRForger(name, label)

			input, err = client.CertificatesV1().CertificateSigningRequests().Create(ctx, input, v1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			// Avoid mismatches when compared.
			input.ResourceVersion = "0"
		})

		JustAfterEach(func() {
			watcherCancel()
			Expect(client.CertificatesV1().CertificateSigningRequests().Delete(ctx, input.Name, v1.DeleteOptions{})).To(Succeed())
			Eventually(func() error {
				return util.Second(client.CertificatesV1().CertificateSigningRequests().Get(ctx, input.Name, v1.GetOptions{}))
			}).Should(testutil.BeNotFound())
		})

		When("the RetrieveCertificate function is executed", func() {
			var (
				retrieveCtx context.Context
				cancel      context.CancelFunc

				certificate []byte
				err         error
			)

			BeforeEach(func() {
				name = MatchingName
				label = MatchingLabel
				retrieveCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
			})

			JustBeforeEach(func() {
				cert, err := testutil.FakeSelfSignedCertificate("foobar")
				Expect(err).ToNot(HaveOccurred())

				input.Status.Certificate = cert
				input, err = client.CertificatesV1().CertificateSigningRequests().UpdateStatus(ctx, input, v1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
			})

			ContextBody := func() {
				When("the CSR matches the desired name", func() {
					It("should succeed and retrieve the certificate", func() {
						Eventually(retrieveCtx.Done()).Should(BeClosed())
						Expect(err).ToNot(HaveOccurred())
						Expect(certificate).To(Equal(input.Status.Certificate))
					})
				})

				When("the CSR does not match the desired name", func() {
					BeforeEach(func() { name = NotMatchingName })

					It("should fail and return nil as certificate", func() {
						Eventually(retrieveCtx.Done()).Should(BeClosed())
						Expect(err).To(HaveOccurred())
						Expect(certificate).To(BeNil())
					})
				})
			}

			Context("the function is started before the certificate is ready", func() {
				BeforeEach(func() {
					// Let run this in a goroutine, as it is blocking
					go func() {
						certificate, err = watcher.RetrieveCertificate(retrieveCtx, MatchingName)
						cancel()
					}()
				})

				Describe("checking the outcome", ContextBody)
			})

			Context("the function is started when the certificate is already ready", func() {
				BeforeEach(func() {
					// Let run this in a goroutine, as it is blocking
					go func() {
						// Let sleep for some time, so that the certificate is already ready when the
						// watcher.RetrieveCertificate function is invoked.
						time.Sleep(250 * time.Millisecond)
						certificate, err = watcher.RetrieveCertificate(retrieveCtx, MatchingName)
						cancel()
					}()
				})

				Describe("checking the outcome", ContextBody)
			})
		})

		When("the handlers are registered", func() {
			var (
				receiverGeneric chan *certv1.CertificateSigningRequest
				receiverNamed   chan *certv1.CertificateSigningRequest
			)

			BeforeEach(func() {
				name = MatchingName
				label = MatchingLabel

				receiverGeneric = make(chan *certv1.CertificateSigningRequest, 1)
				receiverNamed = make(chan *certv1.CertificateSigningRequest, 1)

				watcher.RegisterHandler(func(csr *certv1.CertificateSigningRequest) {
					c := csr.DeepCopy()
					c.ResourceVersion = "0"
					receiverGeneric <- c
				})
				watcher.RegisterHandlerForName(MatchingName, func(csr *certv1.CertificateSigningRequest) {
					c := csr.DeepCopy()
					c.ResourceVersion = "0"
					receiverNamed <- c
				})
			})

			When("the CSR matches both the selector and the name constraint", func() {
				It("the generic handler should correctly receive the CSR", func() {
					Eventually(receiverGeneric).Should(Receive(Equal(input)))
				})
				It("the named handler should correctly receive the CSR", func() {
					Eventually(receiverNamed).Should(Receive(Equal(input)))
				})
			})

			When("the CSR matches the selector but not the name constraint", func() {
				BeforeEach(func() { name = NotMatchingName })

				It("the generic handler should correctly receive the CSR", func() {
					Eventually(receiverGeneric).Should(Receive(Equal(input)))
				})
				It("the named handler should not be triggered", func() {
					Consistently(receiverNamed).ShouldNot(Receive())
				})
			})

			When("the CSR does not match the selector", func() {
				BeforeEach(func() { label = NotMatchingLabel })

				It("the generic handler should not be triggered", func() {
					Consistently(receiverGeneric).ShouldNot(Receive())
				})
				It("the named handler should not be triggered", func() {
					Consistently(receiverNamed).ShouldNot(Receive())
				})
			})

			When("the handlers are then unregistered", func() {
				BeforeEach(func() {
					watcher.UnregisterHandler()
					watcher.UnregisterHandlerForName(MatchingName)
				})

				It("the generic handler should not be triggered", func() {
					Consistently(receiverGeneric).ShouldNot(Receive())
				})
				It("the named handler should not be triggered", func() {
					Consistently(receiverNamed).ShouldNot(Receive())
				})
			})
		})
	})

	Describe("The csr.IsApproved function", func() {
		CSRForger := func(name string, cert []byte) *certv1.CertificateSigningRequest {
			return &certv1.CertificateSigningRequest{
				ObjectMeta: v1.ObjectMeta{Name: name},
				Status:     certv1.CertificateSigningRequestStatus{Certificate: cert},
			}
		}

		type IsApprovedCase struct {
			input    *certv1.CertificateSigningRequest
			expected bool
		}

		DescribeTable("should correctly return whether the CRS is approved",
			func(c IsApprovedCase) {
				Expect(IsApproved(c.input)).To(BeIdenticalTo(c.expected))
			},
			Entry("when the CSR is nil", IsApprovedCase{input: nil, expected: false}),
			Entry("when the CSR certificate is nil", IsApprovedCase{
				input:    &certv1.CertificateSigningRequest{},
				expected: false,
			}),
			Entry("when the CSR certificate is empty", IsApprovedCase{
				input:    CSRForger("foo", []byte{}),
				expected: false,
			}),
			Entry("when the CSR certificate is present", IsApprovedCase{
				input:    CSRForger("foo", []byte{55, 22, 57, 90}),
				expected: true,
			}),
		)
	})
})

// Copyright 2019-2022 The Liqo Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
	certificatesv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestApproveSigningRequest(t *testing.T) {
	certificateToValidate := certificatesv1.CertificateSigningRequest{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name: "to_validate",
			Labels: map[string]string{
				"liqo.io/csr": "true",
			},
		},
		Spec:   certificatesv1.CertificateSigningRequestSpec{},
		Status: certificatesv1.CertificateSigningRequestStatus{},
	}

	c := testclient.NewSimpleClientset()
	_, err := c.CertificatesV1().CertificateSigningRequests().Create(context.TODO(), &certificateToValidate, v1.CreateOptions{})
	if err != nil {
		t.Fail()
	}
	err = Approve(c, &certificateToValidate, "LiqoApproval", "This CSR was approved by Liqo Advertisement Operator")
	if err != nil {
		t.Fail()
	}
	cert, err := c.CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "to_validate", v1.GetOptions{})
	if err != nil {
		t.Fail()
	}
	assert.NotNil(t, cert)
	assert.NotEmpty(t, cert.Status.Conditions)
	conditions := cert.Status.Conditions
	assert.Equal(t, conditions[0].Type, certificatesv1.CertificateApproved)
	assert.Equal(t, conditions[0].Reason, "LiqoApproval")
	assert.Equal(t, conditions[0].Message, "This CSR was approved by Liqo Advertisement Operator")
}

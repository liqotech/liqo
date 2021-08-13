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

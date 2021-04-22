package certificateSigningRequest

import (
	"context"

	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

// ApproveCSR approves the provided CertificateSigningRequest
func ApproveCSR(clientSet k8s.Interface, csr *certificatesv1beta1.CertificateSigningRequest, reason string, message string) error {
	// certificate already added to CSR
	if csr.Status.Certificate != nil {
		return nil
	}
	// Check if the certificate is already approved but the certificate is still not available
	for _, b := range csr.Status.Conditions {
		if b.Type == "Approved" {
			return nil
		}
	}
	// Approve
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1beta1.CertificateSigningRequestCondition{
		Type:           certificatesv1beta1.CertificateApproved,
		Reason:         reason,
		Message:        message,
		LastUpdateTime: metav1.Now(),
	})
	_, errApproval := clientSet.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(context.TODO(), csr, metav1.UpdateOptions{})
	if errApproval != nil {
		return errApproval
	}
	return nil
}

package csrApprover

import (
	"context"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

func approveCSR(clientSet k8s.Interface, csr *certificatesv1beta1.CertificateSigningRequest) error {
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
		Reason:         "LiqoApproval",
		Message:        "This CSR was approved by Liqo Advertisement Operator",
		LastUpdateTime: metav1.Now(),
	})
	_, errApproval := clientSet.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(context.TODO(), csr, metav1.UpdateOptions{})
	if errApproval != nil {
		return errApproval
	}
	return nil
}

func WatchCSR(clientset k8s.Interface, label string) {
	watch, err := clientset.CertificatesV1beta1().CertificateSigningRequests().Watch(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		klog.Error(err)
	}
	go func() {
		for event := range watch.ResultChan() {
			csr, ok := event.Object.(*certificatesv1beta1.CertificateSigningRequest)
			if !ok {
				klog.Error("Unable to cast object from watch operation")
			}
			// Check if the certificare is already approved
			err := approveCSR(clientset, csr)
			if err != nil {
				klog.Error(err)
			}
		}
		watch.Stop()
	}()

}

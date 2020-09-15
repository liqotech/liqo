package csrApprover

import (
	"context"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"time"
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

func WatchCSR(clientset k8s.Interface, label string, resyncPeriod time.Duration) {

	stop := make(chan struct{})
	lo := func(options *metav1.ListOptions) {
		options.LabelSelector = label
	}
	options := []informers.SharedInformerOption{
		informers.WithTweakListOptions(lo),
	}
	informer := informers.NewSharedInformerFactoryWithOptions(clientset, resyncPeriod, options...)
	csrInformer := informer.Certificates().V1beta1().CertificateSigningRequests().Informer()
	csrInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			csr, ok := obj.(*certificatesv1beta1.CertificateSigningRequest)
			if !ok {
				klog.Error("Unable to cast object")
				return
			}
			err := approveCSR(clientset, csr)
			if err != nil {
				klog.Error(err)
			} else {
				klog.Infof("CSR %v correctly approved", csr.Name)
			}
		},
	})

	go informer.Start(stop)
}

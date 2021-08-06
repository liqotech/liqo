package csr

import (
	"context"
	"sync"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// ApproveCSR approves the provided CertificateSigningRequest.
func ApproveCSR(clientSet k8s.Interface, csr *certificatesv1.CertificateSigningRequest, reason, message string) error {
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
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:           certificatesv1.CertificateApproved,
		Reason:         reason,
		Message:        message,
		LastUpdateTime: metav1.Now(),
		Status:         corev1.ConditionTrue,
	})
	_, errApproval := clientSet.CertificatesV1().CertificateSigningRequests().
		UpdateApproval(context.TODO(), csr.Name, csr, metav1.UpdateOptions{})
	if errApproval != nil {
		return errApproval
	}
	return nil
}

// WatchCSR initializes informers to watch the creation of new CSRs issued for VirtualKubelet instances.
func WatchCSR(ctx context.Context, clientset k8s.Interface, label string, resyncPeriod time.Duration, wg *sync.WaitGroup) {
	defer wg.Done()
	informer := informers.NewSharedInformerFactoryWithOptions(clientset, resyncPeriod, informers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.LabelSelector = label
		}))
	csrInformer := informer.Certificates().V1().CertificateSigningRequests().Informer()
	csrInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			csr, ok := obj.(*certificatesv1.CertificateSigningRequest)
			if !ok {
				klog.Error("Unable to cast object")
				return
			}
			err := ApproveCSR(clientset, csr, "LiqoApproval", "This CSR was approved by Liqo Advertisement Operator")
			if err != nil {
				klog.Error(err)
			} else {
				klog.Infof("CSR %v correctly approved", csr.Name)
			}
		},
	})

	go informer.Start(ctx.Done())
}

// ForgeInformer returns a SharedInformer. The informer watches a CSR, which name is specified by the name parameter,
// and returns a valid CRT, by pushing it inside the crtChan.
func ForgeInformer(client k8s.Interface, name string, crtChan chan []byte) cache.SharedIndexInformer {
	var resyncPeriod = 10 * time.Second
	var fieldSelector = map[string]string{
		"metadata.name": name,
	}
	informer := informers.NewSharedInformerFactoryWithOptions(client, resyncPeriod, informers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.FieldSelector = labels.SelectorFromSet(fieldSelector).String()
		}))
	csrInformer := informer.Certificates().V1().CertificateSigningRequests().Informer()
	csrInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(newObj interface{}) {
			csrResource, ok := newObj.(*certificatesv1.CertificateSigningRequest)
			if !ok {
				klog.Error("Unable to cast object")
				return
			}
			if len(csrResource.Status.Certificate) > 0 {
				klog.Infof("Certificate retrieved for CSR %s", csrResource.Name)
				crtChan <- csrResource.Status.Certificate
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			csrResource, ok := newObj.(*certificatesv1.CertificateSigningRequest)
			if !ok {
				klog.Error("Unable to cast object")
				return
			}
			if len(csrResource.Status.Certificate) > 0 {
				klog.Infof("Certificate retrieved for CSR %s", csrResource.Name)
				crtChan <- csrResource.Status.Certificate
			}
		},
	})
	return csrInformer
}

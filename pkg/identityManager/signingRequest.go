package identityManager

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	certv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// random package initialization
func init() {
	rand.Seed(time.Now().UnixNano())
}

// Approve a remote CertificateSigningRequest.
// It creates a CertificateSigningRequest CR to be issued by the local cluster, and approves it.
// This function will wait (with a timeout) for an available certificate before returning.
func (certManager *certificateIdentityManager) ApproveSigningRequest(signingRequest []byte) (certificate []byte, err error) {
	rnd := fmt.Sprintf("%v", rand.Int63())

	// TODO: move client-go to a newer version to use certificates/v1
	cert := &certv1beta1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: identitySecretRoot,
			Labels: map[string]string{
				// the informer needs to select it by label, this is a temporal ID for this request
				randomIDLabel: rnd,
			},
		},
		Spec: certv1beta1.CertificateSigningRequestSpec{
			Groups: []string{
				"system:authenticated",
			},
			Request: signingRequest,
			Usages: []certv1beta1.KeyUsage{
				certv1beta1.UsageDigitalSignature,
				certv1beta1.UsageKeyEncipherment,
				certv1beta1.UsageClientAuth,
			},
		},
	}

	cert, err = certManager.client.CertificatesV1beta1().CertificateSigningRequests().Create(context.TODO(), cert, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	cert.Status.Conditions = append(cert.Status.Conditions, certv1beta1.CertificateSigningRequestCondition{
		Type:           certv1beta1.CertificateApproved,
		Reason:         "IdentityManagerApproval",
		Message:        "This CSR was approved by Liqo Identity Manager",
		LastUpdateTime: metav1.Now(),
	})

	cert, err = certManager.client.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(context.TODO(), cert, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return certManager.getCertificate(cert, rnd)
}

// Retrieve the certificate given the CertificateSigningRequest and its randomID.
// If the certificate is not ready yet, it will wait for it (with a timeout)
func (certManager *certificateIdentityManager) getCertificate(csr *certv1beta1.CertificateSigningRequest, randomID string) ([]byte, error) {
	var certificate []byte

	// define a function that will check if a generic object is a CSR with a issued certificate
	checkCertificate := func(obj interface{}) bool {
		csr, ok := obj.(*certv1beta1.CertificateSigningRequest)
		if !ok {
			klog.Errorf("this object is not a CertificateSigningRequest: %v", obj)
			return false
		}

		res := (csr.Status.Certificate != nil && len(csr.Status.Certificate) > 0)
		if res {
			certificate = csr.Status.Certificate
		}
		return res
	}

	if checkCertificate(csr) {
		// the csr is already valid, don't wait for the certificate
		return csr.Status.Certificate, nil
	}

	// create an informer to be notified when the certificate will be available
	// this informer will only watch one CSR, thanks to the random ID
	labelSelector := labels.Set(map[string]string{
		randomIDLabel: randomID,
	}).AsSelector()

	informer := cache.NewSharedIndexInformer(&cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = labelSelector.String()
			return certManager.client.CertificatesV1beta1().CertificateSigningRequests().List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = labelSelector.String()
			return certManager.client.CertificatesV1beta1().CertificateSigningRequests().Watch(context.TODO(), options)
		},
	}, &certv1beta1.CertificateSigningRequest{}, 0, cache.Indexers{})

	stopChan := make(chan struct{})
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if checkCertificate(obj) {
				close(stopChan)
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			if checkCertificate(newObj) {
				close(stopChan)
			}
		},
	})

	go informer.Run(stopChan)

	// wait for the certificate, with a timeout
	select {
	case <-stopChan:
		// finished successfully
		return certificate, nil
	case <-time.NewTimer(30 * time.Second).C:
		err := fmt.Errorf("timeout exceeded waiting for the approved certificate")
		klog.Error(err)
		close(stopChan)
		return nil, err
	}
}

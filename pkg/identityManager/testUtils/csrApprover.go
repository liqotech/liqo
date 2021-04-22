package identityManagerTestUtils

import (
	"context"
	"os"

	certv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

func StartTestApprover(client kubernetes.Interface, stopChan <-chan struct{}) {
	// we need an informer to fill the certificate field, since no api server is running
	informer := cache.NewSharedIndexInformer(&cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return client.CertificatesV1beta1().CertificateSigningRequests().List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return client.CertificatesV1beta1().CertificateSigningRequests().Watch(context.TODO(), options)
		},
	}, &certv1beta1.CertificateSigningRequest{}, 0, cache.Indexers{})

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			csr, ok := newObj.(*certv1beta1.CertificateSigningRequest)
			if !ok {
				klog.Info("not a csr")
				os.Exit(1)
			}

			if csr.Status.Certificate == nil {
				csr.Status.Certificate = []byte("test")
				_, _ = client.CertificatesV1beta1().CertificateSigningRequests().UpdateStatus(context.TODO(), csr, metav1.UpdateOptions{})
			}
		},
	})

	go informer.Run(stopChan)
}

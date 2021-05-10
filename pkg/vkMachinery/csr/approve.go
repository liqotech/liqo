package csr

import (
	"time"

	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/liqotech/liqo/pkg/certificateSigningRequest"
)

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
			err := certificateSigningRequest.ApproveCSR(clientset, csr, "LiqoApproval", "This CSR was approved by Liqo Advertisement Operator")
			if err != nil {
				klog.Error(err)
			} else {
				klog.Infof("CSR %v correctly approved", csr.Name)
			}
		},
	})

	go informer.Start(stop)
}

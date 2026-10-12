// Copyright 2019-2026 The Liqo Authors
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

package identitymanagertestutils

import (
	"context"
	"os"

	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// FakeCRT is the fake CRT returned by the TestApprover as valid CRT.
var FakeCRT = `
-----BEGIN CERTIFICATE-----
MIIBvzCCAWWgAwIBAgIRAMd7Mz3fPrLm1aFUn02lLHowCgYIKoZIzj0EAwIwIzEh
MB8GA1UEAwwYazNzLWNsaWVudC1jYUAxNjE2NDMxOTU2MB4XDTIxMDQxOTIxNTMz
MFoXDTIyMDQxOTIxNTMzMFowMjEVMBMGA1UEChMMc3lzdGVtOm5vZGVzMRkwFwYD
VQQDExBzeXN0ZW06bm9kZTp0ZXN0MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE
Xd9aZm6nftepZpUwof9RSUZqZDgu7dplIiDt8nnhO5Bquy2jn7/AVx20xb0Xz0d2
XLn3nn5M+lR2p3NlZmqWHaNrMGkwDgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoG
CCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwHwYDVR0jBBgwFoAU/fZa5enijRDB25DF
NT1/vPUy/hMwEwYDVR0RBAwwCoIIRE5TOnRlc3QwCgYIKoZIzj0EAwIDSAAwRQIg
b3JL5+Q3zgwFrciwfdgtrKv8MudlA0nu6EDQO7eaJbwCIQDegFyC4tjGPp/5JKqQ
kovW9X7Ook/tTW0HyX6D6HRciA==
-----END CERTIFICATE-----
`

// StartTestApprover mocks the CSRApprover.
// When a CSR is approved, it injects a fake certificate to fill the .status.Certificate field.
func StartTestApprover(client kubernetes.Interface, stopChan <-chan struct{}) {
	// we need an informer to fill the certificate field, since no api server is running
	informer := cache.NewSharedIndexInformer(&cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return client.CertificatesV1().CertificateSigningRequests().List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return client.CertificatesV1().CertificateSigningRequests().Watch(context.TODO(), options)
		},
	}, &certv1.CertificateSigningRequest{}, 0, cache.Indexers{})

	handler := func(obj interface{}) {
		csr, ok := obj.(*certv1.CertificateSigningRequest)
		if !ok {
			klog.Info("not a csr")
			os.Exit(1)
		}

		// Inject the fake certificate only once the CSR has been approved and no
		// certificate has been issued yet. Injecting it earlier (e.g. as soon as the
		// CSR is created) would bump its resourceVersion and make the approval update
		// performed by the caller fail with a conflict.
		if !isApproved(csr) || csr.Status.Certificate != nil {
			return
		}

		csr.Status.Certificate = []byte(FakeCRT)
		if _, err := client.CertificatesV1().CertificateSigningRequests().UpdateStatus(
			context.TODO(), csr, metav1.UpdateOptions{}); err != nil {
			klog.Error(err)
		}
	}

	// The Add handler is required to cover the case in which the CSR is already
	// approved (and without certificate) when the informer performs its initial
	// list: in that case no Update event is ever delivered for the approval.
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handler,
		UpdateFunc: func(_, newObj interface{}) { handler(newObj) },
	}); err != nil {
		klog.Error(err)
	}

	go informer.Run(stopChan)
}

// isApproved returns whether the provided CSR contains an approved condition.
func isApproved(csr *certv1.CertificateSigningRequest) bool {
	for _, condition := range csr.Status.Conditions {
		if condition.Type == certv1.CertificateApproved && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// Copyright 2019-2025 The Liqo Authors
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
	"fmt"
	"sync"
	"time"

	certv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	k8s "k8s.io/client-go/kubernetes"
	certv1listers "k8s.io/client-go/listers/certificates/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// WatcherHandlerFunc represents a the function type executed once an approved CSR is observesd by the informer.
type WatcherHandlerFunc func(*certv1.CertificateSigningRequest)

// Watcher wraps the logic to be notified once a CSR change is detected.
type Watcher struct {
	// The un-exported type is embedded as a pointer to prevent subtle bugs occurring if the watcher struct gets copied.
	*watcher
}

// watcher wraps the logic to be notified once a CSR change is detected.
type watcher struct {
	certv1listers.CertificateSigningRequestLister

	starter        func(context.Context)
	lock           sync.RWMutex
	genericHandler WatcherHandlerFunc
	handlerForName map[string]WatcherHandlerFunc

	EventHandler func(obj interface{})
}

// NewWatcher initializes a new CSR watcher for the given label selector and field selector.
func NewWatcher(clientset k8s.Interface, resync time.Duration, labelSelector labels.Selector, fieldSelector fields.Selector) Watcher {
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, resync, informers.WithTweakListOptions(
		func(lo *metav1.ListOptions) {
			lo.LabelSelector = labelSelector.String()
			lo.FieldSelector = fieldSelector.String()
		},
	))

	lister := factory.Certificates().V1().CertificateSigningRequests().Lister()
	starter := func(ctx context.Context) {
		factory.Start(ctx.Done())
		factory.WaitForCacheSync(ctx.Done())
	}
	watcher := &watcher{
		CertificateSigningRequestLister: lister,

		starter:        starter,
		genericHandler: nil,
		handlerForName: make(map[string]WatcherHandlerFunc),
	}

	eventHandler := func(obj interface{}) {
		csr, ok := obj.(*certv1.CertificateSigningRequest)
		if !ok {
			klog.Errorf("Failed to convert object into CSR")
			return
		}

		watcher.lock.RLock()
		defer watcher.lock.RUnlock()

		// Trigger first the specific handler for the given resource name (if set).
		if handler, ok := watcher.handlerForName[csr.GetName()]; ok {
			handler(csr)
		}

		// Then, trigger the generic handler (if set).
		if watcher.genericHandler != nil {
			watcher.genericHandler(csr)
		}
	}

	watcher.EventHandler = eventHandler

	// Setup the informer to get noticed once a CSR change is detected.
	informer := factory.Certificates().V1().CertificateSigningRequests().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    eventHandler,
		UpdateFunc: func(_, newObj interface{}) { eventHandler(newObj) },
	})

	return Watcher{watcher}
}

// Start starts the CSR watcher.
func (r Watcher) Start(ctx context.Context) {
	r.starter(ctx)
}

// RegisterHandler registers a new handler executed once a CSR change is detected.
func (r Watcher) RegisterHandler(handler WatcherHandlerFunc) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.genericHandler = handler
}

// RegisterHandlerForName registers a new handler executed once a new CSR change with the given name is detected.
func (r Watcher) RegisterHandlerForName(name string, handler WatcherHandlerFunc) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.handlerForName[name] = handler
}

// UnregisterHandler un-registers the handler executed once a CSR change is detected.
func (r Watcher) UnregisterHandler() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.genericHandler = nil
}

// UnregisterHandlerForName un-registers the handler executed once a CSR change with the given name is detected.
func (r Watcher) UnregisterHandlerForName(name string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.handlerForName, name)
}

// RetrieveCertificate registers the appropriate handlers and waits for the certificate retrieval.
func (r Watcher) RetrieveCertificate(ctx context.Context, csrName string) ([]byte, error) {
	certificateChan := make(chan []byte, 1)

	handler := func(csr *certv1.CertificateSigningRequest) {
		if IsApproved(csr) {
			select {
			// A copy of the certificate is created to prevent mutating the cache
			case certificateChan <- append([]byte(nil), csr.Status.Certificate...):
			default: // Avoid to block if a certificate is already present in the channel
			}
		}
	}

	r.RegisterHandlerForName(csrName, handler)
	defer r.UnregisterHandlerForName(csrName)

	// Check if the certificate is already approved, in case this occurred before we registered the handler.
	if obj, err := r.Get(csrName); err == nil {
		handler(obj)
	}

	// Wait for the certificate, with a timeout
	select {
	case certificate := <-certificateChan:
		return certificate, nil
	case <-ctx.Done():
		err := fmt.Errorf("context canceled before certificate retrieval")
		return nil, err
	}
}

// IsApproved returns whether the given CSR is approved (i.e. has a valid certificate).
func IsApproved(csr *certv1.CertificateSigningRequest) bool {
	return csr != nil && len(csr.Status.Certificate) > 0
}

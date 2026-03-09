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

package restcfg

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
)

// certificateHolder holds a TLS certificate that can be updated atomically.
// It supports transparent certificate rotation: when the kubeconfig secret is
// updated with a renewed certificate, the holder is updated and idle connections
// are closed to force new TLS handshakes.
type certificateHolder struct {
	mu         sync.RWMutex
	cert       *tls.Certificate
	config     *rest.Config
	transports []*http.Transport
}

// getClientCertificate returns the current certificate for TLS client authentication.
func (h *certificateHolder) getClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.cert == nil {
		return nil, fmt.Errorf("no client certificate available")
	}
	return h.cert, nil
}

// update replaces the current certificate and closes idle connections to force
// new TLS handshakes with the updated certificate. It is a no-op if the
// certificate has not changed.
func (h *certificateHolder) update(certData, keyData []byte) error {
	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return fmt.Errorf("failed to parse updated certificate: %w", err)
	}

	h.mu.Lock()
	if h.cert != nil && len(h.cert.Certificate) > 0 && len(cert.Certificate) > 0 &&
		bytes.Equal(h.cert.Certificate[0], cert.Certificate[0]) {
		h.mu.Unlock()
		return nil
	}
	h.cert = &cert
	transports := h.transports
	h.mu.Unlock()

	// Update the rest.Config's cert data so that SPDY/exec connections
	// (which read TLSClientConfig directly) also pick up the renewed certificate.
	h.config.CertData = certData
	h.config.KeyData = keyData

	// Close idle connections on all transports to force new TLS handshakes.
	for _, t := range transports {
		t.CloseIdleConnections()
	}

	klog.Info("Updated remote cluster client certificate")
	return nil
}

// UpdateCfgCertOnSecretChange configures the given rest.Config to use a dynamic
// certificate callback for TLS client authentication and watches the specified
// kubeconfig secret for changes. When the secret is updated with a renewed
// certificate, the TLS certificate is transparently rotated for all clients
// sharing this config.
//
// TLS fields (CAData, CertData, KeyData, etc.) are preserved on the config so
// that SPDY-based connections (kubectl exec/attach/port-forward) which read
// TLSClientConfig directly continue to work. CertData and KeyData are also
// updated on renewal for the same reason.
// To allow both SPDY operations and renewal of the certificates on existing clients,
// WrapTransport is used to inject the dynamic certificate callback into each
// HTTP transport created by client-go. This allows new TLS handshakes to use the
// updated certificate.
//
// If the config does not use certificate-based authentication (e.g., AWS IAM),
// this function is a no-op. If the secret watcher goroutine exits unexpectedly,
// the process is terminated.
func UpdateCfgCertOnSecretChange(ctx context.Context, config *rest.Config,
	client kubernetes.Interface, namespace, secretName string) {
	certData := config.CertData
	keyData := config.KeyData
	if len(certData) == 0 || len(keyData) == 0 {
		return
	}

	initialCert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		klog.Fatalf("Failed to parse initial certificate for dynamic rotation: %v", err)
	}

	holder := &certificateHolder{cert: &initialCert, config: config}

	// Use WrapTransport to intercept each HTTP transport created by client-go
	// and replace its static certificate with our dynamic callback.
	// TLS fields are intentionally kept on the config so that SPDY connections
	// (exec/attach/port-forward) which build their own TLS config from
	// rest.Config.TLSClientConfig still have CA and client cert data.
	existingWrap := config.WrapTransport
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if t, ok := rt.(*http.Transport); ok {
			// GetClientCertificate takes precedence over Certificates in crypto/tls.
			t.TLSClientConfig.GetClientCertificate = holder.getClientCertificate
			t.TLSClientConfig.Certificates = nil

			holder.mu.Lock()
			holder.transports = append(holder.transports, t)
			holder.mu.Unlock()
		}
		if existingWrap != nil {
			rt = existingWrap(rt)
		}
		return rt
	}

	// Start the secret watcher goroutine. If it exits unexpectedly
	// (i.e., context is still active), terminate the process.
	go func() {
		runSecretWatcher(ctx, client, namespace, secretName, holder)
		if ctx.Err() == nil {
			klog.Fatal("Kubeconfig secret watcher exited unexpectedly")
		}
	}()

	klog.Info("Dynamic certificate rotation enabled for remote cluster connection")
}

// runSecretWatcher runs an informer that watches the specified kubeconfig secret
// and updates the certificate holder when the secret changes. It blocks until
// the context is canceled.
func runSecretWatcher(ctx context.Context, client kubernetes.Interface,
	namespace, secretName string, holder *certificateHolder) {
	factory := informers.NewSharedInformerFactoryWithOptions(client, 0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", secretName).String()
		}),
	)

	informer := factory.Core().V1().Secrets().Informer()
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_, newObj interface{}) {
			secret, ok := newObj.(*corev1.Secret)
			if !ok {
				return
			}

			kubeconfigData, ok := secret.Data[consts.KubeconfigSecretField]
			if !ok {
				klog.Warning("Updated kubeconfig secret does not contain kubeconfig data")
				return
			}

			cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
			if err != nil {
				klog.Errorf("Failed to parse updated kubeconfig: %v", err)
				return
			}

			if err := holder.update(cfg.CertData, cfg.KeyData); err != nil {
				klog.Errorf("Failed to update certificate from kubeconfig secret: %v", err)
			}
		},
	}); err != nil {
		klog.Fatalf("Failed to add event handler for kubeconfig secret watcher: %v", err)
	}

	// Blocks until ctx.Done() is closed.
	informer.Run(ctx.Done())
}

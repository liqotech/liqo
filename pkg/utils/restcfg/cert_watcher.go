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
	"crypto/x509"
	"fmt"
	"net/http"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilnet "k8s.io/apimachinery/pkg/util/net"
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
	mu        sync.RWMutex
	cert      *tls.Certificate
	transport *http.Transport
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
	transport := h.transport
	h.mu.Unlock()

	if transport != nil {
		transport.CloseIdleConnections()
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
// If the config does not use certificate-based authentication (e.g., AWS IAM),
// this function is a no-op. If the secret watcher goroutine exits unexpectedly,
// the process is terminated.
func UpdateCfgCertOnSecretChange(ctx context.Context, config *rest.Config,
	client kubernetes.Interface, namespace, secretName string) {
	certData := config.TLSClientConfig.CertData
	keyData := config.TLSClientConfig.KeyData
	if len(certData) == 0 || len(keyData) == 0 {
		return
	}

	initialCert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		klog.Fatalf("Failed to parse initial certificate for dynamic rotation: %v", err)
	}

	holder := &certificateHolder{cert: &initialCert}

	// Build TLS config with the dynamic certificate callback.
	tlsConfig := &tls.Config{
		GetClientCertificate: holder.getClientCertificate,
		MinVersion:           tls.VersionTLS12,
	}

	if caData := config.TLSClientConfig.CAData; len(caData) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caData) {
			klog.Fatal("Failed to parse CA certificate data for dynamic rotation")
		}
		tlsConfig.RootCAs = pool
	}

	if config.TLSClientConfig.Insecure {
		tlsConfig.InsecureSkipVerify = true
	}
	if config.TLSClientConfig.ServerName != "" {
		tlsConfig.ServerName = config.TLSClientConfig.ServerName
	}

	// Build an HTTP transport with the dynamic certificate callback and apply
	// the standard client-go defaults (proxy, dialer, timeouts, HTTP/2).
	transport := utilnet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           config.Proxy,
	})

	holder.transport = transport

	// Replace the config's transport and clear TLS fields to avoid conflicts.
	// When rest.Config.Transport is set, client-go uses it directly and validates
	// that no TLS fields are present (see k8s.io/client-go/transport.New).
	config.Transport = transport
	config.TLSClientConfig = rest.TLSClientConfig{}
	config.Proxy = nil

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
// the context is cancelled.
func runSecretWatcher(ctx context.Context, client kubernetes.Interface,
	namespace, secretName string, holder *certificateHolder) {
	factory := informers.NewSharedInformerFactoryWithOptions(client, 0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", secretName).String()
		}),
	)

	informer := factory.Core().V1().Secrets().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
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

			if err := holder.update(cfg.TLSClientConfig.CertData, cfg.TLSClientConfig.KeyData); err != nil {
				klog.Errorf("Failed to update certificate from kubeconfig secret: %v", err)
			}
		},
	})

	// Blocks until ctx.Done() is closed.
	informer.Run(ctx.Done())
}

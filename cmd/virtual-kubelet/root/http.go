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

package root

import (
	"context"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	certificates "k8s.io/api/certificates/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/certificate"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/workload"
)

type crtretriever func(*tls.ClientHelloInfo) (*tls.Certificate, error)

func setupHTTPServer(ctx context.Context, handler workload.PodHandler, localClient kubernetes.Interface,
	remoteConfig *rest.Config, cfg *Opts) (err error) {
	var retriever crtretriever

	parsedIP := net.ParseIP(cfg.NodeIP)
	if parsedIP == nil {
		return fmt.Errorf("failed to parse node IP %q", cfg.NodeIP)
	}

	switch cfg.CertificateType.Value {
	case CertificateTypeSelfSigned:
		retriever = newSelfSignedCertificateRetriever(cfg.NodeName, parsedIP)
	default:
		// Determine the appropriate signer based on the requested certificate type.
		signer := map[string]string{
			CertificateTypeKubelet: certificates.KubeletServingSignerName,
			CertificateTypeAWS:     "beta.eks.amazonaws.com/app-serving",
		}

		retriever, err = newCertificateRetriever(localClient, signer[cfg.CertificateType.Value], cfg.NodeName, parsedIP)
		if err != nil {
			return fmt.Errorf("failed to initialize certificate manager: %w", err)
		}
	}

	mux := http.NewServeMux()

	cl := kubernetes.NewForConfigOrDie(remoteConfig)
	attachMetricsRoutes(ctx, mux, cl.RESTClient(), cfg.HomeCluster.GetClusterID())

	podRoutes := api.PodHandlerConfig{
		RunInContainer:        handler.Exec,
		AttachToContainer:     handler.Attach,
		PortForward:           handler.PortForward,
		GetContainerLogs:      handler.Logs,
		GetStatsSummary:       handler.Stats,
		GetPodsFromKubernetes: handler.List,
		GetPods:               handler.List,
	}

	api.AttachPodRoutes(podRoutes, mux, true)

	server := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", cfg.ListenPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // Required to limit the effects of the Slowloris attack.
		TLSConfig: &tls.Config{
			GetCertificate: retriever,
			MinVersion:     tls.VersionTLS12,
		},
	}

	go func() {
		klog.Infof("Starting the virtual kubelet HTTPs server listening on %q", server.Addr)

		// Key and certificate paths are not specified, since already configured as part of the TLSConfig.
		if err := server.ListenAndServeTLS("", ""); err != nil {
			klog.Errorf("Failed to start the HTTPs server: %v", err)
			os.Exit(1)
		}
	}()

	return nil
}

func attachMetricsRoutes(ctx context.Context, mux *http.ServeMux, cl rest.Interface, localClusterID liqov1beta1.ClusterID) {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		klog.Infof("Received request for %s", r.RequestURI)

		res := cl.Get().RequestURI(path.Clean(fmt.Sprintf("/apis/metrics.liqo.io/v1beta1/scrape/%s/%s",
			localClusterID, r.RequestURI))).Do(ctx)
		err := res.Error()
		if err != nil {
			klog.Error(err)
			http.Error(w, "Server Error", http.StatusInternalServerError)
			return
		}

		var statusCode int
		res.StatusCode(&statusCode)

		data, err := res.Raw()
		if err != nil {
			klog.Error(err)
			http.Error(w, "Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(statusCode)
		if _, err = w.Write(data); err != nil {
			klog.Error(err)
		}
	}

	mux.HandleFunc("/metrics", handlerFunc)
	mux.HandleFunc("/metrics/cadvisor", handlerFunc)
	mux.HandleFunc("/metrics/resource", handlerFunc)
	mux.HandleFunc("/metrics/probes", handlerFunc)
}

// newCertificateManager creates a certificate manager for the kubelet when retrieving a server certificate, or returns an error.
// This function is inspired by the original kubelet implementation:
// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/certificate/kubelet.go
func newCertificateRetriever(kubeClient kubernetes.Interface, signer, nodeName string, nodeIP net.IP) (crtretriever, error) {
	const (
		vkCertsPath   = "/tmp/certs"
		vkCertsPrefix = "virtual-kubelet"
	)

	certificateStore, err := certificate.NewFileStore(vkCertsPrefix, vkCertsPath, vkCertsPath, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize server certificate store: %w", err)
	}

	getTemplate := func() *x509.CertificateRequest {
		return &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName:   fmt.Sprintf("system:node:%s", nodeName),
				Organization: []string{"system:nodes"},
			},
			IPAddresses: []net.IP{nodeIP},
		}
	}

	mgr, err := certificate.NewManager(&certificate.Config{
		ClientsetFn: func(_ *tls.Certificate) (kubernetes.Interface, error) {
			return kubeClient, nil
		},
		GetTemplate: getTemplate,
		SignerName:  signer,
		Usages: []certificates.KeyUsage{
			// https://tools.ietf.org/html/rfc5280#section-4.2.1.3
			//
			// Digital signature allows the certificate to be used to verify
			// digital signatures used during TLS negotiation.
			certificates.UsageDigitalSignature,
			// KeyEncipherment allows the cert/key pair to be used to encrypt
			// keys, including the symmetric keys negotiated during TLS setup
			// and used for data transfer.
			certificates.UsageKeyEncipherment,
			// ServerAuth allows the cert to be used by a TLS server to
			// authenticate itself to a TLS client.
			certificates.UsageServerAuth,
		},
		CertificateStore: certificateStore,
		Logf:             klog.V(2).Infof,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize server certificate manager: %w", err)
	}

	mgr.Start()

	return func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		cert := mgr.Current()
		if cert == nil {
			return nil, fmt.Errorf("no serving certificate available")
		}
		return cert, nil
	}, nil
}

// newSelfSignedCertificateRetriever creates a new retriever for self-signed certificates.
func newSelfSignedCertificateRetriever(nodeName string, nodeIP net.IP) crtretriever {
	creator := func() (*tls.Certificate, time.Time, error) {
		expiration := time.Now().AddDate(1, 0, 0) // 1 year

		// Generate a new private key.
		publicKey, privateKey, err := ed25519.GenerateKey(cryptorand.Reader)
		if err != nil {
			return nil, expiration, fmt.Errorf("failed to generate a key pair: %w", err)
		}

		keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			return nil, expiration, fmt.Errorf("failed to marshal the private key: %w", err)
		}

		// Generate the corresponding certificate.
		cert := &x509.Certificate{
			Subject: pkix.Name{
				CommonName:   fmt.Sprintf("system:node:%s", nodeName),
				Organization: []string{"liqo.io"},
			},
			IPAddresses:  []net.IP{nodeIP},
			SerialNumber: big.NewInt(rand.Int63()), //nolint:gosec // A weak random generator is sufficient.
			NotBefore:    time.Now(),
			NotAfter:     expiration,
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		}

		certBytes, err := x509.CreateCertificate(cryptorand.Reader, cert, cert, publicKey, privateKey)
		if err != nil {
			return nil, expiration, fmt.Errorf("failed to create the self-signed certificate: %w", err)
		}

		// Encode the resulting certificate and private key as a single object.
		output, err := tls.X509KeyPair(
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes}),
			pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}))
		if err != nil {
			return nil, expiration, fmt.Errorf("failed to create the X509 key pair: %w", err)
		}

		return &output, expiration, nil
	}

	// Cache the last generated cert, until it is not expired.
	var cert *tls.Certificate
	var expiration time.Time
	return func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		if cert == nil || expiration.Before(time.Now().AddDate(0, 0, 1)) {
			var err error
			cert, expiration, err = creator()
			if err != nil {
				return nil, err
			}
		}
		return cert, nil
	}
}

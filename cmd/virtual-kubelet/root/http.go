// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package root

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog"

	"github.com/liqotech/liqo/cmd/virtual-kubelet/provider"
	"github.com/liqotech/liqo/pkg/virtualKubelet/node/module/api"
)

// AcceptedCiphers is the list of accepted TLS ciphers, with known weak ciphers elided
// Note this list should be a moving target.
var AcceptedCiphers = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,

	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
}

func loadTLSConfig(certPath, keyPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, errors.Wrap(err, "error loading tls certs")
	}

	return &tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CipherSuites:             AcceptedCiphers,
	}, nil
}

func setupHTTPServer(ctx context.Context, p provider.Provider, cfg *apiServerConfig, getPodsFromKubernetes api.PodListerFunc) (_ func(), retErr error) {
	var closers []io.Closer
	cancel := func() {
		for _, c := range closers {
			c.Close()
		}
	}
	defer func() {
		if retErr != nil {
			cancel()
		}
	}()

	if cfg.CertPath == "" || cfg.KeyPath == "" {
		klog.Error("TLS certificates not provided, not setting up pod http server")
	} else {
		s, err := startPodHandlerServer(ctx, p, cfg, getPodsFromKubernetes)
		if err != nil {
			return nil, err
		}
		if s != nil {
			closers = append(closers, s)
		}
	}

	if cfg.MetricsAddr == "" {
		klog.Info("Pod metrics server not setup due to empty metrics address")
	} else {
		l, err := net.Listen("tcp", cfg.MetricsAddr)
		if err != nil {
			return nil, errors.Wrap(err, "could not setup listener for pod metrics http server")
		}

		mux := http.NewServeMux()

		var summaryHandlerFunc api.PodStatsSummaryHandlerFunc
		if mp, ok := p.(provider.PodMetricsProvider); ok {
			summaryHandlerFunc = mp.GetStatsSummary
		}
		podMetricsRoutes := api.PodMetricsConfig{
			GetStatsSummary: summaryHandlerFunc,
		}
		api.AttachPodMetricsRoutes(podMetricsRoutes, mux)
		s := &http.Server{
			Handler: mux,
		}
		go serveHTTP(ctx, s, l, "pod metrics")
		closers = append(closers, s)
	}

	return cancel, nil
}

func startPodHandlerServer(ctx context.Context, p provider.Provider,
	cfg *apiServerConfig, getPodsFromKubernetes api.PodListerFunc) (*http.Server, error) {
	tlsCfg, err := loadTLSConfig(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		klog.Error(err)
		// we are ingnoring this error at the moment to allow the kubelet execution with Kubernetes providers
		// that are not issuing node certificates (i.e. EKS)
		return nil, nil
	}
	l, err := tls.Listen("tcp", cfg.Addr, tlsCfg)
	if err != nil {
		return nil, errors.Wrap(err, "error setting up listener for pod http server")
	}

	mux := http.NewServeMux()

	podRoutes := api.PodHandlerConfig{
		RunInContainer:        p.RunInContainer,
		GetContainerLogs:      p.GetContainerLogs,
		GetPodsFromKubernetes: getPodsFromKubernetes,
		GetStatsSummary:       p.GetStatsSummary,
		GetPods:               p.GetPods,
	}

	api.AttachPodRoutes(podRoutes, mux, true)

	s := &http.Server{
		Handler:   mux,
		TLSConfig: tlsCfg,
	}
	go serveHTTP(ctx, s, l, "pods")
	return s, nil
}

func serveHTTP(ctx context.Context, s *http.Server, l net.Listener, name string) {
	if err := s.Serve(l); err != nil {
		select {
		case <-ctx.Done():
		default:
			klog.Error(errors.Wrapf(err, "Error setting up %s http server", name))
		}
	}
	l.Close()
}

type apiServerConfig struct {
	CertPath              string
	KeyPath               string
	Addr                  string
	MetricsAddr           string
	StreamIdleTimeout     time.Duration
	StreamCreationTimeout time.Duration
}

func getAPIConfig(c Opts) (*apiServerConfig, error) {
	config := apiServerConfig{
		CertPath: os.Getenv("APISERVER_CERT_LOCATION"),
		KeyPath:  os.Getenv("APISERVER_KEY_LOCATION"),
	}

	config.Addr = fmt.Sprintf(":%d", c.ListenPort)
	config.MetricsAddr = c.MetricsAddr

	return &config, nil
}

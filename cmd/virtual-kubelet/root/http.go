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
	"path"
	"time"

	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/workload"
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

func setupHTTPServer(ctx context.Context,
	handler workload.PodHandler, cfg *apiServerConfig,
	localClusterID string, remoteConfig *rest.Config) (_ func(), retErr error) {
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
		s, err := startPodHandlerServer(ctx, handler, cfg, localClusterID, remoteConfig)
		if err != nil {
			return nil, err
		}
		if s != nil {
			closers = append(closers, s)
		}
	}

	return cancel, nil
}

func startPodHandlerServer(ctx context.Context, handler workload.PodHandler, cfg *apiServerConfig,
	localClusterID string, remoteConfig *rest.Config) (*http.Server, error) {
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

	cl := kubernetes.NewForConfigOrDie(remoteConfig)
	attachMetricsRoutes(ctx, mux, cl.RESTClient(), localClusterID)

	podRoutes := api.PodHandlerConfig{
		RunInContainer:        handler.Exec,
		GetContainerLogs:      handler.Logs,
		GetStatsSummary:       handler.Stats,
		GetPodsFromKubernetes: handler.List,
		GetPods:               handler.List,
	}

	api.AttachPodRoutes(podRoutes, mux, true)

	s := &http.Server{
		Handler:   mux,
		TLSConfig: tlsCfg,
	}
	go serveHTTP(ctx, s, l, "pods")
	return s, nil
}

func attachMetricsRoutes(ctx context.Context, mux *http.ServeMux, cl rest.Interface, localClusterID string) {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		klog.Infof("Received request for %s", r.RequestURI)

		res := cl.Get().RequestURI(path.Clean(fmt.Sprintf("/apis/metrics.liqo.io/v1alpha1/scrape/%s/%s",
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

func getAPIConfig(c *Opts) *apiServerConfig {
	config := apiServerConfig{
		CertPath: os.Getenv("APISERVER_CERT_LOCATION"),
		KeyPath:  os.Getenv("APISERVER_KEY_LOCATION"),
	}

	config.Addr = fmt.Sprintf(":%d", c.ListenPort)
	config.MetricsAddr = c.MetricsAddress

	return &config
}

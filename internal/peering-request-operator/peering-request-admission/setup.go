package peering_request_admission

import (
	"crypto/tls"
	"fmt"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	"k8s.io/klog"
	"net/http"
	"os"
	"path/filepath"
)

func StartWebhook(certPath string, keyPath string, namespace string) *WebhookServer {
	port := 8443
	return startTls(certPath, keyPath, port, namespace)
}

func startTls(certPath string, keyPath string, port int, namespace string) *WebhookServer {
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	config, err := v1alpha1.NewKubeconfig(filepath.Join(os.Getenv("HOME"), ".kube", "config"), &discoveryv1.GroupVersion)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	crdClient, err := v1alpha1.NewFromConfig(config)
	if err != nil {
		klog.Error(err, "unable to create crd client")
		os.Exit(1)
	}

	whsvr := &WebhookServer{
		Server: &http.Server{
			Addr:      fmt.Sprintf(":%v", port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},

		crdClient: crdClient,
		Namespace: namespace,
	}

	// define http Server and Server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", whsvr.serve)
	whsvr.Server.Handler = mux

	// start webhook Server in new routine
	go func() {
		if err := whsvr.Server.ListenAndServeTLS(certPath, keyPath); err != nil {
			klog.Error(err, "Failed to listen and serve webhook Server: "+err.Error())
		}
	}()

	klog.Info("Server started")

	return whsvr
}

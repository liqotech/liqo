package peering_request_admission

import (
	"crypto/tls"
	"fmt"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	"net/http"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

func StartWebhook(certPath string, namespace string) *WebhookServer {
	port := 8443
	return startTls(certPath, port, namespace)
}

func startTls(certPath string, port int, namespace string) *WebhookServer {
	certFile := certPath + "/cert.pem"
	keyFile := certPath + "/key.pem"

	log := ctrl.Log.WithName("peering-webhook-admission")

	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Error(err, err.Error())
		os.Exit(1)
	}

	client, err := clients.NewK8sClient()
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	whsvr := &WebhookServer{
		Server: &http.Server{
			Addr:      fmt.Sprintf(":%v", port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},

		Log:       log,
		client:    client,
		Namespace: namespace,
	}

	// define http Server and Server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", whsvr.serve)
	whsvr.Server.Handler = mux

	// start webhook Server in new routine
	go func() {
		if err := whsvr.Server.ListenAndServeTLS(certFile, keyFile); err != nil {
			whsvr.Log.Error(err, "Failed to listen and serve webhook Server: "+err.Error())
		}
	}()

	whsvr.Log.Info("Server started")

	return whsvr
}

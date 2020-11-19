package main

import (
	"flag"
	auth_service "github.com/liqotech/liqo/internal/auth-service"
	"k8s.io/klog"
	"os"
	"path/filepath"
	"time"
)

func main() {
	klog.Info("Starting")

	var namespace string
	var kubeconfigPath string
	var resyncSeconds int64
	var listeningPort string
	var certFile string
	var keyFile string

	flag.StringVar(&namespace, "namespace", "default", "Namespace where your configs are stored.")
	flag.StringVar(&kubeconfigPath, "kubeconfigPath", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "For debug purpose, set path to local kubeconfig")
	flag.Int64Var(&resyncSeconds, "resyncSeconds", 30, "Resync seconds for the informers")
	flag.StringVar(&listeningPort, "listeningPort", "5000", "Sets the port where the service will listen")
	flag.StringVar(&certFile, "certFile", "/certs/cert.pem", "Path to cert file")
	flag.StringVar(&keyFile, "keyFile", "/certs/key.pem", "Path to key file")
	flag.Parse()

	klog.Info("Namespace: ", namespace)

	authService, err := auth_service.NewAuthServiceCtrl(namespace, kubeconfigPath, time.Duration(resyncSeconds)*time.Second)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	if err = authService.Start(listeningPort, certFile, keyFile); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

package main

import (
	"flag"
	"github.com/joho/godotenv"
	peering_request_operator "github.com/liqoTech/liqo/internal/peering-request-operator"
	peering_request_admission "github.com/liqoTech/liqo/internal/peering-request-operator/peering-request-admission"
	"k8s.io/klog"
	"os"
	"path/filepath"
)

func main() {
	klog.Info("Starting")

	var broadcasterImage, broadcasterServiceAccount string
	var inputEnvFile string
	var kubeconfigPath string

	flag.StringVar(&inputEnvFile, "input-env-file", "/etc/environment/liqo/env", "The environment variable file to source at startup")
	flag.StringVar(&broadcasterImage, "broadcaster-image", "liqo/advertisement-broadcaster", "Broadcaster-operator image name")
	flag.StringVar(&broadcasterServiceAccount, "broadcaster-sa", "broadcaster", "Broadcaster-operator ServiceAccount name")
	flag.StringVar(&kubeconfigPath, "kubeconfigPath", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "For debug purpose, set path to local kubeconfig")
	flag.Parse()

	if err := godotenv.Load(inputEnvFile); err != nil {
		klog.Error(err, "The env variable file hasn't been correctly loaded")
		os.Exit(1)
	}

	namespace, ok := os.LookupEnv("liqonamespace")
	if !ok {
		namespace = "default"
	}
	certPath, ok := os.LookupEnv("liqocert")
	if !ok {
		certPath = "/etc/ssl/liqo/server-cert.pem"
	}
	keyPath, ok := os.LookupEnv("liqokey")
	if !ok {
		certPath = "/etc/ssl/liqo/server-key.pem"
	}

	klog.Info("Starting admission webhook")
	_ = peering_request_admission.StartWebhook(certPath, keyPath, namespace, kubeconfigPath)

	klog.Info("Starting peering-request operator")
	peering_request_operator.StartOperator(namespace, broadcasterImage, broadcasterServiceAccount, kubeconfigPath)
}

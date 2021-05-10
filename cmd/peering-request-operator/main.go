package main

import (
	"flag"
	"os"
	"path/filepath"

	"k8s.io/klog"

	peering_request_operator "github.com/liqotech/liqo/internal/peering-request-operator"
)

func main() {
	klog.Info("Starting")

	var broadcasterImage, broadcasterServiceAccount, vkServiceAccount string
	var kubeconfigPath string

	flag.StringVar(&broadcasterImage, "broadcaster-image", "liqo/advertisement-broadcaster", "Broadcaster-operator image name")
	flag.StringVar(&broadcasterServiceAccount, "broadcaster-sa", "broadcaster", "Broadcaster-operator ServiceAccount name")
	flag.StringVar(&vkServiceAccount, "vk-sa", "vk-remote", "Remote VirtualKubelet ServiceAccount name")
	flag.StringVar(&kubeconfigPath, "kubeconfigPath", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "For debug purpose, set path to local kubeconfig")
	flag.Parse()

	namespace, ok := os.LookupEnv("POD_NAMESPACE")
	if !ok {
		namespace = "default"
	}

	klog.Info("Starting peering-request operator")
	peering_request_operator.StartOperator(namespace, broadcasterImage, broadcasterServiceAccount, vkServiceAccount, kubeconfigPath)
}

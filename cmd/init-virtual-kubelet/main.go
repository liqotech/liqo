package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils"
	vk "github.com/liqotech/liqo/pkg/vkMachinery"
	"github.com/liqotech/liqo/pkg/vkMachinery/csr"
)

const timeout = 30 * time.Second

func main() {
	var config *rest.Config
	var distribution string
	klog.Info("Loading client config")
	flag.StringVar(&distribution, "k8s-distribution", "kubernetes", "determine the provider to adapt csr generation")
	ctx := context.Background()

	kubeconfigPath, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		kubeconfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	klog.Infof("Loading client: %s", kubeconfigPath)
	config, err := utils.UserConfig(kubeconfigPath)
	if err != nil {
		klog.Fatalf("Unable to create client config: %s", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Unable to create client: %s", err)
	}

	name, ok := os.LookupEnv("POD_NAME")
	if !ok {
		klog.Fatal("Unable to create CSR: POD_NAME undefined")
	}

	// Generate Key and CSR files in PEM format
	if err := csr.CreateCSRResource(ctx, name, client, vk.CsrLocation, vk.KeyLocation, distribution); err != nil {
		klog.Fatalf("Unable to create CSR: %s", err)
	}

	cancelCtx, cancel := context.WithTimeout(ctx, timeout)
	var crtChan = make(chan []byte)
	var cert []byte
	informer := csr.ForgeInformer(client, name, crtChan)
	go informer.Run(cancelCtx.Done())
	select {
	case <-cancelCtx.Done():
		klog.Error("Unable to get certificate: timeout elapsed")
	case cert = <-crtChan:
		if err := utils.WriteFile(vk.CertLocation, cert); err != nil {
			klog.Fatalf("Unable to write the CRT file in location: %s", vk.CertLocation)
		}
	}
	cancel()
}

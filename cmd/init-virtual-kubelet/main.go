package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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

	namespace, ok := os.LookupEnv("POD_NAMESPACE")
	if !ok {
		klog.Fatal("Unable to create CSR: POD_NAMESPACE undefined")
	}

	nodeName, ok := os.LookupEnv("NODE_NAME")
	if !ok {
		klog.Fatal("Unable to create CSR: NODE_NAME undefined")
	}

	defer func() {
		if err = csr.PersistCertificates(ctx, client, nodeName, namespace,
			vk.CsrLocation, vk.KeyLocation, vk.CertLocation); err != nil {
			klog.Error(err)
			os.Exit(1)
		}
	}()

	_, hasCertificate, err := csr.GetCSRSecret(ctx, client, nodeName, namespace)
	if !apierrors.IsNotFound(err) && !hasCertificate {
		if err != nil {
			klog.Error(err)
		} else {
			klog.Info("Certificate already present for this nodeName. Skipping")
		}
		return
	}

	// Generate Key and CSR files in PEM format
	if err := csr.CreateCSRResource(ctx, name, client, nodeName, namespace, distribution); err != nil {
		klog.Fatalf("Unable to create CSR: %s", err)
	}

	cancelCtx, cancel := context.WithTimeout(ctx, timeout)
	csrWatcher := csr.NewWatcher(client, 0, labels.SelectorFromSet(vk.CsrLabels))
	csrWatcher.Start(ctx)
	cert, err := csrWatcher.RetrieveCertificate(cancelCtx, name)
	cancel()

	if err != nil {
		klog.Error("Unable to get certificate: %w", err)
		return
	}

	if err := csr.StoreCertificate(ctx, client, cert, namespace, nodeName); err != nil {
		klog.Fatal("Unable to store the CRT file in secret")
	}
}

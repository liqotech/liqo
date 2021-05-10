package main

import (
	"context"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils"
	vk "github.com/liqotech/liqo/pkg/vkMachinery"
	"github.com/liqotech/liqo/pkg/vkMachinery/csr"
)

func main() {
	var config *rest.Config
	klog.Info("Loading client config")

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
	if err := createCSRResource(name, client, vk.CsrLocation, vk.KeyLocation); err != nil {
		klog.Fatalf("Unable to create CSR: %s", err)
	}

	cert, err := csr.WaitForApproval(client, name)
	if err != nil {
		klog.Fatalf("Unable to get certificate: %s", err)
	}

	if err := utils.WriteFile(vk.CertLocation, cert); err != nil {
		os.Exit(1)
	}
}

func createCSRResource(name string, client kubernetes.Interface, CsrLocation string, KeyLocation string) error {
	csrPem, keyPem, err := csr.GenerateVKCertificateBundle(name)
	if err != nil {
		return err
	}

	if err := utils.WriteFile(CsrLocation, csrPem); err != nil {
		return err
	}

	if err := utils.WriteFile(KeyLocation, keyPem); err != nil {
		return err
	}

	// Generate and create CSR resource
	csrResource := csr.GenerateVKCSR(name, csrPem)
	_, err = client.CertificatesV1beta1().CertificateSigningRequests().Create(context.TODO(), csrResource, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		klog.Infof("CSR already exists: %s", err)
	} else if err != nil {
		klog.Errorf("Unable to create CSR: %s", err)
		return err
	}
	return nil
}

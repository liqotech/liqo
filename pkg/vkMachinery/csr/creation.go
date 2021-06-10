package csr

import (
	"context"

	certificatesv1 "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils"
)

// CreateCSRResource creates a CSR Resource for a new Virtual Kubelet instance.
func CreateCSRResource(ctx context.Context, name string, client kubernetes.Interface, csrLocation, keyLocation, distribution string) error {
	csrPem, keyPem, err := generateVKCertificateBundle(name)
	var csrResource *certificatesv1.CertificateSigningRequest
	if err != nil {
		return err
	}

	if err := utils.WriteFile(csrLocation, csrPem); err != nil {
		return err
	}

	if err := utils.WriteFile(keyLocation, keyPem); err != nil {
		return err
	}

	// Generate and create CSR resource
	switch distribution {
	// For standard Kubernetes Clusters, it will create a CSR for KubeletServing signerName, allowing the VK to act as a server.
	case "kubernetes":
		csrResource = GenerateVKCSR(name, csrPem, kubeletServingSignerName)
	// Fallback selection will generate an always accepted CSR with kubeletAPIServingSignerName signer
	default:
		csrResource = GenerateVKCSR(name, csrPem, kubeletAPIServingSignerName)
	}

	_, err = client.CertificatesV1().CertificateSigningRequests().Create(ctx, csrResource, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		klog.Infof("CSR already exists: %s", err)
	} else if err != nil {
		klog.Errorf("Unable to create CSR: %s", err)
		return err
	}
	return nil
}

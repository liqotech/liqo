// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csr

import (
	"context"
	"net"

	certificatesv1 "k8s.io/api/certificates/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CreateCSRResource creates a CSR Resource for a new Virtual Kubelet instance.
func CreateCSRResource(ctx context.Context,
	name string, client kubernetes.Interface, nodeName, namespace, distribution string, podIP net.IP) error {
	csrPem, keyPem, err := generateVKCertificateBundle(name, podIP)
	var csrResource *certificatesv1.CertificateSigningRequest
	if err != nil {
		return err
	}

	if err = createCSRSecret(ctx, client, keyPem, csrPem, nodeName, namespace); errors.IsAlreadyExists(err) {
		// the secret already exists, it has the key and the csr, we have to retrieve the certificate
		_, csrPem, _, err = getCSRData(ctx, client, nodeName, namespace)
	}
	if err != nil {
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

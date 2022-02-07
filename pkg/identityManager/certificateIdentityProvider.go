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

package identitymanager

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	certv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	certificateSigningRequest "github.com/liqotech/liqo/pkg/vkMachinery/csr"
)

// random package initialization.
func init() {
	rand.Seed(time.Now().UnixNano())
}

type certificateIdentityProvider struct {
	namespaceManager tenantnamespace.Manager
	client           kubernetes.Interface
	csrWatcher       certificateSigningRequest.Watcher
}

// GetRemoteCertificate retrieves a certificate issued in the past,
// given the clusterid and the signingRequest.
func (identityProvider *certificateIdentityProvider) GetRemoteCertificate(cluster discoveryv1alpha1.ClusterIdentity,
	namespace, signingRequest string) (response *responsetypes.SigningRequestResponse, err error) {
	secret, err := identityProvider.client.CoreV1().Secrets(namespace).Get(context.TODO(), remoteCertificateSecret, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			klog.V(4).Info(err)
		} else {
			klog.Error(err)
		}
		return response, err
	}

	signingRequestSecret, ok := secret.Data[csrSecretKey]
	if !ok {
		klog.Errorf("no %v key in secret %v/%v", csrSecretKey, secret.Namespace, secret.Name)
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteCertificateSecret)
		return response, err
	}

	// check that this certificate is related to this signing request
	if csr := base64.StdEncoding.EncodeToString(signingRequestSecret); csr != signingRequest {
		err = kerrors.NewBadRequest(fmt.Sprintf("the stored and the provided CSR for cluster %s does not match", cluster.ClusterName))
		klog.Error(err)
		return response, err
	}

	response = &responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseCertificate,
	}
	response.Certificate, ok = secret.Data[certificateSecretKey]
	if !ok {
		klog.Errorf("no %v key in secret %v/%v", certificateSecretKey, secret.Namespace, secret.Name)
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteCertificateSecret)
		return response, err
	}

	return response, nil
}

// ApproveSigningRequest approves a remote CertificateSigningRequest.
// It creates a CertificateSigningRequest CR to be issued by the local cluster, and approves it.
// This function will wait (with a timeout) for an available certificate before returning.
func (identityProvider *certificateIdentityProvider) ApproveSigningRequest(cluster discoveryv1alpha1.ClusterIdentity,
	signingRequest string) (response *responsetypes.SigningRequestResponse, err error) {
	signingBytes, err := base64.StdEncoding.DecodeString(signingRequest)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	cert := &certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: strings.Join([]string{identitySecretRoot, ""}, "-"),
			Labels:       map[string]string{remoteTenantCSRLabel: strconv.FormatBool(true)},
		},
		Spec: certv1.CertificateSigningRequestSpec{
			Groups: []string{
				"system:authenticated",
			},
			SignerName: certv1.KubeAPIServerClientSignerName,
			Request:    signingBytes,
			Usages: []certv1.KeyUsage{
				certv1.UsageDigitalSignature,
				certv1.UsageKeyEncipherment,
				certv1.UsageClientAuth,
			},
		},
	}

	cert, err = identityProvider.client.CertificatesV1().CertificateSigningRequests().Create(context.TODO(), cert, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return response, err
	}

	// approve the CertificateSigningRequest
	if err = certificateSigningRequest.Approve(identityProvider.client, cert, "IdentityManagerApproval",
		"This CSR was approved by Liqo Identity Manager"); err != nil {
		klog.Error(err)
		return response, err
	}

	response = &responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseCertificate,
	}
	// retrieve the certificate issued by the Kubernetes issuer in the CSR (with a 30 seconds timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response.Certificate, err = identityProvider.csrWatcher.RetrieveCertificate(ctx, cert.Name)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	// store the certificate in a Secret, in this way is possbile to retrieve it again in the future
	if _, err = identityProvider.storeRemoteCertificate(cluster, signingBytes, response.Certificate); err != nil {
		klog.Error(err)
		return response, err
	}
	return response, nil
}

// storeRemoteCertificate stores the issued certificate in a Secret in the TenantNamespace.
func (identityProvider *certificateIdentityProvider) storeRemoteCertificate(cluster discoveryv1alpha1.ClusterIdentity,
	signingRequest, certificate []byte) (*v1.Secret, error) {
	namespace, err := identityProvider.namespaceManager.GetNamespace(cluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      remoteCertificateSecret,
			Namespace: namespace.Name,
			Labels: map[string]string{
				discovery.ClusterIDLabel: cluster.ClusterID,
			},
		},
		Data: map[string][]byte{
			csrSecretKey:         signingRequest,
			certificateSecretKey: certificate,
		},
	}

	if secret, err = identityProvider.client.CoreV1().
		Secrets(namespace.Name).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		klog.Error(err)
		return nil, err
	}
	return secret, nil
}

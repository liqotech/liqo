// Copyright 2019-2024 The Liqo Authors
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
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	certv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	certificateSigningRequest "github.com/liqotech/liqo/pkg/utils/csr"
)

// random package initialization.
func init() {
	rand.Seed(time.Now().UnixNano())
}

type certificateIdentityProvider struct {
	namespaceManager tenantnamespace.Manager
	k8sClient        kubernetes.Interface
	csrWatcher       certificateSigningRequest.Watcher
}

// GetRemoteCertificate retrieves a certificate issued in the past,
// given the clusterid and the signingRequest.
func (identityProvider *certificateIdentityProvider) GetRemoteCertificate(ctx context.Context,
	options *SigningRequestOptions) (response *responsetypes.SigningRequestResponse, err error) {
	response = &responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseCertificate,
	}

	secretName := remoteCertificateSecretName(options)
	secret, err := identityProvider.k8sClient.CoreV1().Secrets(options.Namespace).Get(ctx, secretName, metav1.GetOptions{})
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
		}, secretName)
		return response, err
	}

	// check that this certificate is related to this signing request
	if !bytes.Equal(signingRequestSecret, options.SigningRequest) {
		err = kerrors.NewBadRequest(fmt.Sprintf("the stored and the provided CSR for cluster %s does not match", options.Cluster.ClusterName))
		klog.Error(err)
		return response, err
	}

	response.Certificate, ok = secret.Data[certificateSecretKey]
	if !ok {
		klog.Errorf("no %v key in secret %v/%v", certificateSecretKey, secret.Namespace, secret.Name)
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, secretName)
		return response, err
	}

	return response, nil
}

// ApproveSigningRequest approves a remote CertificateSigningRequest.
// It creates a CertificateSigningRequest CR to be issued by the local cluster, and approves it.
// This function will wait (with a timeout) for an available certificate before returning.
func (identityProvider *certificateIdentityProvider) ApproveSigningRequest(ctx context.Context,
	options *SigningRequestOptions) (response *responsetypes.SigningRequestResponse, err error) {
	cert := &certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: identitySecretRoot + "-",
			Labels:       map[string]string{remoteTenantCSRLabel: strconv.FormatBool(true)},
		},
		Spec: certv1.CertificateSigningRequestSpec{
			Groups: []string{
				"system:authenticated",
			},
			SignerName: certv1.KubeAPIServerClientSignerName,
			Request:    options.SigningRequest,
			Usages: []certv1.KeyUsage{
				certv1.UsageDigitalSignature,
				certv1.UsageKeyEncipherment,
				certv1.UsageClientAuth,
			},
		},
	}

	cert, err = identityProvider.k8sClient.CertificatesV1().CertificateSigningRequests().Create(ctx, cert, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return response, err
	}

	// approve the CertificateSigningRequest
	if err = certificateSigningRequest.Approve(identityProvider.k8sClient, cert, "IdentityManagerApproval",
		"This CSR was approved by Liqo Identity Manager"); err != nil {
		klog.Error(err)
		return response, err
	}

	response = &responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseCertificate,
	}
	// retrieve the certificate issued by the Kubernetes issuer in the CSR (with a 30 seconds timeout)
	ctxC, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response.Certificate, err = identityProvider.csrWatcher.RetrieveCertificate(ctxC, cert.Name)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	// store the certificate in a Secret, in this way is possbile to retrieve it again in the future
	if _, err = identityProvider.storeRemoteCertificate(ctx, options, response.Certificate); err != nil {
		klog.Error(err)
		return response, err
	}
	return response, nil
}

func (identityProvider *certificateIdentityProvider) ForgeAuthParams(resp *responsetypes.SigningRequestResponse,
	apiServer string, ca []byte) *authv1alpha1.AuthParams {
	return &authv1alpha1.AuthParams{
		CA:        ca,
		SignedCRT: resp.Certificate,
		APIServer: apiServer,
	}
}

func remoteCertificateSecretName(options *SigningRequestOptions) string {
	switch options.IdentityType {
	case authv1alpha1.ResourceSliceIdentityType:
		return fmt.Sprintf("%s-%s", remoteCertificateSecret, options.Name)
	default:
		return remoteCertificateSecret
	}
}

// storeRemoteCertificate stores the issued certificate in a Secret in the TenantNamespace.
func (identityProvider *certificateIdentityProvider) storeRemoteCertificate(ctx context.Context,
	options *SigningRequestOptions, certificate []byte) (*v1.Secret, error) {
	namespace, err := identityProvider.namespaceManager.GetNamespace(ctx, *options.Cluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      remoteCertificateSecretName(options),
			Namespace: namespace.Name,
			Labels: map[string]string{
				discovery.ClusterIDLabel: options.Cluster.ClusterID,
			},
		},
		Data: map[string][]byte{
			csrSecretKey:         options.SigningRequest,
			certificateSecretKey: certificate,
		},
	}

	if secret, err = identityProvider.k8sClient.CoreV1().
		Secrets(namespace.Name).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		klog.Error(err)
		return nil, err
	}
	return secret, nil
}

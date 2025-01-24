// Copyright 2019-2025 The Liqo Authors
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
//

package userfactory

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	certv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	liqoctlutils "github.com/liqotech/liqo/pkg/liqoctl/utils"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	certificateSigningRequest "github.com/liqotech/liqo/pkg/utils/csr"
	"github.com/liqotech/liqo/pkg/utils/getters"
	kubeconfigutils "github.com/liqotech/liqo/pkg/utils/kubeconfig"
)

// GeneratePeerUser generates a new user to peer with the local cluster and returns its kubeconfig.
func GeneratePeerUser(ctx context.Context, clusterID liqov1beta1.ClusterID, tenantNsName string, opts *factory.Factory) (string, error) {
	if exists, err := IsExistingPeerUser(ctx, opts.CRClient, clusterID); err != nil {
		return "", fmt.Errorf("unable to check if the user already exists: %w", err)
	} else if exists {
		return "", fmt.Errorf("a user to peer from cluster with ID %q already exists. Please delete it first before creting a new one."+
			"You can delete the previous secret via 'liqoctl delete peering-user --consumer-cluster-id %s'", clusterID, clusterID)
	}

	// Get the certification authority
	ca, err := apiserver.RetrieveAPIServerCA(opts.RESTConfig, nil, false)
	if err != nil {
		return "", fmt.Errorf("unable to get the API server CA: %w", err)
	}

	// Forge a new pair of keys.
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("error while generating token credentials: %w", err)
	}

	// Generate a CSR with the newly created keys.
	csr, userCN, err := authentication.GenerateCSRForPeerUser(private, clusterID)
	if err != nil {
		return "", fmt.Errorf("error while generating the csr for the token credentials: %w", err)
	}

	// Sign the csr to generate the certificate
	cert, err := generateSignedCert(ctx, opts.CRClient, opts.KubeClient, csr, clusterID)
	if err != nil {
		return "", fmt.Errorf("unable to generate certificate for the user: %w", err)
	}

	if err := EnsureRoles(ctx, opts.CRClient, clusterID, userCN, tenantNsName); err != nil {
		return "", fmt.Errorf("unable to ensure roles: %w", err)
	}

	// Convert the private key in PEM format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return "", fmt.Errorf("unable to parse private key: %w", err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})

	// Get K8S API Server address
	apiAddr, err := getAPIServerAddress(ctx, opts.CRClient, opts.LiqoNamespace)
	if err != nil {
		return "", fmt.Errorf("unable to get the API Server addr: %w", err)
	}

	userName := fmt.Sprintf("%s-user", clusterID)
	kubeconfig, err := kubeconfigutils.GenerateKubeconfig(userName, string(clusterID), apiAddr, ca, cert, privatePEM, nil, &tenantNsName)
	if err != nil {
		return "", fmt.Errorf("unable to generate kubeconfig: %w", err)
	}

	return string(kubeconfig), nil
}

// GetUserNameFromClusterID returns the username of the peering user for the given clusterID.
func GetUserNameFromClusterID(clusterID liqov1beta1.ClusterID) string {
	return fmt.Sprintf("liqo-peer-user-%s", clusterID)
}

func getAPIServerAddress(ctx context.Context, c client.Client, liqoNamespaceName string) (string, error) {
	// Get the controller manager deployment
	ctrlDeployment, err := getters.GetControllerManagerDeployment(ctx, c, liqoNamespaceName)
	if err != nil {
		return "", err
	}

	// Get the controller manager container
	ctrlContainer, err := liqoctlutils.GetCtrlManagerContainer(ctrlDeployment)
	if err != nil {
		return "", err
	}

	// Get the URL of the K8s API
	apiServerAddressOverride, _ := liqoctlutils.ExtractValuesFromArgumentList("--api-server-address-override", ctrlContainer.Args)
	apiAddr, err := apiserver.GetURL(ctx, c, apiServerAddressOverride)
	if err != nil {
		return "", err
	}

	return apiAddr, nil
}

// generateSignedCert generates a new signed certificate to create a peering with the local cluster.
func generateSignedCert(
	ctx context.Context,
	c client.Client,
	clientset kubernetes.Interface,
	csr []byte,
	clusterID liqov1beta1.ClusterID,
) ([]byte, error) {
	userName := GetUserNameFromClusterID(clusterID)
	cert := &certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: userName,
			Labels: map[string]string{
				consts.PeeringUserNameLabelKey: userName,
			},
		},
		Spec: certv1.CertificateSigningRequestSpec{
			Groups: []string{
				"system:authenticated",
			},
			SignerName: certv1.KubeAPIServerClientSignerName,
			Request:    csr,
			Usages: []certv1.KeyUsage{
				certv1.UsageDigitalSignature,
				certv1.UsageKeyEncipherment,
				certv1.UsageClientAuth,
			},
		},
	}

	cert, err := clientset.CertificatesV1().CertificateSigningRequests().Create(ctx, cert, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// approve the CertificateSigningRequest
	if err = certificateSigningRequest.Approve(clientset, cert, "IdentityManagerApproval",
		"This CSR was approved by liqoctl generate token"); err != nil {
		return nil, err
	}

	// retrieve the certificate issued by the Kubernetes issuer in the CSR (with a 30 seconds timeout)
	ctxC, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return getSignedCSRBlocker(ctxC, c, cert.Name)
}

// getSignedCSRBlocker waits until the csr with the given name has been signed and it returned the certificate.
func getSignedCSRBlocker(ctx context.Context, c client.Client, csrName string) ([]byte, error) {
	var certificate []byte
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		var csr certv1.CertificateSigningRequest
		if err := c.Get(ctx, client.ObjectKey{Name: csrName}, &csr); err != nil {
			return false, err
		}
		if len(csr.Status.Certificate) > 0 {
			certificate = csr.Status.Certificate
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed waiting for CSR to be signed: %w", err)
	}

	return certificate, nil
}

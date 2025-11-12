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

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	versionpkg "github.com/liqotech/liqo/pkg/liqo-controller-manager/version"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// GenerateTenant generates a Tenant resource to be applied on a remote cluster.
// Using the cluster keys it generates a CSR to obtain a ControlPlane Identity from
// the provider cluster.
// It needs the local cluster identity to get the authentication keys and the signature
// of the nonce given by the provider cluster to complete the authentication challenge.
func GenerateTenant(ctx context.Context, cl client.Client,
	localClusterID liqov1beta1.ClusterID, liqoNamespace, remoteTenantNamespace string,
	signature []byte, proxyURL *string) (*authv1beta1.Tenant, error) {
	// Get public and private keys of the local cluster.
	privateKey, publicKey, err := authentication.GetClusterKeys(ctx, cl, liqoNamespace)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster keys: %w", err)
	}

	// Generate a CSR for the remote cluster.
	CSR, err := authentication.GenerateCSRForControlPlane(privateKey, localClusterID)
	if err != nil {
		return nil, fmt.Errorf("unable to generate CSR: %w", err)
	}

	// Get the local cluster's API server URL from the kubeconfig in the auth secret.
	apiServerURL, err := getLocalAPIServerURL(ctx, cl, liqoNamespace)
	if err != nil {
		// Log error but continue - API server URL is optional
		fmt.Printf("Warning: unable to get local API server URL: %v\n", err)
		apiServerURL = ""
	}

	// Get the version reader token from the secret.
	// Use the in-cluster clientset for this
	var versionReaderToken string
	clientset, err := kubernetes.NewForConfig(restcfg.SetRateLimiter(ctrl.GetConfigOrDie()))
	if err == nil {
		versionReaderToken, err = versionpkg.GetVersionReaderToken(ctx, clientset, liqoNamespace)
		if err != nil {
			// Log error but continue - version reader token is optional
			fmt.Printf("Warning: unable to get version reader token: %v\n", err)
			versionReaderToken = ""
		}
	}

	// Forge tenant resource for the remote cluster.
	return forge.TenantForRemoteCluster(localClusterID, publicKey, CSR, signature, &remoteTenantNamespace, proxyURL, apiServerURL, versionReaderToken), nil
}

// getLocalAPIServerURL retrieves the local cluster's API server URL from the auth secret.
func getLocalAPIServerURL(ctx context.Context, cl client.Client, liqoNamespace string) (string, error) {
	// Get the auth secret containing the kubeconfig
	var secret corev1.Secret
	if err := cl.Get(ctx, types.NamespacedName{
		Namespace: liqoNamespace,
		Name:      consts.AuthKeysSecretName,
	}, &secret); err != nil {
		return "", fmt.Errorf("unable to get auth secret: %w", err)
	}

	// Extract the API server URL from the kubeconfig
	kubeconfigData, ok := secret.Data[consts.KubeconfigSecretField]
	if !ok {
		return "", fmt.Errorf("kubeconfig not found in auth secret")
	}

	// Parse the kubeconfig to get the API server URL
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return "", fmt.Errorf("unable to parse kubeconfig: %w", err)
	}

	if config.Host == "" {
		return "", fmt.Errorf("API server URL not found in kubeconfig")
	}

	return config.Host, nil
}

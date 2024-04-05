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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// GetConfig gets a rest config from the secret, given the remote clusterID and (optionally) the namespace.
// This rest config con be used to create a client to the remote cluster.
func (certManager *identityManager) GetConfig(remoteCluster discoveryv1alpha1.ClusterIdentity, _ string) (*rest.Config, error) {
	ctx := context.TODO()

	// Get Secret with ControlPlane Identity associated to the given remote cluster.
	secret, err := getters.GetControlPlaneKubeconfigSecretByClusterID(ctx, certManager.client, remoteCluster.ClusterID)
	if err != nil {
		return nil, err
	}

	// TODO: refactor to work with the new identity.
	if certManager.isAwsIdentity(secret) {
		return certManager.getIAMConfig(secret, remoteCluster)
	}

	return buildConfigFromSecret(secret)
}

func (certManager *identityManager) GetSecretNamespacedName(remoteCluster discoveryv1alpha1.ClusterIdentity,
	_ string) (types.NamespacedName, error) {
	ctx := context.TODO()

	// Get Secret with ControlPlane Identity associated to the given remote cluster.
	secret, err := getters.GetControlPlaneKubeconfigSecretByClusterID(ctx, certManager.client, remoteCluster.ClusterID)
	if err != nil {
		return types.NamespacedName{}, err
	}

	return client.ObjectKeyFromObject(secret), nil
}

// GetConfigFromSecret gets a rest config from a secret.
func (certManager *identityManager) GetConfigFromSecret(secret *corev1.Secret) (*rest.Config, error) {
	if certManager.isAwsIdentity(secret) {
		return certManager.getIAMConfig(secret, discoveryv1alpha1.ClusterIdentity{})
	}

	return buildConfigFromSecret(secret)
}

// GetRemoteTenantNamespace returns the tenant namespace that
// the remote cluster assigned to this peering.
func (certManager *identityManager) GetRemoteTenantNamespace(remoteCluster discoveryv1alpha1.ClusterIdentity, _ string) (string, error) {
	ctx := context.TODO()

	// Get Secret with ControlPlane Identity associated to the given remote cluster.
	secret, err := getters.GetControlPlaneKubeconfigSecretByClusterID(ctx, certManager.client, remoteCluster.ClusterID)
	if err != nil {
		return "", err
	}

	remoteTenantNamespace, ok := secret.Annotations[consts.RemoteTenantNamespaceAnnotKey]
	if !ok {
		return "", fmt.Errorf("remote tenant namespace annotation (%s) not found in secret %q",
			consts.RemoteTenantNamespaceAnnotKey, client.ObjectKeyFromObject(secret))
	}

	return remoteTenantNamespace, nil
}

func (certManager *identityManager) getIAMConfig(
	secret *corev1.Secret, remoteCluster discoveryv1alpha1.ClusterIdentity) (*rest.Config, error) {
	return certManager.iamTokenManager.getConfig(secret, remoteCluster)
}

func buildConfigFromSecret(secret *corev1.Secret) (*rest.Config, error) {
	kubeconfig, ok := secret.Data[consts.KubeconfigSecretField]
	if !ok {
		return nil, fmt.Errorf("key %v not found in secret %v/%v", consts.KubeconfigSecretField, secret.Namespace, secret.Name)
	}

	return clientcmd.RESTConfigFromKubeConfig(kubeconfig)
}

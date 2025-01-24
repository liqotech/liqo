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

package forge

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	kubeconfigutils "github.com/liqotech/liqo/pkg/utils/kubeconfig"
)

// generateKubeconfigSecretName generates the name of the kubeconfig secret associated to an identity.
func generateKubeconfigSecretName(identity *authv1beta1.Identity) string {
	return "kubeconfig-" + identity.Name
}

// KubeconfigSecret forges a new Secret object stroing the kubeconfig associated to the provided identity.
func KubeconfigSecret(identity *authv1beta1.Identity) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateKubeconfigSecretName(identity),
			Namespace: identity.Namespace,
		},
	}
}

// MutateKubeconfigSecret mutate a Secret object storing the kubeconfig associated to the provided identity.
func MutateKubeconfigSecret(secret *corev1.Secret, identity *authv1beta1.Identity, clientKey []byte, namespace *string) error {
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	secret.Labels[consts.RemoteClusterID] = string(identity.Spec.ClusterID)
	secret.Labels[consts.IdentityTypeLabelKey] = string(identity.Spec.Type)

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	// if the namespace is not empty, it stores the remote tenant namespace where the kubeconfig is used.
	if namespace != nil && *namespace != "" {
		secret.Annotations[consts.RemoteTenantNamespaceAnnotKey] = *namespace
	}

	kubeconfig, err := kubeconfigutils.GenerateKubeconfig(identity.Name, string(identity.Spec.ClusterID),
		identity.Spec.AuthParams.APIServer, identity.Spec.AuthParams.CA, identity.Spec.AuthParams.SignedCRT, clientKey,
		identity.Spec.AuthParams.ProxyURL, namespace)
	if err != nil {
		return err
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[consts.KubeconfigSecretField] = kubeconfig

	if identity.Spec.AuthParams.AwsConfig != nil {
		if secret.StringData == nil {
			secret.StringData = make(map[string]string)
		}
		secret.StringData[identitymanager.AwsAccessKeyIDSecretKey] = identity.Spec.AuthParams.AwsConfig.AwsAccessKeyID
		secret.StringData[identitymanager.AwsSecretAccessKeySecretKey] = identity.Spec.AuthParams.AwsConfig.AwsSecretAccessKey
		secret.StringData[identitymanager.AwsRegionSecretKey] = identity.Spec.AuthParams.AwsConfig.AwsRegion
		secret.StringData[identitymanager.AwsEKSClusterIDSecretKey] = identity.Spec.AuthParams.AwsConfig.AwsClusterName
		secret.StringData[identitymanager.AwsIAMUserArnSecretKey] = identity.Spec.AuthParams.AwsConfig.AwsUserArn
	}

	return nil
}

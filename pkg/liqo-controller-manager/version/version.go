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

package version

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;create;update
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;create;update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;create;update

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils/resource"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

const (
	// LiqoVersionConfigMapName is the name of the ConfigMap containing the Liqo version.
	LiqoVersionConfigMapName = "liqo-version"
	// LiqoVersionReaderRoleName is the name of the Role that allows reading the liqo-version ConfigMap.
	LiqoVersionReaderRoleName = "liqo-version-reader"
	// LiqoVersionReaderRoleBindingName is the name of the RoleBinding for the liqo-version-reader Role.
	LiqoVersionReaderRoleBindingName = "liqo-version-reader-binding"
	// LiqoVersionReaderServiceAccountName is the name of the ServiceAccount that can read the liqo-version ConfigMap.
	LiqoVersionReaderServiceAccountName = "liqo-version-reader"
	// LiqoVersionReaderSecretName is the name of the Secret containing the token for the version reader ServiceAccount.
	LiqoVersionReaderSecretName = "liqo-version-reader-token"
	// LiqoVersionKey is the key in the ConfigMap data where the version is stored.
	LiqoVersionKey = "version"
	// VersionReaderGroupName is the RBAC group name that can read the version ConfigMap.
	// Using system:authenticated allows any authenticated user to read the version.
	VersionReaderGroupName = "system:authenticated"
)

// GetVersionFromDeployment reads the liqo-controller-manager deployment and extracts
// the version from its container image tag.
func GetVersionFromDeployment(ctx context.Context, clientset kubernetes.Interface, namespace, deploymentName string) string {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		klog.Warningf("Failed to get deployment %s/%s: %v", namespace, deploymentName, err)
		return ""
	}

	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		klog.Warning("No containers found in deployment")
		return ""
	}

	image := deployment.Spec.Template.Spec.Containers[0].Image

	// Extract the tag from the image (format: registry/org/image:tag)
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		klog.Warningf("Image %q does not contain a tag, cannot determine Liqo version", image)
		return ""
	}

	tag := parts[len(parts)-1]
	klog.Infof("Detected Liqo version from deployment: %s", tag)
	return tag
}

// SetupVersionResources creates or updates the ConfigMap, Role, RoleBinding, ServiceAccount, and Secret
// for exposing the Liqo version to remote clusters.
func SetupVersionResources(ctx context.Context, clientset kubernetes.Interface, liqoNamespace, version string) error {
	if version == "" {
		klog.Warning("Liqo version is empty, skipping version resources setup")
		return nil
	}

	// Create or update the ConfigMap
	if err := createOrUpdateVersionConfigMap(ctx, clientset, liqoNamespace, version); err != nil {
		return fmt.Errorf("failed to create/update version ConfigMap: %w", err)
	}

	// Create or update the ServiceAccount
	if err := createOrUpdateVersionReaderServiceAccount(ctx, clientset, liqoNamespace); err != nil {
		return fmt.Errorf("failed to create/update version reader ServiceAccount: %w", err)
	}

	// Create or update the Role
	if err := createOrUpdateVersionReaderRole(ctx, clientset, liqoNamespace); err != nil {
		return fmt.Errorf("failed to create/update version reader Role: %w", err)
	}

	// Create or update the RoleBinding
	if err := createOrUpdateVersionReaderRoleBinding(ctx, clientset, liqoNamespace); err != nil {
		return fmt.Errorf("failed to create/update version reader RoleBinding: %w", err)
	}

	// Create or update the Secret with long-lived token
	if err := createOrUpdateVersionReaderSecret(ctx, clientset, liqoNamespace); err != nil {
		return fmt.Errorf("failed to create/update version reader Secret: %w", err)
	}

	klog.Infof("Successfully set up version resources (version: %s) in namespace %s", version, liqoNamespace)
	return nil
}

// createOrUpdateVersionConfigMap creates or updates the ConfigMap containing the Liqo version.
func createOrUpdateVersionConfigMap(ctx context.Context, clientset kubernetes.Interface, liqoNamespace, version string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LiqoVersionConfigMapName,
			Namespace: liqoNamespace,
		},
		Data: map[string]string{
			LiqoVersionKey: version,
		},
	}

	resource.AddGlobalLabels(configMap)

	_, err := clientset.CoreV1().ConfigMaps(liqoNamespace).Get(ctx, LiqoVersionConfigMapName, metav1.GetOptions{})
	if err != nil {
		// ConfigMap doesn't exist, create it
		_, err = clientset.CoreV1().ConfigMaps(liqoNamespace).Create(ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ConfigMap %s/%s: %w", liqoNamespace, LiqoVersionConfigMapName, err)
		}
		klog.Infof("Created ConfigMap %s/%s with version %s", liqoNamespace, LiqoVersionConfigMapName, version)
	} else {
		// ConfigMap exists, update it
		_, err = clientset.CoreV1().ConfigMaps(liqoNamespace).Update(ctx, configMap, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ConfigMap %s/%s: %w", liqoNamespace, LiqoVersionConfigMapName, err)
		}
		klog.Infof("Updated ConfigMap %s/%s with version %s", liqoNamespace, LiqoVersionConfigMapName, version)
	}

	return nil
}

// createOrUpdateVersionReaderRole creates or updates the Role that allows reading the liqo-version ConfigMap.
func createOrUpdateVersionReaderRole(ctx context.Context, clientset kubernetes.Interface, liqoNamespace string) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LiqoVersionReaderRoleName,
			Namespace: liqoNamespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{LiqoVersionConfigMapName},
				Verbs:         []string{"get"},
			},
		},
	}

	resource.AddGlobalLabels(role)

	_, err := clientset.RbacV1().Roles(liqoNamespace).Get(ctx, LiqoVersionReaderRoleName, metav1.GetOptions{})
	if err != nil {
		// Role doesn't exist, create it
		_, err = clientset.RbacV1().Roles(liqoNamespace).Create(ctx, role, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Role %s/%s: %w", liqoNamespace, LiqoVersionReaderRoleName, err)
		}
		klog.Infof("Created Role %s/%s", liqoNamespace, LiqoVersionReaderRoleName)
	} else {
		// Role exists, update it
		_, err = clientset.RbacV1().Roles(liqoNamespace).Update(ctx, role, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update Role %s/%s: %w", liqoNamespace, LiqoVersionReaderRoleName, err)
		}
		klog.V(6).Infof("Updated Role %s/%s", liqoNamespace, LiqoVersionReaderRoleName)
	}

	return nil
}

// createOrUpdateVersionReaderRoleBinding creates or updates the RoleBinding for the liqo-version-reader Role.
func createOrUpdateVersionReaderRoleBinding(ctx context.Context, clientset kubernetes.Interface, liqoNamespace string) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LiqoVersionReaderRoleBindingName,
			Namespace: liqoNamespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     VersionReaderGroupName,
				APIGroup: "rbac.authorization.k8s.io",
			},
			{
				Kind:      "ServiceAccount",
				Name:      LiqoVersionReaderServiceAccountName,
				Namespace: liqoNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     LiqoVersionReaderRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	resource.AddGlobalLabels(roleBinding)

	_, err := clientset.RbacV1().RoleBindings(liqoNamespace).Get(ctx, LiqoVersionReaderRoleBindingName, metav1.GetOptions{})
	if err != nil {
		// RoleBinding doesn't exist, create it
		_, err = clientset.RbacV1().RoleBindings(liqoNamespace).Create(ctx, roleBinding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create RoleBinding %s/%s: %w", liqoNamespace, LiqoVersionReaderRoleBindingName, err)
		}
		klog.Infof("Created RoleBinding %s/%s for ServiceAccount %s", liqoNamespace, LiqoVersionReaderRoleBindingName, LiqoVersionReaderServiceAccountName)
	} else {
		// RoleBinding exists, update it
		_, err = clientset.RbacV1().RoleBindings(liqoNamespace).Update(ctx, roleBinding, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update RoleBinding %s/%s: %w", liqoNamespace, LiqoVersionReaderRoleBindingName, err)
		}
		klog.V(6).Infof("Updated RoleBinding %s/%s", liqoNamespace, LiqoVersionReaderRoleBindingName)
	}

	return nil
}

// createOrUpdateVersionReaderServiceAccount creates or updates the ServiceAccount for reading the liqo-version ConfigMap.
func createOrUpdateVersionReaderServiceAccount(ctx context.Context, clientset kubernetes.Interface, liqoNamespace string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LiqoVersionReaderServiceAccountName,
			Namespace: liqoNamespace,
		},
	}

	resource.AddGlobalLabels(sa)

	_, err := clientset.CoreV1().ServiceAccounts(liqoNamespace).Get(ctx, LiqoVersionReaderServiceAccountName, metav1.GetOptions{})
	if err != nil {
		// ServiceAccount doesn't exist, create it
		_, err = clientset.CoreV1().ServiceAccounts(liqoNamespace).Create(ctx, sa, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ServiceAccount %s/%s: %w", liqoNamespace, LiqoVersionReaderServiceAccountName, err)
		}
		klog.Infof("Created ServiceAccount %s/%s", liqoNamespace, LiqoVersionReaderServiceAccountName)
	} else {
		// ServiceAccount exists, update it
		_, err = clientset.CoreV1().ServiceAccounts(liqoNamespace).Update(ctx, sa, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ServiceAccount %s/%s: %w", liqoNamespace, LiqoVersionReaderServiceAccountName, err)
		}
		klog.V(6).Infof("Updated ServiceAccount %s/%s", liqoNamespace, LiqoVersionReaderServiceAccountName)
	}

	return nil
}

// createOrUpdateVersionReaderSecret creates or updates the Secret containing a long-lived token for the version reader ServiceAccount.
func createOrUpdateVersionReaderSecret(ctx context.Context, clientset kubernetes.Interface, liqoNamespace string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LiqoVersionReaderSecretName,
			Namespace: liqoNamespace,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: LiqoVersionReaderServiceAccountName,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	resource.AddGlobalLabels(secret)

	_, err := clientset.CoreV1().Secrets(liqoNamespace).Get(ctx, LiqoVersionReaderSecretName, metav1.GetOptions{})
	if err != nil {
		// Secret doesn't exist, create it
		_, err = clientset.CoreV1().Secrets(liqoNamespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Secret %s/%s: %w", liqoNamespace, LiqoVersionReaderSecretName, err)
		}
		klog.Infof("Created Secret %s/%s for ServiceAccount token", liqoNamespace, LiqoVersionReaderSecretName)
	} else {
		// Secret already exists
		klog.V(6).Infof("Secret %s/%s already exists", liqoNamespace, LiqoVersionReaderSecretName)
	}

	return nil
}

// GetRemoteVersion retrieves the Liqo version from a remote cluster using the provided clientset.
// It returns an empty string if the ConfigMap doesn't exist or if there's an error.
func GetRemoteVersion(ctx context.Context, remoteClientset kubernetes.Interface, liqoNamespace string) string {
	configMap, err := remoteClientset.CoreV1().ConfigMaps(liqoNamespace).Get(ctx, LiqoVersionConfigMapName, metav1.GetOptions{})
	if err != nil {
		klog.V(4).Infof("Failed to get remote version ConfigMap: %v", err)
		return ""
	}

	version, found := configMap.Data[LiqoVersionKey]
	if !found {
		klog.V(4).Infof("Version key not found in remote ConfigMap")
		return ""
	}

	return version
}

// GetRemoteVersionWithToken retrieves the Liqo version from a remote cluster using API server URL and token.
// This is used for non-peered clusters or when only minimal authentication is available.
func GetRemoteVersionWithToken(ctx context.Context, apiServerURL, token, liqoNamespace string) string {
	if apiServerURL == "" || token == "" {
		klog.V(4).Info("API server URL or token is empty, cannot fetch remote version")
		return ""
	}

	// Create a REST config using the token
	config := restcfg.SetRateLimiter(&rest.Config{
		Host:        apiServerURL,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true, // In production, you may want to handle CA certificates properly
		},
	})

	// Create a clientset
	remoteClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.V(4).Infof("Failed to create clientset for remote cluster: %v", err)
		return ""
	}

	// Use the existing GetRemoteVersion function
	return GetRemoteVersion(ctx, remoteClientset, liqoNamespace)
}

// GetVersionReaderToken retrieves the version reader token from the secret.
// It waits briefly for the token to be populated if the secret exists but is empty.
// Returns the token string or empty string if not available.
func GetVersionReaderToken(ctx context.Context, clientset kubernetes.Interface, liqoNamespace string) (string, error) {
	secret, err := clientset.CoreV1().Secrets(liqoNamespace).Get(ctx, LiqoVersionReaderSecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get version reader secret: %w", err)
	}

	token, ok := secret.Data[corev1.ServiceAccountTokenKey]
	if !ok || len(token) == 0 {
		return "", fmt.Errorf("token not found or empty in version reader secret")
	}

	return string(token), nil
}

// GetLocalVersion retrieves the Liqo version from the local cluster's ConfigMap.
// This can be used to check the local version without needing to query the deployment.
func GetLocalVersion(ctx context.Context, clientset kubernetes.Interface, liqoNamespace string) (string, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(liqoNamespace).Get(ctx, LiqoVersionConfigMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get local version ConfigMap: %w", err)
	}

	version, found := configMap.Data[LiqoVersionKey]
	if !found {
		return "", fmt.Errorf("version key not found in local ConfigMap")
	}

	return version, nil
}

// QueryRemoteVersion is a standalone function to query a remote cluster's Liqo version
// without establishing a full peering relationship. It requires:
// - apiServerURL: The API server URL of the remote cluster
// - token: A bearer token with read access to the liqo-version ConfigMap (e.g., from liqo-version-reader ServiceAccount)
// - liqoNamespace: The namespace where Liqo is installed on the remote cluster (typically "liqo")
//
// Returns the remote cluster's Liqo version string or an error if the query fails.
// This is useful for checking version compatibility before initiating peering.
func QueryRemoteVersion(ctx context.Context, apiServerURL, token, liqoNamespace string) (string, error) {
	if apiServerURL == "" {
		return "", fmt.Errorf("API server URL is required")
	}
	if token == "" {
		return "", fmt.Errorf("authentication token is required")
	}
	if liqoNamespace == "" {
		return "", fmt.Errorf("Liqo namespace is required")
	}

	version := GetRemoteVersionWithToken(ctx, apiServerURL, token, liqoNamespace)
	if version == "" {
		return "", fmt.Errorf("failed to retrieve remote version (check API server URL, token validity, and network connectivity)")
	}

	return version, nil
}

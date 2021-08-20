package auth

import (
	"context"
	"crypto/rand"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// TokenSecretName is the name of the secret containing the authentication token for the local cluster.
	TokenSecretName = "auth-token"
)

// GetToken retrieves the token for the local cluster.
func GetToken(ctx context.Context, c client.Client, namespace string) (string, error) {
	var secret v1.Secret
	if err := c.Get(ctx, types.NamespacedName{
		Name:      TokenSecretName,
		Namespace: namespace,
	}, &secret); err != nil {
		return "", err
	}

	return GetTokenFromSecret(&secret)
}

// GetTokenFromSecret retrieves the token for the local cluster given its secret.
func GetTokenFromSecret(secret *v1.Secret) (string, error) {
	v, ok := secret.Data["token"]
	if !ok {
		err := fmt.Errorf("invalid secret %v/%v: does not contain a valid token",
			secret.GetNamespace(), secret.GetName())
		klog.Error(err)
		return "", err
	}
	return string(v), nil
}

// GenerateToken generates a random authentication token.
func GenerateToken() (string, error) {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	if err != nil {
		klog.Error(err)
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

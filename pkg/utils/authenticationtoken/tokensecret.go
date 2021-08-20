package authenticationtoken

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/discovery"
)

const (
	authTokenSecretNamePrefix = "remote-token-"

	tokenKey = "token"
)

// StoreInSecret stores an authentication token for a given remote cluster in a secret,
// or updates it if it already exists.
func StoreInSecret(ctx context.Context, clientset kubernetes.Interface,
	clusterID, authToken, liqoNamespace string) error {
	secretName := fmt.Sprintf("%v%v", authTokenSecretNamePrefix, clusterID)

	secret, err := clientset.CoreV1().Secrets(liqoNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// create the secret
		return createAuthTokenSecret(ctx, clientset, secretName, liqoNamespace, clusterID, authToken)
	}
	if err != nil {
		klog.Error(err)
		return err
	}

	// the secret already exists, update it
	return updateAuthTokenSecret(ctx, clientset, secret, clusterID, authToken)
}

func updateAuthTokenSecret(ctx context.Context, clientset kubernetes.Interface,
	secret *v1.Secret, clusterID, authToken string) error {
	labels := secret.GetLabels()
	labels[discovery.ClusterIDLabel] = clusterID
	labels[discovery.AuthTokenLabel] = ""
	secret.SetLabels(labels)

	if secret.StringData == nil {
		secret.StringData = map[string]string{}
	}
	secret.StringData[tokenKey] = authToken

	_, err := clientset.CoreV1().Secrets(secret.GetNamespace()).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

func createAuthTokenSecret(ctx context.Context, clientset kubernetes.Interface,
	secretName, liqoNamespace, clusterID, authToken string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: liqoNamespace,
			Labels: map[string]string{
				discovery.ClusterIDLabel: clusterID,
				discovery.AuthTokenLabel: "",
			},
		},
		StringData: map[string]string{
			"token": authToken,
		},
	}

	_, err := clientset.CoreV1().Secrets(liqoNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

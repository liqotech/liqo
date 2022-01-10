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

package authenticationtoken

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/discovery"
)

const (
	authTokenSecretNamePrefix = "remote-token-"

	tokenKey = "token"
)

// GetAuthToken loads the auth token for a foreignCluster with a given clusterID.
func GetAuthToken(ctx context.Context, clusterID string, k8sClient client.Client) (string, error) {
	req1, err := labels.NewRequirement(discovery.ClusterIDLabel, selection.Equals, []string{clusterID})
	utilruntime.Must(err)
	req2, err := labels.NewRequirement(discovery.AuthTokenLabel, selection.Exists, []string{})
	utilruntime.Must(err)

	var tokenSecrets corev1.SecretList
	if err := k8sClient.List(ctx, &tokenSecrets, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(*req1, *req2),
	}); err != nil {
		return "", err
	}

	for i := range tokenSecrets.Items {
		if token, found := tokenSecrets.Items[i].Data["token"]; found {
			return string(token), nil
		}
	}
	return "", nil
}

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
	secret *corev1.Secret, clusterID, authToken string) error {
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
	secret := &corev1.Secret{
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

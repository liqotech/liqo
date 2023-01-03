// Copyright 2019-2023 The Liqo Authors
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
	"sort"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/discovery"
)

// StoreIdentity stores the identity to authenticate with a remote cluster.
func (certManager *identityManager) StoreIdentity(ctx context.Context, remoteCluster discoveryv1alpha1.ClusterIdentity,
	namespace string, key []byte, remoteProxyURL string, identityResponse *auth.CertificateIdentityResponse) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: identitySecretRoot + "-",
			Namespace:    namespace,
			Labels: map[string]string{
				localIdentitySecretLabel:  "true",
				discovery.ClusterIDLabel:  remoteCluster.ClusterID,
				certificateAvailableLabel: "true",
			},
			Annotations: map[string]string{
				// one year starting from now
				certificateExpireTimeAnnotation: fmt.Sprintf("%v", time.Now().AddDate(1, 0, 0).Unix()),
			},
		},
		StringData: map[string]string{
			APIServerURLSecretKey: identityResponse.APIServerURL,
			namespaceSecretKey:    identityResponse.Namespace,
		},
		Data: map[string][]byte{
			privateKeySecretKey: key,
		},
	}

	if identityResponse.HasAWSValues() || certManager.isAwsIdentity(secret) {
		secret.StringData[awsAccessKeyIDSecretKey] = identityResponse.AWSIdentityInfo.AccessKeyID
		secret.StringData[awsSecretAccessKeySecretKey] = identityResponse.AWSIdentityInfo.SecretAccessKey
		secret.StringData[awsRegionSecretKey] = identityResponse.AWSIdentityInfo.Region
		secret.StringData[awsEKSClusterIDSecretKey] = identityResponse.AWSIdentityInfo.EKSClusterID
		secret.StringData[awsIAMUserArnSecretKey] = identityResponse.AWSIdentityInfo.IAMUserArn
	} else {
		certificate, err := base64.StdEncoding.DecodeString(identityResponse.Certificate)
		if err != nil {
			return fmt.Errorf("failed to decode certificate: %w", err)
		}

		secret.Data[certificateSecretKey] = certificate
	}

	// ApiServerCA may be empty if the remote cluster exposes the ApiServer with a certificate issued by "public" CAs
	if identityResponse.APIServerCA != "" {
		apiServerCa, err := base64.StdEncoding.DecodeString(identityResponse.APIServerCA)
		if err != nil {
			return fmt.Errorf("failed to decode certification authority: %w", err)
		}

		secret.Data[apiServerCaSecretKey] = apiServerCa
	}

	if remoteProxyURL != "" {
		secret.StringData[apiProxyURLSecretKey] = remoteProxyURL
	}

	if _, err := certManager.client.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	return nil
}

// getSecret retrieves the identity secret given the clusterID.
func (certManager *identityManager) getSecret(remoteCluster discoveryv1alpha1.ClusterIdentity) (*v1.Secret, error) {
	namespace, err := certManager.namespaceManager.GetNamespace(context.TODO(), remoteCluster)
	if err != nil {
		return nil, err
	}

	return certManager.getSecretInNamespace(remoteCluster, namespace.Name)
}

// getSecretInNamespace retrieves the identity secret in the given Namespace.
func (certManager *identityManager) getSecretInNamespace(remoteCluster discoveryv1alpha1.ClusterIdentity,
	namespace string) (*v1.Secret, error) {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			localIdentitySecretLabel: "true",
			discovery.ClusterIDLabel: remoteCluster.ClusterID,
		},
	}
	secretList, err := certManager.client.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	})
	if err != nil {
		return nil, err
	}

	secrets := secretList.Items
	if nItems := len(secrets); nItems == 0 {
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, fmt.Sprintf("Identity for cluster %v in namespace %v", remoteCluster.ClusterID, namespace))
		return nil, err
	}

	// sort by reverse certificate expire time
	sort.Slice(secrets, func(i, j int) bool {
		time1 := getExpireTime(&secretList.Items[i])
		time2 := getExpireTime(&secretList.Items[j])
		return time1 > time2
	})

	// if there are multiple secrets, get the one with the certificate that will expire last
	return &secrets[0], nil
}

// getExpireTime reads the expire time from the annotations of the secret.
func getExpireTime(secret *v1.Secret) int64 {
	now := time.Now().Unix()
	if secret.Annotations == nil {
		klog.Warningf("annotation %v not found in secret %v/%v", certificateExpireTimeAnnotation, secret.Namespace, secret.Name)
		return now
	}

	timeStr, ok := secret.Annotations[certificateExpireTimeAnnotation]
	if !ok {
		klog.Warningf("annotation %v not found in secret %v/%v", certificateExpireTimeAnnotation, secret.Namespace, secret.Name)
		return now
	}

	n, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		klog.Warning(err)
		return now
	}
	return n
}

func (certManager *identityManager) isAwsIdentity(secret *v1.Secret) bool {
	data := secret.Data
	keys := []string{awsAccessKeyIDSecretKey, awsSecretAccessKeySecretKey, awsRegionSecretKey, awsEKSClusterIDSecretKey, awsIAMUserArnSecretKey}
	for i := range keys {
		if _, ok := data[keys[i]]; !ok {
			return false
		}
	}
	return true
}

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

package identitymanager

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"sort"
	"strconv"
	"strings"
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

// CreateIdentity creates a new key and a new csr to be used as an identity to authenticate with a remote cluster.
func (certManager *identityManager) CreateIdentity(remoteCluster discoveryv1alpha1.ClusterIdentity) (*v1.Secret, error) {
	namespace, err := certManager.namespaceManager.GetNamespace(remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return certManager.createIdentityInNamespace(remoteCluster.ClusterID, namespace.Name)
}

// GetSigningRequest gets the CertificateSigningRequest for a remote cluster.
func (certManager *identityManager) GetSigningRequest(remoteCluster discoveryv1alpha1.ClusterIdentity) ([]byte, error) {
	secret, err := certManager.getSecret(remoteCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	csrBytes, ok := secret.Data[csrSecretKey]
	if !ok {
		err = fmt.Errorf("csr not found in secret %v/%v for clusterid %v",
			secret.Namespace, secret.Name, remoteCluster.ClusterID)
		klog.Error(err)
		return nil, err
	}

	return csrBytes, nil
}

// StoreCertificate stores the certificate issued by a remote authority for the specified remoteClusterID.
func (certManager *identityManager) StoreCertificate(remoteCluster discoveryv1alpha1.ClusterIdentity,
	remoteProxyURL string, identityResponse *auth.CertificateIdentityResponse) error {
	secret, err := certManager.getSecret(remoteCluster)
	if err != nil {
		klog.Error(err)
		return err
	}

	// It is always nil. So we have to create the map.
	secret.StringData = make(map[string]string)

	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels[certificateAvailableLabel] = "true"

	if identityResponse.HasAWSValues() || certManager.isAwsIdentity(secret) {
		secret.Data[awsAccessKeyIDSecretKey] = []byte(identityResponse.AWSIdentityInfo.AccessKeyID)
		secret.Data[awsSecretAccessKeySecretKey] = []byte(identityResponse.AWSIdentityInfo.SecretAccessKey)
		secret.Data[awsRegionSecretKey] = []byte(identityResponse.AWSIdentityInfo.Region)
		secret.Data[awsEKSClusterIDSecretKey] = []byte(identityResponse.AWSIdentityInfo.EKSClusterID)
		secret.Data[awsIAMUserArnSecretKey] = []byte(identityResponse.AWSIdentityInfo.IAMUserArn)
	} else {
		certificate, err := base64.StdEncoding.DecodeString(identityResponse.Certificate)
		if err != nil {
			klog.Error(err)
			return err
		}

		secret.Data[certificateSecretKey] = certificate
	}

	// ApiServerCA may be empty if the remote cluster exposes the ApiServer with a certificate issued by "public" CAs
	if identityResponse.APIServerCA != "" {
		apiServerCa, err := base64.StdEncoding.DecodeString(identityResponse.APIServerCA)
		if err != nil {
			klog.Error(err)
			return err
		}

		secret.Data[apiServerCaSecretKey] = apiServerCa
	}

	secret.Data[APIServerURLSecretKey] = []byte(identityResponse.APIServerURL)
	if remoteProxyURL != "" {
		secret.StringData[apiProxyURLSecretKey] = remoteProxyURL
	}
	secret.Data[namespaceSecretKey] = []byte(identityResponse.Namespace)

	if _, err = certManager.client.CoreV1().Secrets(secret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

// getSecret retrieves the identity secret given the clusterID.
func (certManager *identityManager) getSecret(remoteCluster discoveryv1alpha1.ClusterIdentity) (*v1.Secret, error) {
	namespace, err := certManager.namespaceManager.GetNamespace(remoteCluster)
	if err != nil {
		klog.Error(err)
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
		klog.Error(err)
		return nil, err
	}

	secrets := secretList.Items
	if nItems := len(secrets); nItems == 0 {
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, fmt.Sprintf("Identity for cluster %v in namespace %v", remoteCluster.ClusterID, namespace))
		klog.Error(err)
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

// createCSR generates a key and a certificate signing request.
func (certManager *identityManager) createCSR() (keyBytes, csrBytes []byte, err error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		klog.Error(err)
		return nil, nil, err
	}

	subj := pkix.Name{
		CommonName:   certManager.localCluster.ClusterID,
		Organization: []string{defaultOrganization},
	}
	rawSubj := subj.ToRDNSequence()

	asn1Subj, err := asn1.Marshal(rawSubj)
	if err != nil {
		klog.Error(err)
		return nil, nil, err
	}

	template := x509.CertificateRequest{
		RawSubject:         asn1Subj,
		SignatureAlgorithm: x509.PureEd25519,
	}

	csrBytes, err = x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		klog.Error(err)
		return nil, nil, err
	}
	csrBytes = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	})

	keyBytes, err = x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		klog.Error("Failed to marshal private key: %w", err)
		return nil, nil, err
	}
	keyBytes = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})
	return keyBytes, csrBytes, nil
}

// createIdentityInNamespace creates a new key and a new csr to be used as an identity to authenticate with a remote cluster in a given namespace.
func (certManager *identityManager) createIdentityInNamespace(remoteClusterID, namespace string) (*v1.Secret, error) {
	key, csrBytes, err := certManager.createCSR()
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: strings.Join([]string{identitySecretRoot, ""}, "-"),
			Namespace:    namespace,
			Labels: map[string]string{
				localIdentitySecretLabel: "true",
				discovery.ClusterIDLabel: remoteClusterID,
			},
			Annotations: map[string]string{
				// one year starting from now
				certificateExpireTimeAnnotation: fmt.Sprintf("%v", time.Now().AddDate(1, 0, 0).Unix()),
			},
		},
		Data: map[string][]byte{
			privateKeySecretKey: key,
			csrSecretKey:        csrBytes,
		},
	}

	return certManager.client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
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

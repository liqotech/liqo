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

package csr

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils"
)

const (
	csrPrivateKey  = "privateKey"
	csrKey         = "csr"
	csrCertificate = "certificate"
)

// GetCSRSecret returns the secret containing the CSR data.
func GetCSRSecret(ctx context.Context,
	clientset kubernetes.Interface, nodeName, namespace string) (secret *v1.Secret, hasCertificate bool, err error) {
	secret, err = clientset.CoreV1().Secrets(namespace).Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	_, hasCertificate = secret.Data[csrCertificate]
	return secret, hasCertificate, nil
}

// getCSRData reads the CSR data from the secret.
func getCSRData(ctx context.Context,
	clientset kubernetes.Interface, nodeName, namespace string) (privateKey, csr, certificate []byte, err error) {
	secret, hasCertificate, err := GetCSRSecret(ctx, clientset, nodeName, namespace)
	if err != nil {
		klog.Error(err)
		return nil, nil, nil, err
	}

	privateKey = secret.Data[csrPrivateKey]
	csr = secret.Data[csrKey]
	if hasCertificate {
		certificate = secret.Data[csrCertificate]
	}
	return privateKey, csr, certificate, nil
}

// PersistCertificates persists the data stored in the secret into the default path.
func PersistCertificates(ctx context.Context,
	clientset kubernetes.Interface, nodeName, namespace,
	csrLocation, keyLocation, certLocation string) error {
	privateKey, csr, certificate, err := getCSRData(ctx, clientset, nodeName, namespace)
	if err != nil {
		klog.Error(err)
		return err
	}

	if err := utils.WriteFile(csrLocation, csr); err != nil {
		klog.Errorf("Unable to write the CSR file in location: %s", csrLocation)
		return err
	}

	if err := utils.WriteFile(keyLocation, privateKey); err != nil {
		klog.Errorf("Unable to write the KEY file in location: %s", keyLocation)
		return err
	}

	if len(certificate) > 0 {
		if err := utils.WriteFile(certLocation, certificate); err != nil {
			klog.Errorf("Unable to write the CRT file in location: %s", certLocation)
			return err
		}
	}

	return nil
}

// createCSRSecret creates the CSR secret with the given key and csr.
func createCSRSecret(ctx context.Context,
	clientset kubernetes.Interface, privateKey, csr []byte,
	nodeName, namespace string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: namespace,
			Labels: map[string]string{
				csrSecretLabel: "true",
			},
		},
		Data: map[string][]byte{
			csrPrivateKey: privateKey,
			csrKey:        csr,
		},
	}

	_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

// StoreCertificate stores the retrieved certificate into the CSR secret.
func StoreCertificate(ctx context.Context,
	clientset kubernetes.Interface, certificate []byte,
	namespace, nodeName string) error {
	secret, _, err := GetCSRSecret(ctx, clientset, nodeName, namespace)
	if err != nil {
		klog.Error(err)
		return err
	}

	secret.Data[csrCertificate] = certificate

	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

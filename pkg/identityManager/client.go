// Copyright 2019-2021 The Liqo Authors
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
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// GetConfig gets a rest config from the secret, given the remote clusterID and (optionally) the namespace.
// This rest config con be used to create a client to the remote cluster.
func (certManager *identityManager) GetConfig(remoteClusterID, namespace string) (*rest.Config, error) {
	var secret *v1.Secret
	var err error

	if namespace == "" {
		secret, err = certManager.getSecret(remoteClusterID)
	} else {
		secret, err = certManager.getSecretInNamespace(remoteClusterID, namespace)
	}
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	if certManager.isAwsIdentity(secret) {
		return certManager.getIAMConfig(secret, remoteClusterID)
	}

	// retrieve the data required to build the rest config

	keyData, ok := secret.Data[privateKeySecretKey]
	if !ok {
		klog.Errorf("key %v not found in secret %v/%v", privateKeySecretKey, secret.Namespace, secret.Name)
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteClusterID)
		return nil, err
	}

	certData, ok := secret.Data[certificateSecretKey]
	if !ok {
		klog.Errorf("key %v not found in secret %v/%v", certificateSecretKey, secret.Namespace, secret.Name)
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteClusterID)
		return nil, err
	}

	caData, ok := secret.Data[apiServerCaSecretKey]
	if !ok {
		// CAData may be nil if the remote cluster exposes the API Server with a trusted certificate
		caData = nil
	}

	host, ok := secret.Data[apiServerURLSecretKey]
	if !ok {
		klog.Errorf("key %v not found in secret %v/%v", apiServerURLSecretKey, secret.Namespace, secret.Name)
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteClusterID)
		return nil, err
	}

	// create the rest config that can be used to create a client
	return &rest.Config{
		Host:    string(host),
		APIPath: "/apis",
		TLSClientConfig: rest.TLSClientConfig{
			CertData: certData,
			KeyData:  keyData,
			CAData:   caData,
		},
	}, nil
}

// GetRemoteTenantNamespace returns the tenant namespace that
// the remote cluster assigned to this peering.
func (certManager *identityManager) GetRemoteTenantNamespace(
	remoteClusterID, localTenantNamespaceName string) (string, error) {
	var secret *v1.Secret
	var err error

	if localTenantNamespaceName == "" {
		secret, err = certManager.getSecret(remoteClusterID)
	} else {
		secret, err = certManager.getSecretInNamespace(remoteClusterID, localTenantNamespaceName)
	}
	if err != nil {
		klog.Error(err)
		return "", err
	}

	remoteNamespace, ok := secret.Data[namespaceSecretKey]
	if !ok {
		klog.Errorf("key %v not found in secret %v/%v", namespaceSecretKey, secret.Namespace, secret.Name)
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteClusterID)
		return "", err
	}
	return string(remoteNamespace), nil
}

func (certManager *identityManager) getIAMConfig(secret *v1.Secret, remoteClusterID string) (*rest.Config, error) {
	return certManager.iamTokenManager.getConfig(secret, remoteClusterID)
}

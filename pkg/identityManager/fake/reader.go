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

package fake

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
)

var _ identitymanager.IdentityReader = &IdentityReader{}

// IdentityReader is a struct implementing an IdentityReader mock for testing purposes.
type IdentityReader struct {
	configs     map[string]*rest.Config
	namespaces  map[string]string
	secretNames map[string]string
}

// NewIdentityReader creates a new identityReader instance.
func NewIdentityReader() *IdentityReader {
	return &IdentityReader{
		configs:     make(map[string]*rest.Config),
		namespaces:  make(map[string]string),
		secretNames: make(map[string]string),
	}
}

// Add adds the associations about a remote cluster to the identityReader.
func (i *IdentityReader) Add(clusterID, namespace, secretName string, restcfg *rest.Config) *IdentityReader {
	i.configs[clusterID] = restcfg
	i.namespaces[clusterID] = namespace
	i.secretNames[clusterID] = secretName
	return i
}

// GetConfig retrieves the rest config associated with a remote cluster.
func (i *IdentityReader) GetConfig(remoteCluster discoveryv1alpha1.ClusterIdentity, namespace string) (*rest.Config, error) {
	if restcfg, found := i.configs[remoteCluster.ClusterID]; found {
		return restcfg, nil
	}
	return nil, fmt.Errorf("remote cluster ID %v not found", remoteCluster.ClusterID)
}

// GetSecretNamespacedName retrieves the secret namespaced name associated with a remote cluster.
func (i *IdentityReader) GetSecretNamespacedName(remoteCluster discoveryv1alpha1.ClusterIdentity,
	_ string) (types.NamespacedName, error) {
	if ns, found := i.namespaces[remoteCluster.ClusterID]; found {
		if secretName, found := i.secretNames[remoteCluster.ClusterID]; found {
			return types.NamespacedName{
				Namespace: ns,
				Name:      secretName,
			}, nil
		}
		return types.NamespacedName{}, fmt.Errorf("secret name for remote cluster ID %v not found", remoteCluster.ClusterID)
	}
	return types.NamespacedName{}, fmt.Errorf("remote cluster ID %v not found", remoteCluster.ClusterID)
}

// GetConfigFromSecret retrieves the rest config associated with a remote cluster.
func (i *IdentityReader) GetConfigFromSecret(_ *corev1.Secret) (*rest.Config, error) {
	panic("implement me")
}

// GetRemoteTenantNamespace retrieves the tenant namespace associated with a remote cluster.
func (i *IdentityReader) GetRemoteTenantNamespace(remoteCluster discoveryv1alpha1.ClusterIdentity, namespace string) (string, error) {
	if namespace, found := i.namespaces[remoteCluster.ClusterID]; found {
		return namespace, nil
	}
	return "", fmt.Errorf("remote cluster ID %v not found", remoteCluster.ClusterID)
}

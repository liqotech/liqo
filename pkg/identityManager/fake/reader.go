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

package fake

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
)

var _ identitymanager.IdentityReader = &IdentityReader{}

// IdentityReader is a struct implementing an IdentityReader mock for testing purposes.
type IdentityReader struct {
	configs     map[liqov1beta1.ClusterID]*rest.Config
	namespaces  map[liqov1beta1.ClusterID]string
	secretNames map[liqov1beta1.ClusterID]string
}

// NewIdentityReader creates a new identityReader instance.
func NewIdentityReader() *IdentityReader {
	return &IdentityReader{
		configs:     make(map[liqov1beta1.ClusterID]*rest.Config),
		namespaces:  make(map[liqov1beta1.ClusterID]string),
		secretNames: make(map[liqov1beta1.ClusterID]string),
	}
}

// Add adds the associations about a remote cluster to the identityReader.
func (i *IdentityReader) Add(clusterID liqov1beta1.ClusterID, namespace, secretName string, restcfg *rest.Config) *IdentityReader {
	i.configs[clusterID] = restcfg
	i.namespaces[clusterID] = namespace
	i.secretNames[clusterID] = secretName
	return i
}

// GetConfig retrieves the rest config associated with a remote cluster.
func (i *IdentityReader) GetConfig(remoteCluster liqov1beta1.ClusterID, _ string) (*rest.Config, error) {
	if restcfg, found := i.configs[remoteCluster]; found {
		return restcfg, nil
	}
	return nil, fmt.Errorf("remote cluster ID %v not found", remoteCluster)
}

// GetSecretNamespacedName retrieves the secret namespaced name associated with a remote cluster.
func (i *IdentityReader) GetSecretNamespacedName(remoteCluster liqov1beta1.ClusterID,
	_ string) (types.NamespacedName, error) {
	if ns, found := i.namespaces[remoteCluster]; found {
		if secretName, found := i.secretNames[remoteCluster]; found {
			return types.NamespacedName{
				Namespace: ns,
				Name:      secretName,
			}, nil
		}
		return types.NamespacedName{}, fmt.Errorf("secret name for remote cluster ID %v not found", remoteCluster)
	}
	return types.NamespacedName{}, fmt.Errorf("remote cluster ID %v not found", remoteCluster)
}

// GetConfigFromSecret retrieves the rest config associated with a remote cluster.
func (i *IdentityReader) GetConfigFromSecret(_ liqov1beta1.ClusterID, _ *corev1.Secret) (*rest.Config, error) {
	panic("implement me")
}

// GetRemoteTenantNamespace retrieves the tenant namespace associated with a remote cluster.
func (i *IdentityReader) GetRemoteTenantNamespace(remoteCluster liqov1beta1.ClusterID, _ string) (string, error) {
	if namespace, found := i.namespaces[remoteCluster]; found {
		return namespace, nil
	}
	return "", fmt.Errorf("remote cluster ID %v not found", remoteCluster)
}

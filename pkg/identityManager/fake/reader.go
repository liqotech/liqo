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

package fake

import (
	"fmt"

	"k8s.io/client-go/rest"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// IdentityReader is a struct implementing an IdentityReader mock for testing purposes.
type IdentityReader struct {
	configs    map[string]*rest.Config
	namespaces map[string]string
}

// NewIdentityReader creates a new identityReader instance.
func NewIdentityReader() *IdentityReader {
	return &IdentityReader{
		configs:    make(map[string]*rest.Config),
		namespaces: make(map[string]string),
	}
}

// Add adds the associations about a remote cluster to the identityReader.
func (i *IdentityReader) Add(clusterID, namespace string, restcfg *rest.Config) *IdentityReader {
	i.configs[clusterID] = restcfg
	i.namespaces[clusterID] = namespace
	return i
}

// GetConfig retrieves the rest config associated with a remote cluster.
func (i *IdentityReader) GetConfig(remoteCluster discoveryv1alpha1.ClusterIdentity, namespace string) (*rest.Config, error) {
	if restcfg, found := i.configs[remoteCluster.ClusterID]; found {
		return restcfg, nil
	}
	return nil, fmt.Errorf("remote cluster ID %v not found", remoteCluster.ClusterID)
}

// GetRemoteTenantNamespace retrieves the tenant namespace associated with a remote cluster.
func (i *IdentityReader) GetRemoteTenantNamespace(remoteCluster discoveryv1alpha1.ClusterIdentity, namespace string) (string, error) {
	if namespace, found := i.namespaces[remoteCluster.ClusterID]; found {
		return namespace, nil
	}
	return "", fmt.Errorf("remote cluster ID %v not found", remoteCluster.ClusterID)
}

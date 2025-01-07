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

package remotemetrics

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRemoteMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Remote Metrics Suite")
}

type fakeResourceGetter struct {
	namespaces map[string][]MappedNamespace
	pods       map[string]map[string][]string
	nodes      []string
}

// GetNamespaces returns the names of all namespaces in the cluster owned by the remote clusterID.
func (m *fakeResourceGetter) GetNamespaces(ctx context.Context, clusterID string) []MappedNamespace {
	return m.namespaces[clusterID]
}

// GetPodNames returns the names of all pods in the cluster owned by the remote clusterID and scheduled in the given node.
func (m *fakeResourceGetter) GetPodNames(ctx context.Context, clusterID, node string) []string {
	return m.pods[node][clusterID]
}

// GetNodeNames returns the names of all physical nodes in the cluster.
func (m *fakeResourceGetter) GetNodeNames(ctx context.Context) []string {
	return m.nodes
}

type fakeRawGetter struct {
	data map[string][]byte
}

func (rg *fakeRawGetter) get(ctx context.Context, nodeName, path string) ([]byte, error) {
	return rg.data[nodeName], nil
}

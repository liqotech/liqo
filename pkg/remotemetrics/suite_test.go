// Copyright 2019-2026 The Liqo Authors
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestRemoteMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Remote Metrics Suite")
}

type fakeResourceGetter struct {
	namespaces map[string][]MappedNamespace
	pods       map[string]map[string][]string
	nodes      []corev1.Node
}

// GetNamespaces returns the names of all namespaces in the cluster owned by the remote clusterID.
func (m *fakeResourceGetter) GetNamespaces(ctx context.Context, clusterID string) []MappedNamespace {
	return m.namespaces[clusterID]
}

// GetPodsPerNode returns, for each node, the names of the pods in the cluster owned by the remote clusterID.
func (m *fakeResourceGetter) GetPodsPerNode(ctx context.Context, clusterID string) map[string][]string {
	res := map[string][]string{}
	for node, podsByCluster := range m.pods {
		if pods, ok := podsByCluster[clusterID]; ok {
			res[node] = pods
		}
	}
	return res
}

// GetNodes returns the names of the nodes matching the given selector (nil matches all).
func (m *fakeResourceGetter) GetNodes(_ context.Context, selector labels.Selector) []string {
	res := []string{}
	for i := range m.nodes {
		if selector == nil || selector.Empty() || selector.Matches(labels.Set(m.nodes[i].GetLabels())) {
			res = append(res, m.nodes[i].GetName())
		}
	}
	return res
}

type fakeRawGetter struct {
	data map[string][]byte
	errs map[string]error
}

func (rg *fakeRawGetter) get(ctx context.Context, nodeName, path string) ([]byte, error) {
	return rg.data[nodeName], rg.errs[nodeName]
}

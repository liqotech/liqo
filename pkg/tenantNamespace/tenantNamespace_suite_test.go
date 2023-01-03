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

package tenantnamespace

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestTenantNamespace(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TenantNamespace Suite")
}

var (
	ctx         context.Context
	cancel      context.CancelFunc
	cluster     testutil.Cluster
	homeCluster discoveryv1alpha1.ClusterIdentity

	namespaceManager Manager
)

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
	ctx, cancel = context.WithCancel(context.Background())

	homeCluster = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "home-cluster-id",
		ClusterName: "home-cluster-name",
	}

	var err error
	cluster, _, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
	Expect(err).ToNot(HaveOccurred())

	namespaceManager = NewCachedManager(ctx, cluster.GetClient())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(cluster.GetEnv().Stop()).To(Succeed())
})

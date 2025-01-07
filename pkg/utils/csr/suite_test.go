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

package csr

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestCsr(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSR Suite")
}

var (
	cluster testutil.Cluster
	err     error
	ctx     context.Context
	cancel  context.CancelFunc
)

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())

	cluster, _, err = testutil.NewTestCluster([]string{
		filepath.Join("..", "..", "..", "deployments", "liqo", "charts", "liqo-crds", "crds"),
	})
	Expect(err).To(BeNil())
})

var _ = AfterSuite(func() {
	cancel()

	err := cluster.GetEnv().Stop()
	Expect(err).To(BeNil())
})

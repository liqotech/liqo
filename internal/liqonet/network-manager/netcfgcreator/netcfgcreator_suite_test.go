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

package netcfgcreator

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var (
	testcluster testutil.Cluster
)

func TestForeignclusterwatcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NetworkConfigCreator Suite")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
	clstr, _, err := testutil.NewTestCluster([]string{filepath.Join("..", "..", "..", "..", "deployments", "liqo", "crds")})
	Expect(err).ToNot(HaveOccurred())

	testcluster = clstr
})

var _ = AfterSuite(func() {
	Expect(testcluster.GetEnv().Stop()).To(Succeed())
})

package csr

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
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

	cluster, _, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
	Expect(err).To(BeNil())
})

var _ = AfterSuite(func() {
	cancel()

	err := cluster.GetEnv().Stop()
	Expect(err).To(BeNil())
})

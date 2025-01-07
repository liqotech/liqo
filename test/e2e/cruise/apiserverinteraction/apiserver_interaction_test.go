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

package apiserverinteraction

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"

	"github.com/liqotech/liqo/test/e2e/testconsts"
	"github.com/liqotech/liqo/test/e2e/testutils/apiserver"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// testName is the name of this E2E test.
	testName = "APISERVER_INTERACTION"
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)

	if util.GetEnvironmentVariableOrDie(testconsts.InfrastructureEnvVar) == testconsts.ProviderK3s {
		t.Skipf("Skipping %s test on k3s", testName)
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var (
	ctx           = context.Background()
	testContext   = tester.GetTester(ctx)
	interval      = config.Interval
	timeout       = config.Timeout
	namespaceName = util.GetNameNamespaceTest(testName)
)

var _ = Describe("Liqo E2E", func() {
	Context("API server interaction Testing", func() {
		const (
			retries             = 60
			sleepBetweenRetries = 1 * time.Second
		)

		var (
			options *k8s.KubectlOptions
			v       *version.Info
		)

		BeforeEach(func() {
			client, err := discovery.NewDiscoveryClientForConfig(testContext.Clusters[0].Config)
			Expect(err).ToNot(HaveOccurred())
			v, err = client.ServerVersion()
			Expect(err).ToNot(HaveOccurred())
			// trim special characters from the version string
			v.Major = strings.Trim(v.Major, "+")
			v.Minor = strings.Trim(v.Minor, "+")

			options = k8s.NewKubectlOptions("", testContext.Clusters[0].KubeconfigPath, namespaceName)
		})

		AfterEach(func() {
			Eventually(func() error {
				return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[0].NativeClient, namespaceName)
			}, timeout, interval).Should(Succeed())
		})

		It("run offloaded kubectl pod", func() {
			// This test offloads a kubectl pod to a remote cluster, binds it to a service account with permissions to get the pods in the
			// current namespace (in the local cluster), and checks whether it can successfully retrieve the list of running pods (itself).

			By("Creating the different resources")
			_, err := util.EnforceNamespace(ctx, testContext.Clusters[0].NativeClient, testContext.Clusters[0].Cluster, namespaceName)
			Expect(err).ToNot(HaveOccurred())

			Expect(util.OffloadNamespace(testContext.Clusters[0].KubeconfigPath, namespaceName,
				"--pod-offloading-strategy", "Remote")).To(Succeed())
			time.Sleep(2 * time.Second)

			Expect(apiserver.CreateServiceAccount(ctx, testContext.Clusters[0].ControllerClient, namespaceName)).To(Succeed())
			Expect(apiserver.CreateRoleBinding(ctx, testContext.Clusters[0].ControllerClient, namespaceName)).To(Succeed())

			By("Deploying the kubectl job")
			Expect(apiserver.CreateKubectlJob(ctx, testContext.Clusters[0].ControllerClient, namespaceName, v)).To(Succeed())

			By("Waiting for the job to complete")
			Expect(k8s.WaitUntilJobSucceedE(GinkgoT(), options, apiserver.JobName, retries, sleepBetweenRetries)).To(Succeed())

			By("Retrieving the pod logs, and asserting their correctness")
			podName, retrieved, err := apiserver.RetrieveJobLogs(ctx, testContext.Clusters[0].NativeClient, namespaceName)
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.TrimSuffix(retrieved, "\n")).To(Equal(podName))
		})
	})

})

var _ = AfterSuite(func() {
	for i := range testContext.Clusters {
		Eventually(func() error {
			return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[i].NativeClient, namespaceName)
		}, timeout, interval).Should(Succeed())
	}
})

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

package storageprovisioner

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var (
	testEnv       envtest.Environment
	testEnvClient kubernetes.Interface
)

func TestStorageProvisioner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Storage Provisioner")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()

	testEnv = envtest.Environment{}
	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	// Need to use a real client, as server side apply seems not to be currently supported by the fake one.
	testEnvClient = kubernetes.NewForConfigOrDie(cfg)
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})

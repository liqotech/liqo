// Copyright 2019-2021 The Liqo Authors
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

package liqodeploymentctrl

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/liqo-deployment-controller/testutils"
	errorsmanagement "github.com/liqotech/liqo/pkg/utils/errorsManagement"
)

const (
	// Different namespaces for the two core test Contexts.
	namespaceContext1 = "ns-test-1"
	namespaceContext2 = "ns-test-2"

	virtualNode1Name = "liqo-6a0e9f-b52-4ed0"
	virtualNode2Name = "liqo-899890-dsd-323s"
	virtualNode3Name = "liqo-refc453-ds43d-43rs"

	remoteClusterID1 = "1-6a0e9f-b52-4ed0"
	remoteClusterID2 = "2-899890-dsd-323s"
	remoteClusterID3 = "3-refc453-ds43d-43rs"
)

var (
	envTest    *envtest.Environment
	k8sClient  client.Client
	controller = &LiqoDeploymentReconciler{}
	ctx        context.Context
	cancel     context.CancelFunc
)

func TestLiqoDeploymentController(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(Fail)
	RunSpecs(t, "LiqoDeploymentReconciler Suite")
}

func createCluster() {
	By("Bootstrapping test environment")
	ctx, cancel = context.WithCancel(context.Background())
	envTest = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "deployments", "liqo", "crds"),
			filepath.Join("..", "..", "..", "externalcrds"),
		},
	}

	var err error
	cfg, err := envTest.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = offv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	By("Starting a new manager")
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	Expect(err).ToNot(HaveOccurred())

	// Disabling panic on failure.
	errorsmanagement.SetPanicOnErrorMode(false)

	controller = NewLiqoDeploymentReconciler(k8sManager.GetClient(), k8sManager.GetScheme())

	// Starting the manager
	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())
}

func initializeResources() {
	// Every Context in the test suite will use a different namespace to avoid mess.
	namespace1 := testutils.GetNamespace(namespaceContext1)
	namespace2 := testutils.GetNamespace(namespaceContext2)

	namespaceOffloading1 := testutils.GetNamespaceOffloading(testutils.GetNodeSelector(testutils.EmptySelector),
		namespaceContext1)
	namespaceOffloading2 := testutils.GetNamespaceOffloading(testutils.GetNodeSelector(testutils.DefaultSelector),
		namespaceContext2)

	// REGION=A PROVIDER=A
	virtualNode1 := testutils.GetVirtualNode(virtualNode1Name, remoteClusterID1, testutils.RegionA, testutils.ProviderA)
	// REGION=B PROVIDER=B
	virtualNode2 := testutils.GetVirtualNode(virtualNode2Name, remoteClusterID2, testutils.RegionB, testutils.ProviderB)
	// REGION=C PROVIDER=C
	virtualNode3 := testutils.GetVirtualNode(virtualNode3Name, remoteClusterID3, testutils.RegionC, testutils.ProviderC)

	By("creating the necessary resources in the cluster")
	Eventually(func() error { return k8sClient.Create(ctx, namespace1) }).Should(Succeed())
	Eventually(func() error { return k8sClient.Create(ctx, namespaceOffloading1) }).Should(Succeed())
	Eventually(func() error { return k8sClient.Create(ctx, namespace2) }).Should(Succeed())
	Eventually(func() error { return k8sClient.Create(ctx, namespaceOffloading2) }).Should(Succeed())
	Eventually(func() error { return k8sClient.Create(ctx, virtualNode1) }).Should(Succeed())
	Eventually(func() error { return k8sClient.Create(ctx, virtualNode2) }).Should(Succeed())
	Eventually(func() error { return k8sClient.Create(ctx, virtualNode3) }).Should(Succeed())

}

func destroyCluster() {
	By("tearing down the test environment")
	cancel()
	err := envTest.Stop()
	Expect(err).ToNot(HaveOccurred())
}

var _ = BeforeSuite(func() {
	createCluster()
	initializeResources()
})

var _ = AfterSuite(func() {
	destroyCluster()
})

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

package authenticationtoken

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	clientset kubernetes.Interface
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestAuthenticationTokenUtils(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(Fail)
	RunSpecs(t, "AuthenticationTokenUtils")
}

func createCluster() {
	By("Bootstrapping test environment")
	ctx, cancel = context.WithCancel(context.Background())
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "deployments", "liqo", "crds"),
		},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).To(Succeed())
	Expect(cfg).ToNot(BeNil())

	err = discoveryv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).To(Succeed())

	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).To(Succeed())
	Expect(k8sClient).ToNot(BeNil())

	clientset = kubernetes.NewForConfigOrDie(cfg)
}

func destroyCluster() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
}

var _ = BeforeSuite(func() {
	createCluster()
})

var _ = AfterSuite(func() {
	destroyCluster()
})

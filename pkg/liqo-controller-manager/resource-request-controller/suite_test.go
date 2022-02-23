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

package resourcerequestoperator

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	resourcemonitors "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller/resource-monitors"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
)

var (
	cfg           *rest.Config
	k8sClient     client.Client
	homeCluster   discoveryv1alpha1.ClusterIdentity
	clientset     kubernetes.Interface
	testEnv       *envtest.Environment
	monitor       *resourcemonitors.LocalResourceMonitor
	scaledMonitor *resourcemonitors.ResourceScaler
	updater       *OfferUpdater
	ctx           context.Context
	cancel        context.CancelFunc
	group         sync.WaitGroup
)

func TestAPIs(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
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
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = discoveryv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = sharingv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	By("Starting a new manager")
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	Expect(err).ToNot(HaveOccurred())
	// Disabling panic on failure.
	liqoerrors.SetPanicOnErrorMode(false)
	clientset = kubernetes.NewForConfigOrDie(k8sManager.GetConfig())
	homeCluster = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "home-cluster-id",
		ClusterName: "home-cluster-name",
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: k8sManager.GetScheme()})
	Expect(err).ToNot(HaveOccurred())

	// Initializing a new notifier and adding it to the manager.
	localStorageClassName := ""
	enableStorage := true
	monitor = resourcemonitors.NewLocalMonitor(ctx, clientset, 5*time.Second)
	scaledMonitor = &resourcemonitors.ResourceScaler{Provider: monitor, Factor: DefaultScaleFactor}
	updater = NewOfferUpdater(ctx, k8sClient, homeCluster, nil, scaledMonitor, 5, localStorageClassName, enableStorage)

	Expect(k8sManager.Add(updater)).To(Succeed())

	// Adding ResourceRequest reconciler to the manager
	err = (&ResourceRequestReconciler{
		Client:                k8sClient,
		Scheme:                k8sManager.GetScheme(),
		HomeCluster:           homeCluster,
		OfferUpdater:          updater,
		EnableIncomingPeering: true,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	// Starting the manager
	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ResourcesNamespace2}})).To(Succeed())
}

func destroyCluster() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
	group.Wait()
}

var _ = BeforeSuite(func() {
	createCluster()
})

var _ = AfterSuite(func() {
	destroyCluster()
})

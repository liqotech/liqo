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

package resourcerequestoperator

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	capsulev1beta1 "github.com/clastix/capsule/api/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterid"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller/testutils"
	errorsmanagement "github.com/liqotech/liqo/pkg/utils/errorsManagement"
)

var (
	cfg           *rest.Config
	k8sClient     client.Client
	homeClusterID string
	clientset     kubernetes.Interface
	testEnv       *envtest.Environment
	broadcaster   Broadcaster
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
			filepath.Join("..", "..", "..", "externalcrds"),
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
	err = capsulev1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	By("Starting a new manager")
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	Expect(err).ToNot(HaveOccurred())
	// Disabling panic on failure.
	errorsmanagement.SetPanicOnErrorMode(false)
	clientset = kubernetes.NewForConfigOrDie(k8sManager.GetConfig())
	homeClusterID = clusterid.NewStaticClusterID("test-cluster").GetClusterID()

	// Initializing a new updater and adding it to the manager.
	updater := OfferUpdater{}
	updater.Setup(homeClusterID, k8sManager.GetScheme(), &broadcaster, k8sManager.GetClient(), nil)

	// Initializing a new broadcaster, starting it and adding it its configuration.
	err = broadcaster.SetupBroadcaster(clientset, &updater, 5*time.Second, testutils.DefaultScalePercentage, 5)
	Expect(err).ToNot(HaveOccurred())
	broadcaster.StartBroadcaster(ctx, &group)

	// Adding ResourceRequest reconciler to the manager
	err = (&ResourceRequestReconciler{
		Client:                k8sManager.GetClient(),
		Scheme:                k8sManager.GetScheme(),
		ClusterID:             homeClusterID,
		Broadcaster:           &broadcaster,
		EnableIncomingPeering: true,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	// Starting the manager
	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	ctx = context.TODO()
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

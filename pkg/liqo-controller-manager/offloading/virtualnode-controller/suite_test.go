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

package virtualnodectrl

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

const (
	nameVirtualNode1          = "virtual-node-1"
	nameVirtualNode2          = "virtual-node-2"
	nameSimpleNode            = "simple-node"
	remoteClusterID1          = "6a0e9f-b52-4ed0"
	remoteClusterName1        = "remote-1"
	remoteClusterID2          = "899890-dsd-323"
	remoteClusterName2        = "remote-2"
	remoteClusterIDSimpleNode = "909030-sd-3231"
	offloadingCluster1Label1  = "offloading.liqo.io/cluster-1"
	offloadingCluster1Label2  = "offloading.liqo.io/AWS"
	timeout                   = time.Second * 10
	interval                  = time.Millisecond * 250
)

var (
	ctx    context.Context
	cancel context.CancelFunc

	localID liqov1beta1.ClusterID = "local-ID"

	nms *offloadingv1beta1.NamespaceMapList

	virtualNode1     *offloadingv1beta1.VirtualNode
	virtualNode2     *offloadingv1beta1.VirtualNode
	simpleNode       *corev1.Node
	tenantNamespace1 *corev1.Namespace
	tenantNamespace2 *corev1.Namespace
	namespaceManager tenantnamespace.Manager
)

func TestVirtualNode(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VirtualNode Controller")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "deployments", "liqo", "charts", "liqo-crds", "crds"),
		},
		ErrorIfCRDPathMissing: true,
	}

	ctx, cancel = context.WithCancel(context.Background())
	testutil.LogsToGinkgoWriter()

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = offloadingv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = liqov1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: server.Options{BindAddress: "0"}, // this avoids port binding collision
	})
	Expect(err).ToNot(HaveOccurred())

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	namespaceManager = tenantnamespace.NewManager(kubernetes.NewForConfigOrDie(cfg), k8sClient.Scheme())

	vnr, err := NewVirtualNodeReconciler(ctx,
		k8sClient,
		scheme.Scheme,
		k8sManager.GetEventRecorderFor("virtualnode-controller"),
		localID,
		namespaceManager,
	)
	Expect(err).ToNot(HaveOccurred())

	err = (vnr).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	nms = &offloadingv1beta1.NamespaceMapList{}

	// create the 2 tenant namespace
	tenantNamespace1, err = namespaceManager.CreateNamespace(ctx, remoteClusterID1)
	Expect(err).ToNot(HaveOccurred())
	tenantNamespace2, err = namespaceManager.CreateNamespace(ctx, remoteClusterID2)
	Expect(err).ToNot(HaveOccurred())

	simpleNode = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nameSimpleNode,
			Labels: map[string]string{
				liqoconst.RemoteClusterID: remoteClusterIDSimpleNode,
				offloadingCluster1Label1:  "",
				offloadingCluster1Label2:  "",
			},
		},
	}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	Expect(testEnv.Stop()).To(Succeed())
})

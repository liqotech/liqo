// Copyright 2019-2024 The Liqo Authors
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/vkMachinery/forge"
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
	tenantNamespaceNameID1    = "liqo-tenant-namespace-1"
	tenantNamespaceNameID2    = "liqo-tenant-namespace-2"
	offloadingCluster1Label1  = "offloading.liqo.io/cluster-1"
	offloadingCluster1Label2  = "offloading.liqo.io/AWS"
	timeout                   = time.Second * 10
	interval                  = time.Millisecond * 250
)

var (
	ctx    context.Context
	cancel context.CancelFunc

	localIdentity = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "local-ID",
		ClusterName: "local-name",
	}

	nms *mapsv1alpha1.NamespaceMapList

	virtualNode1     *mapsv1alpha1.VirtualNode
	virtualNode2     *mapsv1alpha1.VirtualNode
	simpleNode       *corev1.Node
	tenantNamespace1 *corev1.Namespace
	tenantNamespace2 *corev1.Namespace
	foreignCluster1  *discoveryv1alpha1.ForeignCluster
	foreignCluster2  *discoveryv1alpha1.ForeignCluster
)

func TestVirtualNode(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VirtualNode Controller")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "deployments", "liqo", "charts", "liqo-crds", "crds"),
		},
	}

	ctx, cancel = context.WithCancel(context.Background())
	testutil.LogsToGinkgoWriter()

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = mapsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = discoveryv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: server.Options{BindAddress: "0"}, // this avoids port binding collision
	})
	Expect(err).ToNot(HaveOccurred())

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	vnr, err := NewVirtualNodeReconciler(ctx,
		k8sClient,
		scheme.Scheme,
		k8sManager.GetEventRecorderFor("virtualnode-controller"),
		&localIdentity,
		&forge.VirtualKubeletOpts{},
	)
	Expect(err).ToNot(HaveOccurred())

	err = (vnr).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	nms = &mapsv1alpha1.NamespaceMapList{}

	tenantNamespace1 = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenantNamespaceNameID1,
		},
	}

	tenantNamespace2 = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenantNamespaceNameID2,
		},
	}

	foreignCluster1 = &discoveryv1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: remoteClusterID1,
			Labels: map[string]string{
				discovery.ClusterIDLabel: remoteClusterID1,
			},
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ForeignAuthURL:         "https://example.com",
			ClusterIdentity:        discoveryv1alpha1.ClusterIdentity{ClusterID: remoteClusterID1, ClusterName: "remote-1"},
			OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			InsecureSkipTLSVerify:  pointer.BoolPtr(true),
		},
	}

	foreignCluster2 = &discoveryv1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: remoteClusterID2,
			Labels: map[string]string{
				discovery.ClusterIDLabel: remoteClusterID2,
			},
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ForeignAuthURL:         "https://example.com",
			ClusterIdentity:        discoveryv1alpha1.ClusterIdentity{ClusterID: remoteClusterID2, ClusterName: "remote-2"},
			OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			InsecureSkipTLSVerify:  pointer.BoolPtr(true),
		},
	}

	// create the 2 tenant namespaces and the foreignClusters.
	Expect(k8sClient.Create(ctx, foreignCluster1)).To(Succeed())
	Expect(k8sClient.Create(ctx, foreignCluster2)).To(Succeed())
	Expect(k8sClient.Create(ctx, tenantNamespace1)).To(Succeed())
	Expect(k8sClient.Create(ctx, tenantNamespace2)).To(Succeed())

	fc := &discoveryv1alpha1.ForeignCluster{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterID1}, fc)).To(Succeed())
	fc.Status = discoveryv1alpha1.ForeignClusterStatus{
		TenantNamespace: discoveryv1alpha1.TenantNamespaceType{
			Local:  tenantNamespaceNameID1,
			Remote: "remote",
		},
	}
	Expect(k8sClient.Status().Update(ctx, fc)).To(Succeed())

	fc = &discoveryv1alpha1.ForeignCluster{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterID2}, fc)).To(Succeed())
	fc.Status = discoveryv1alpha1.ForeignClusterStatus{
		TenantNamespace: discoveryv1alpha1.TenantNamespaceType{
			Local:  tenantNamespaceNameID2,
			Remote: "remote",
		},
	}
	Expect(k8sClient.Status().Update(ctx, fc)).To(Succeed())

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

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

package nsoffctrl

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (
	// namespace where the NamespaceMaps are created.
	mapNamespaceName = "default"
	mapNumber        = 3

	namespaceName = "namespace"

	virtualNode1Name = "liqo-remote-1"
	virtualNode2Name = "liqo-remote-2"
	virtualNode3Name = "liqo-remote-3"

	regionA     = "A"
	regionB     = "B"
	providerAWS = "AWS"
	providerGKE = "GKE"
)

var (
	ctx    context.Context
	cancel context.CancelFunc

	localCluster   liqov1beta1.ClusterID = "local-cluster-id"
	remoteCluster1 liqov1beta1.ClusterID = "remote-cluster-1-id"
	remoteCluster2 liqov1beta1.ClusterID = "remote-cluster-2-id"
	remoteCluster3 liqov1beta1.ClusterID = "remote-cluster-3-id"

	homeCfg        *rest.Config
	cl             client.Client
	homeClusterEnv *envtest.Environment

	// Resources.

	tenantNamespace1 *corev1.Namespace
	tenantNamespace2 *corev1.Namespace
	tenantNamespace3 *corev1.Namespace

	virtualNode1 *offloadingv1beta1.VirtualNode
	virtualNode2 *offloadingv1beta1.VirtualNode
	virtualNode3 *offloadingv1beta1.VirtualNode

	node1 *corev1.Node
	node2 *corev1.Node
	node3 *corev1.Node

	nm1 *offloadingv1beta1.NamespaceMap
	nm2 *offloadingv1beta1.NamespaceMap
	nm3 *offloadingv1beta1.NamespaceMap

	namespace *corev1.Namespace
	nsoff     *offloadingv1beta1.NamespaceOffloading
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NamespaceOffloadingController Suite")
}

var _ = BeforeSuite(func() {
	SetDefaultEventuallyTimeout(3 * time.Second)
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultConsistentlyDuration(500 * time.Millisecond)
	SetDefaultEventuallyPollingInterval(50 * time.Millisecond)

	ForgeNamespaceMap := func(cluster liqov1beta1.ClusterID) *offloadingv1beta1.NamespaceMap {
		return &offloadingv1beta1.NamespaceMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      string(cluster),
				Namespace: mapNamespaceName,
				Labels: map[string]string{
					liqoconst.ReplicationRequestedLabel:   "true",
					liqoconst.RemoteClusterID:             string(cluster),
					liqoconst.ReplicationDestinationLabel: string(cluster),
				},
			},
		}
	}

	By("bootstrapping test environments")
	ctx, cancel = context.WithCancel(context.Background())
	testutil.LogsToGinkgoWriter()

	homeClusterEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "deployments", "liqo", "charts", "liqo-crds", "crds"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error

	// Home cluster
	homeCfg, err = homeClusterEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(homeCfg).ToNot(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = offloadingv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = offloadingv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sManager, err := ctrl.NewManager(homeCfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	// Initialize a non-cached client to reduce race conditions during tests.
	cl, err = client.New(homeCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())

	err = (&NamespaceOffloadingReconciler{
		Client:       k8sManager.GetClient(),
		Recorder:     k8sManager.GetEventRecorderFor("namespaceoffloading-controller"),
		LocalCluster: localCluster,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	// Necessary resources in HomeCluster.

	tenantNamespace1 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tenant-namespace-1"}}
	tenantNamespace2 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tenant-namespace-2"}}
	tenantNamespace3 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tenant-namespace-3"}}

	virtualNode1 = &offloadingv1beta1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualNode1Name,
			Namespace: tenantNamespace1.Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:                  liqoconst.TypeNode,
				liqoconst.RemoteClusterID:            string(remoteCluster1),
				liqoconst.TopologyRegionClusterLabel: regionA,
				liqoconst.ProviderClusterLabel:       providerAWS,
			},
		},
		Spec: offloadingv1beta1.VirtualNodeSpec{
			ClusterID: remoteCluster1,
		},
	}

	virtualNode2 = &offloadingv1beta1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualNode2Name,
			Namespace: tenantNamespace2.Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:                  liqoconst.TypeNode,
				liqoconst.RemoteClusterID:            string(remoteCluster2),
				liqoconst.TopologyRegionClusterLabel: regionB,
				liqoconst.ProviderClusterLabel:       providerGKE,
			},
		},
		Spec: offloadingv1beta1.VirtualNodeSpec{
			ClusterID: remoteCluster2,
		},
	}

	virtualNode3 = &offloadingv1beta1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualNode3Name,
			Namespace: tenantNamespace3.Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:                  liqoconst.TypeNode,
				liqoconst.RemoteClusterID:            string(remoteCluster3),
				liqoconst.TopologyRegionClusterLabel: regionA,
				liqoconst.ProviderClusterLabel:       providerGKE,
			},
		},
		Spec: offloadingv1beta1.VirtualNodeSpec{
			ClusterID: remoteCluster3,
		},
	}

	node1 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode1Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:                  liqoconst.TypeNode,
				liqoconst.RemoteClusterID:            string(remoteCluster1),
				liqoconst.TopologyRegionClusterLabel: regionA,
				liqoconst.ProviderClusterLabel:       providerAWS,
			},
		},
	}

	node2 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode2Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:                  liqoconst.TypeNode,
				liqoconst.RemoteClusterID:            string(remoteCluster2),
				liqoconst.TopologyRegionClusterLabel: regionB,
				liqoconst.ProviderClusterLabel:       providerGKE,
			},
		},
	}

	node3 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode3Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:                  liqoconst.TypeNode,
				liqoconst.RemoteClusterID:            string(remoteCluster3),
				liqoconst.TopologyRegionClusterLabel: regionA,
				liqoconst.ProviderClusterLabel:       providerGKE,
			},
		},
	}

	nm1 = ForgeNamespaceMap(remoteCluster1)
	nm2 = ForgeNamespaceMap(remoteCluster2)
	nm3 = ForgeNamespaceMap(remoteCluster3)

	Expect(cl.Create(ctx, tenantNamespace1)).Should(Succeed())
	Expect(cl.Create(ctx, tenantNamespace2)).Should(Succeed())
	Expect(cl.Create(ctx, tenantNamespace3)).Should(Succeed())

	Expect(cl.Create(ctx, virtualNode1)).Should(Succeed())
	Expect(cl.Create(ctx, virtualNode2)).Should(Succeed())
	Expect(cl.Create(ctx, virtualNode3)).Should(Succeed())

	Expect(cl.Create(ctx, node1)).Should(Succeed())
	Expect(cl.Create(ctx, node2)).Should(Succeed())
	Expect(cl.Create(ctx, node3)).Should(Succeed())

	Expect(cl.Create(ctx, nm1)).Should(Succeed())
	Expect(cl.Create(ctx, nm2)).Should(Succeed())
	Expect(cl.Create(ctx, nm3)).Should(Succeed())

	namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	Expect(cl.Create(ctx, namespace)).To(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	Expect(homeClusterEnv.Stop()).To(Succeed())
})

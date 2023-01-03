// Copyright 2019-2023 The Liqo Authors
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

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
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

	localCluster = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "local-cluster-id",
		ClusterName: "local-cluster-name",
	}
	remoteCluster1 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "remote-cluster-1-id",
		ClusterName: "remote-cluster-1",
	}
	remoteCluster2 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "remote-cluster-2-id",
		ClusterName: "remote-cluster-2",
	}
	remoteCluster3 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "remote-cluster-3-id",
		ClusterName: "remote-cluster-3",
	}

	homeCfg        *rest.Config
	cl             client.Client
	homeClusterEnv *envtest.Environment

	// Resources.
	virtualNode1 *corev1.Node
	virtualNode2 *corev1.Node
	virtualNode3 *corev1.Node

	nm1 *vkv1alpha1.NamespaceMap
	nm2 *vkv1alpha1.NamespaceMap
	nm3 *vkv1alpha1.NamespaceMap

	namespace *corev1.Namespace
	nsoff     *offv1alpha1.NamespaceOffloading
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

	ForgeNamespaceMap := func(cluster discoveryv1alpha1.ClusterIdentity) *vkv1alpha1.NamespaceMap {
		return &vkv1alpha1.NamespaceMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.ClusterName,
				Namespace: mapNamespaceName,
				Labels: map[string]string{
					liqoconst.ReplicationRequestedLabel:   "true",
					liqoconst.RemoteClusterID:             cluster.ClusterID,
					liqoconst.ReplicationDestinationLabel: cluster.ClusterID,
				},
			},
		}
	}

	By("bootstrapping test environments")
	ctx, cancel = context.WithCancel(context.Background())
	testutil.LogsToGinkgoWriter()

	homeClusterEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
	}

	var err error

	// Home cluster
	homeCfg, err = homeClusterEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(homeCfg).ToNot(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = vkv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = offv1alpha1.AddToScheme(scheme.Scheme)
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

	// Necessary resources in HomeCluster
	virtualNode1 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode1Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:                  liqoconst.TypeNode,
				liqoconst.RemoteClusterID:            remoteCluster1.ClusterID,
				liqoconst.TopologyRegionClusterLabel: regionA,
				liqoconst.ProviderClusterLabel:       providerAWS,
			},
		},
	}

	virtualNode2 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode2Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:                  liqoconst.TypeNode,
				liqoconst.RemoteClusterID:            remoteCluster2.ClusterID,
				liqoconst.TopologyRegionClusterLabel: regionB,
				liqoconst.ProviderClusterLabel:       providerGKE,
			},
		},
	}

	virtualNode3 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode3Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:                  liqoconst.TypeNode,
				liqoconst.RemoteClusterID:            remoteCluster3.ClusterID,
				liqoconst.TopologyRegionClusterLabel: regionA,
				liqoconst.ProviderClusterLabel:       providerGKE,
			},
		},
	}

	nm1 = ForgeNamespaceMap(remoteCluster1)
	nm2 = ForgeNamespaceMap(remoteCluster2)
	nm3 = ForgeNamespaceMap(remoteCluster3)

	Expect(cl.Create(ctx, virtualNode1)).Should(Succeed())
	Expect(cl.Create(ctx, virtualNode2)).Should(Succeed())
	Expect(cl.Create(ctx, virtualNode3)).Should(Succeed())

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

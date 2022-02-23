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

package namespaceoffloadingctrl

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
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

	virtualNode1Name = "liqo-6a0e9f-b52-4ed0"
	virtualNode2Name = "liqo-899890-dsd-323s"
	virtualNode3Name = "liqo-refc453-ds43d-43rs"

	providerLabel = "provider/liqo.io"
	regionLabel   = "region/liqo.io"
	regionA       = "A"
	regionB       = "B"
	providerAWS   = "AWS"
	providerGKE   = "GKE"
)

var (
	localCluster = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "0-789o-uhibi-oioi",
		ClusterName: "local-cluster-name",
	}
	remoteCluster1 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "1-6a0e9f-b52-4ed0",
		ClusterName: "remote-cluster-1",
	}
	remoteCluster2 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "2-899890-dsd-323s",
		ClusterName: "remote-cluster-2",
	}
	remoteCluster3 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "3-refc453-ds43d-43rs",
		ClusterName: "remote-cluster-3",
	}

	homeCfg        *rest.Config
	homeClient     client.Client
	homeClusterEnv *envtest.Environment

	// Resources.
	nms          *vkv1alpha1.NamespaceMapList
	virtualNode1 *corev1.Node
	virtualNode2 *corev1.Node
	virtualNode3 *corev1.Node

	nm1 *vkv1alpha1.NamespaceMap
	nm2 *vkv1alpha1.NamespaceMap
	nm3 *vkv1alpha1.NamespaceMap
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NamespaceOffloadingController Suite")
}

var _ = BeforeSuite(func() {
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

	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(homeCfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	homeClient = k8sManager.GetClient()
	Expect(homeClient).ToNot(BeNil())

	err = (&NamespaceOffloadingReconciler{
		Client:       homeClient,
		Scheme:       k8sManager.GetScheme(),
		LocalCluster: localCluster,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	// Necessary resources in HomeCluster
	virtualNode1 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode1Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:       liqoconst.TypeNode,
				liqoconst.RemoteClusterID: remoteCluster1.ClusterID,
				regionLabel:               regionA,
				providerLabel:             providerAWS,
			},
		},
	}

	virtualNode2 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode2Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:       liqoconst.TypeNode,
				liqoconst.RemoteClusterID: remoteCluster2.ClusterID,
				regionLabel:               regionB,
				providerLabel:             providerGKE,
			},
		},
	}

	virtualNode3 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode3Name,
			Labels: map[string]string{
				liqoconst.TypeLabel:       liqoconst.TypeNode,
				liqoconst.RemoteClusterID: remoteCluster3.ClusterID,
				regionLabel:               regionA,
				providerLabel:             providerGKE,
			},
		},
	}

	nms = &vkv1alpha1.NamespaceMapList{}

	nm1 = ForgeNamespaceMap(remoteCluster1)
	nm2 = ForgeNamespaceMap(remoteCluster2)
	nm3 = ForgeNamespaceMap(remoteCluster3)

	Expect(homeClient.Create(context.TODO(), virtualNode1)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), virtualNode2)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), virtualNode3)).Should(Succeed())

	Expect(homeClient.Create(context.TODO(), nm1)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), nm2)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), nm3)).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := homeClusterEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

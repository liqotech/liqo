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

package offloadingstatuscontroller

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (

	// namespace where the NamespaceMaps are created.
	mapNamespaceName = "default"
	mapNumber        = 3
	namespace1Name   = "namespace1"

	remoteClusterID1 = "remote-cluster-1"
	remoteClusterID2 = "remote-cluster-2"
	remoteClusterID3 = "remote-cluster-3"
)

var (
	homeCfg        *rest.Config
	homeClient     client.Client
	homeClusterEnv *envtest.Environment

	// Resources.
	nms                  *vkv1alpha1.NamespaceMapList
	namespace1           *corev1.Namespace
	namespaceOffloading1 *offv1alpha1.NamespaceOffloading

	nm1 *vkv1alpha1.NamespaceMap
	nm2 *vkv1alpha1.NamespaceMap
	nm3 *vkv1alpha1.NamespaceMap
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OffloadingStatusController Suite")
}

var _ = BeforeSuite(func() {
	ForgeNamespaceMap := func(clusterID string) *vkv1alpha1.NamespaceMap {
		return &vkv1alpha1.NamespaceMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterID,
				Namespace: mapNamespaceName,
				Labels: map[string]string{
					liqoconst.ReplicationRequestedLabel:   "true",
					liqoconst.RemoteClusterID:             clusterID,
					liqoconst.ReplicationDestinationLabel: clusterID,
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

	err = (&OffloadingStatusReconciler{
		Client:      homeClient,
		Scheme:      k8sManager.GetScheme(),
		RequeueTime: time.Second * 3,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	// Necessary resources in HomeCluster
	nms = &vkv1alpha1.NamespaceMapList{}

	nm1 = ForgeNamespaceMap(remoteClusterID1)
	nm2 = ForgeNamespaceMap(remoteClusterID2)
	nm3 = ForgeNamespaceMap(remoteClusterID3)

	namespace1 = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace1Name,
		},
	}

	namespaceOffloading1 = &offv1alpha1.NamespaceOffloading{
		ObjectMeta: metav1.ObjectMeta{
			Name:      liqoconst.DefaultNamespaceOffloadingName,
			Namespace: namespace1Name,
		},
		Spec: offv1alpha1.NamespaceOffloadingSpec{
			NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
			PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
			ClusterSelector:          corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
		},
	}

	Expect(homeClient.Create(context.TODO(), namespace1)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), nm1)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), nm2)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), nm3)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), namespaceOffloading1)).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := homeClusterEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

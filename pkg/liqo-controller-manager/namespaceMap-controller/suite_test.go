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

package namespacemapctrl

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
)

const (
	// Namespace where the NamespaceMaps are created.
	mapNamespaceName = "default"
)

var (
	remoteCluster1 = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "899890-dsd-323s",
		ClusterName: "remote-cluster-1",
	}
	localCluster = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "478374-dsa-432dd",
		ClusterName: "home-cluster",
	}

	homeCfg    *rest.Config
	remote1Cfg *rest.Config
	remote2Cfg *rest.Config

	homeClient    client.Client
	remoteClient1 kubernetes.Interface
	remoteClient2 kubernetes.Interface

	homeClusterEnv    *envtest.Environment
	remoteCluster1Env *envtest.Environment
	remoteCluster2Env *envtest.Environment

	nms *mapsv1alpha1.NamespaceMapList
	nm1 *mapsv1alpha1.NamespaceMap
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {

	By("bootstrapping test environments")

	homeClusterEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
	}

	remoteCluster1Env = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
	}

	remoteCluster2Env = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
	}

	var err error

	// Home cluster
	homeCfg, err = homeClusterEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(homeCfg).ToNot(BeNil())

	remote1Cfg, err = remoteCluster1Env.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(remote1Cfg).ToNot(BeNil())

	remote2Cfg, err = remoteCluster2Env.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(remote2Cfg).ToNot(BeNil())

	Expect(corev1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(discoveryv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(mapsv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(offv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(homeCfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	remoteClient1, err = kubernetes.NewForConfig(remote1Cfg)
	Expect(remoteClient1).ToNot(BeNil())
	Expect(err).NotTo(HaveOccurred())

	remoteClient2, err = kubernetes.NewForConfig(remote2Cfg)
	Expect(remoteClient2).ToNot(BeNil())
	Expect(err).NotTo(HaveOccurred())

	homeClient = k8sManager.GetClient()
	Expect(homeClient).ToNot(BeNil())

	controllerClients := map[string]kubernetes.Interface{
		remoteCluster1.ClusterID: remoteClient1,
	}

	// Necessary resources in HomeCluster
	fc1 := discoveryv1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: remoteCluster1.ClusterName,
			Labels: map[string]string{
				discovery.ClusterIDLabel: remoteCluster1.ClusterID,
			},
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID:   remoteCluster1.ClusterID,
				ClusterName: remoteCluster1.ClusterName,
			},
			OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			ForeignAuthURL:         "https://example.com",
			InsecureSkipTLSVerify:  pointer.BoolPtr(true),
		},
	}
	Expect(homeClient.Create(context.Background(), &fc1)).To(Succeed())

	nms = &mapsv1alpha1.NamespaceMapList{}

	nm1 = &mapsv1alpha1.NamespaceMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", remoteCluster1.ClusterID),
			Namespace:    mapNamespaceName,
			Labels: map[string]string{
				liqoconst.RemoteClusterID: remoteCluster1.ClusterID,
			},
		},
	}
	Expect(homeClient.Create(context.TODO(), nm1)).Should(Succeed())

	err = (&NamespaceMapReconciler{
		Client:        homeClient,
		RemoteClients: controllerClients,
		LocalCluster:  localCluster,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := homeClusterEnv.Stop()
	Expect(err).ToNot(HaveOccurred())

	err = remoteCluster1Env.Stop()
	Expect(err).ToNot(HaveOccurred())

	err = remoteCluster2Env.Stop()
	Expect(err).ToNot(HaveOccurred())
})

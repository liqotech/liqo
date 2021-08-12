/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
   http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	// Namespace where the NamespaceMaps are created.
	mapNamespaceName = "default"
	remoteClusterID1 = "899890-dsd-323s"
	localClusterID   = "478374-dsa-432dd"
)

var (
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

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = mapsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = offv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

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
		remoteClusterID1: remoteClient1,
	}

	// Necessary resources in HomeCluster
	nms = &mapsv1alpha1.NamespaceMapList{}

	nm1 = &mapsv1alpha1.NamespaceMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", remoteClusterID1),
			Namespace:    mapNamespaceName,
			Labels: map[string]string{
				liqoconst.RemoteClusterID: remoteClusterID1,
			},
		},
	}
	Expect(homeClient.Create(context.TODO(), nm1)).Should(Succeed())

	err = (&NamespaceMapReconciler{
		Client:         homeClient,
		RemoteClients:  controllerClients,
		LocalClusterID: localClusterID,
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

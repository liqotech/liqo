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

package namespaceoffloadingctrl

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (

	// namespace where the NamespaceMaps are created
	mapNamespaceName = "default"
	mapNumber        = 3

	virtualNode1Name = "liqo-6a0e9f-b52-4ed0"
	virtualNode2Name = "liqo-899890-dsd-323s"
	virtualNode3Name = "liqo-refc453-ds43d-43rs"

	localClusterId   = "0-789o-uhibi-oioi"
	remoteClusterId1 = "1-6a0e9f-b52-4ed0"
	remoteClusterId2 = "2-899890-dsd-323s"
	remoteClusterId3 = "3-refc453-ds43d-43rs"

	providerLabel = "provider/liqo.io"
	regionLabel   = "region/liqo.io"
	regionA       = "A"
	regionB       = "B"
	providerAWS   = "AWS"
	providerGKE   = "GKE"
)

var (
	homeCfg        *rest.Config
	homeClient     client.Client
	homeClusterEnv *envtest.Environment

	// Resources
	nms          *mapsv1alpha1.NamespaceMapList
	virtualNode1 *corev1.Node
	virtualNode2 *corev1.Node
	virtualNode3 *corev1.Node

	nm1 *mapsv1alpha1.NamespaceMap
	nm2 *mapsv1alpha1.NamespaceMap
	nm3 *mapsv1alpha1.NamespaceMap

	flags *flag.FlagSet
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

	flags = &flag.FlagSet{}
	klog.InitFlags(flags)
	_ = flags.Set("v", "2")

	var err error

	// Home cluster
	homeCfg, err = homeClusterEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(homeCfg).ToNot(BeNil())

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

	homeClient = k8sManager.GetClient()
	Expect(homeClient).ToNot(BeNil())

	err = (&NamespaceOffloadingReconciler{
		Client:         homeClient,
		Scheme:         k8sManager.GetScheme(),
		LocalClusterID: localClusterId,
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
			Annotations: map[string]string{
				liqoconst.RemoteClusterID: remoteClusterId1,
			},
			Labels: map[string]string{
				liqoconst.TypeLabel: liqoconst.TypeNode,
				regionLabel:         regionA,
				providerLabel:       providerAWS,
			},
		},
	}

	virtualNode2 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode2Name,
			Annotations: map[string]string{
				liqoconst.RemoteClusterID: remoteClusterId2,
			},
			Labels: map[string]string{
				liqoconst.TypeLabel: liqoconst.TypeNode,
				regionLabel:         regionB,
				providerLabel:       providerGKE,
			},
		},
	}

	virtualNode3 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: virtualNode3Name,
			Annotations: map[string]string{
				liqoconst.RemoteClusterID: remoteClusterId3,
			},
			Labels: map[string]string{
				liqoconst.TypeLabel: liqoconst.TypeNode,
				regionLabel:         regionA,
				providerLabel:       providerGKE,
			},
		},
	}

	nms = &mapsv1alpha1.NamespaceMapList{}

	nm1 = &mapsv1alpha1.NamespaceMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", remoteClusterId1),
			Namespace:    mapNamespaceName,
			Labels: map[string]string{
				liqoconst.RemoteClusterID: remoteClusterId1,
			},
		},
	}

	nm2 = &mapsv1alpha1.NamespaceMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", remoteClusterId2),
			Namespace:    mapNamespaceName,
			Labels: map[string]string{
				liqoconst.RemoteClusterID: remoteClusterId2,
			},
		},
	}

	nm3 = &mapsv1alpha1.NamespaceMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", remoteClusterId3),
			Namespace:    mapNamespaceName,
			Labels: map[string]string{
				liqoconst.RemoteClusterID: remoteClusterId3,
			},
		},
	}

	Expect(homeClient.Create(context.TODO(), virtualNode1)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), virtualNode2)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), virtualNode3)).Should(Succeed())

	Expect(homeClient.Create(context.TODO(), nm1)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), nm2)).Should(Succeed())
	Expect(homeClient.Create(context.TODO(), nm3)).Should(Succeed())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := homeClusterEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

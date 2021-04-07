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

package virtualNode_controller

import (
	namespaceresourcesv1 "github.com/liqotech/liqo/apis/virtualKubelet/v1"
	"bytes"
	"context"
	"flag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	// +kubebuilder:scaffold:imports
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
	remoteClusterId1          = "6a0e9f-b52-4ed0"
	remoteClusterId2          = "899890-dsd-323"
	remoteClusterIdSimpleNode = "909030-sd-3231"
)

var (
	ctx          context.Context
	virtualNode1 *corev1.Node
	virtualNode2 *corev1.Node
	simpleNode   *corev1.Node
	flags        *flag.FlagSet
	buffer       *bytes.Buffer
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {

	By("bootstrapping test environment")
	//useCluster := true
	testEnv = &envtest.Environment{
		//UseExistingCluster: &useCluster,
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
		//AttachControlPlaneOutput: true,
	}

	buffer = &bytes.Buffer{}
	flags = &flag.FlagSet{}
	klog.InitFlags(flags)
	_ = flags.Set("v", "2")
	_ = flags.Set("logtostderr", "false")
	klog.SetOutput(buffer)
	buffer.Reset()

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = namespaceresourcesv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&VirtualNodeReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	virtualNode1 = &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nameVirtualNode1,
			Annotations: map[string]string{
				"cluster-id": remoteClusterId1,
			},
			Labels: map[string]string{
				"type":                         "virtual-node",
				"offloading.liqo.io/cluster-1": "",
				"offloading.liqo.io/AWS":       "",
			},
		},
	}

	virtualNode2 = &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nameVirtualNode2,
			Annotations: map[string]string{
				"cluster-id": remoteClusterId2,
			},
			Labels: map[string]string{
				"type":                         "virtual-node",
				"offloading.liqo.io/cluster-2": "",
				"offloading.liqo.io/GKE":       "",
			},
		},
	}

	simpleNode = &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nameSimpleNode,
			Annotations: map[string]string{
				"cluster-id": remoteClusterIdSimpleNode,
			},
			Labels: map[string]string{
				"offloading.liqo.io/cluster-1": "",
				"offloading.liqo.io/AWS":       "",
			},
		},
	}

	Expect(k8sClient.Create(context.TODO(), virtualNode1)).Should(Succeed())
	Expect(k8sClient.Create(context.TODO(), virtualNode2)).Should(Succeed())
	Expect(k8sClient.Create(context.TODO(), simpleNode)).Should(Succeed())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

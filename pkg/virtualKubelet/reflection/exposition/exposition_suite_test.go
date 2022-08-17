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

package exposition_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	LocalNamespace  = "local-namespace"
	RemoteNamespace = "remote-namespace"

	LocalClusterID    = "local-cluster-id"
	LocalClusterName  = "local-cluster-name"
	RemoteClusterID   = "remote-cluster-id"
	RemoteClusterName = "remote-cluster-name"

	LiqoNodeName = "local-node"
	LiqoNodeIP   = "1.1.1.1"
)

var (
	testEnv envtest.Environment
	client  kubernetes.Interface

	ctx    context.Context
	cancel context.CancelFunc
)

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exposition Reflection Suite")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()

	ctx := context.Background()

	testEnv = envtest.Environment{}
	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	// Need to use a real client, as server side apply seems not to be currently supported by the fake one.
	client = kubernetes.NewForConfigOrDie(cfg)
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: LocalNamespace}}, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: RemoteNamespace}}, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())

	local := discoveryv1alpha1.ClusterIdentity{ClusterID: LocalClusterID, ClusterName: LocalClusterName}
	remote := discoveryv1alpha1.ClusterIdentity{ClusterID: RemoteClusterID, ClusterName: RemoteClusterName}
	forge.Init(local, remote, LiqoNodeName, LiqoNodeIP)
})

var _ = BeforeEach(func() { ctx, cancel = context.WithCancel(context.Background()) })
var _ = AfterEach(func() { cancel() })

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})

var FakeEventHandler = func(options.Keyer) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) {},
		UpdateFunc: func(_, obj interface{}) {},
		DeleteFunc: func(_ interface{}) {},
	}
}

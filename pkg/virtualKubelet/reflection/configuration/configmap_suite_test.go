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

package configuration_test

import (
	"context"
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	LocalNamespace  = "local-namespace"
	RemoteNamespace = "remote-namespace"

	LocalClusterID  = "local-cluster"
	RemoteClusterID = "remote-cluster"
)

var (
	testEnv envtest.Environment
	client  kubernetes.Interface

	ctx    context.Context
	cancel context.CancelFunc
)

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configuration Reflection Suite")
}

var _ = BeforeSuite(func() {
	klog.SetOutput(GinkgoWriter)
	flagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(flagset)
	Expect(flagset.Set("v", "4")).To(Succeed())
	klog.LogToStderr(false)

	ctx, cancel = context.WithCancel(context.Background())

	testEnv = envtest.Environment{}
	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	// Need to use a real client, as server side apply seems not to be currently supported by the fake one.
	client = kubernetes.NewForConfigOrDie(cfg)
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: LocalNamespace}}, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: RemoteNamespace}}, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())

	forge.LocalClusterID = LocalClusterID
	forge.RemoteClusterID = RemoteClusterID
	forge.LiqoNodeName = func() string { return "liqo-node" }
})

var _ = AfterSuite(func() {
	cancel()
	Expect(testEnv.Stop()).To(Succeed())
})

var FakeEventHandler = func(options.Keyer) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) {},
		UpdateFunc: func(_, obj interface{}) {},
		DeleteFunc: func(_ interface{}) {},
	}
}

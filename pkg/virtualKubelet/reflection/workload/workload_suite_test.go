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

package workload_test

import (
	"context"
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	vkalpha1scheme "github.com/liqotech/liqo/pkg/client/clientset/versioned/scheme"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	LocalNamespace  = "local-namespace"
	RemoteNamespace = "remote-namespace"

	LocalClusterID  = "local-cluster"
	RemoteClusterID = "remote-cluster"

	LiqoNodeName = "local-node"
	LiqoNodeIP   = "1.1.1.1"
)

var (
	ctx    context.Context
	cancel context.CancelFunc
)

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Workload Reflection Suite")
}

var _ = BeforeSuite(func() {
	utilruntime.Must(vkalpha1scheme.AddToScheme(scheme.Scheme))

	klog.SetOutput(GinkgoWriter)
	flagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(flagset)
	Expect(flagset.Set("v", "4")).To(Succeed())
	klog.LogToStderr(false)

	forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP)
})

var _ = BeforeEach(func() { ctx, cancel = context.WithCancel(context.Background()) })
var _ = AfterEach(func() { cancel() })

var FakeEventHandler = func(options.Keyer) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) {},
		UpdateFunc: func(_, obj interface{}) {},
		DeleteFunc: func(_ interface{}) {},
	}
}

func GetPod(client kubernetes.Interface, namespace, name string) *corev1.Pod {
	pod, errpod := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
	return pod
}

func GetPodError(client kubernetes.Interface, namespace, name string) error {
	_, errpod := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	return errpod
}

func GetShadowPod(client liqoclient.Interface, namespace, name string) *vkv1alpha1.ShadowPod {
	pod, errpod := client.VirtualkubeletV1alpha1().ShadowPods(namespace).Get(ctx, name, metav1.GetOptions{})
	ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
	return pod
}

func GetShadowPodError(client liqoclient.Interface, namespace, name string) error {
	_, errpod := client.VirtualkubeletV1alpha1().ShadowPods(namespace).Get(ctx, name, metav1.GetOptions{})
	return errpod
}

func CreatePod(client kubernetes.Interface, pod *corev1.Pod) *corev1.Pod {
	pod, errpod := client.CoreV1().Pods(pod.GetNamespace()).Create(ctx, pod, metav1.CreateOptions{})
	ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
	return pod
}

func CreateShadowPod(client liqoclient.Interface, pod *vkv1alpha1.ShadowPod) *vkv1alpha1.ShadowPod {
	pod, errpod := client.VirtualkubeletV1alpha1().ShadowPods(pod.GetNamespace()).Create(ctx, pod, metav1.CreateOptions{})
	ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
	return pod
}

func UpdatePod(client kubernetes.Interface, pod *corev1.Pod) *corev1.Pod {
	pod, errpod := client.CoreV1().Pods(pod.GetNamespace()).Update(ctx, pod, metav1.UpdateOptions{})
	ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
	return pod
}

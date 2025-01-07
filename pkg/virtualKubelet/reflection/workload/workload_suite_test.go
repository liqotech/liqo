// Copyright 2019-2025 The Liqo Authors
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
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	vkalpha1scheme "github.com/liqotech/liqo/pkg/client/clientset/versioned/scheme"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	LocalNamespace  = "local-namespace"
	RemoteNamespace = "remote-namespace"

	LocalClusterID  = "local-cluster-id"
	RemoteClusterID = "remote-cluster-id"

	LiqoNodeName = "local-node"
	LiqoNodeIP   = "1.1.1.1"
)

var (
	ctx    context.Context
	cancel context.CancelFunc

	fakeAPIServerRemapping = func(ip string) func(ctx context.Context) (string, error) {
		return func(ctx context.Context) (string, error) {
			return ip, nil
		}
	}
)

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Workload Reflection Suite")
}

var _ = BeforeSuite(func() {
	utilruntime.Must(vkalpha1scheme.AddToScheme(scheme.Scheme))

	testutil.LogsToGinkgoWriter()

	Expect(os.Setenv("KUBERNETES_SERVICE_PORT", "8443")).To(Succeed())
	forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP)
})

var _ = BeforeEach(func() {
	Expect(os.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")).To(Succeed())
	ctx, cancel = context.WithCancel(context.Background())
})
var _ = AfterEach(func() { cancel() })

var FakeEventHandler = func(options.Keyer, ...options.EventFilter) cache.ResourceEventHandler {
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

func GetShadowPod(client liqoclient.Interface, namespace, name string) *offloadingv1beta1.ShadowPod {
	pod, errpod := client.OffloadingV1beta1().ShadowPods(namespace).Get(ctx, name, metav1.GetOptions{})
	ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
	return pod
}

func GetShadowPodError(client liqoclient.Interface, namespace, name string) error {
	_, errpod := client.OffloadingV1beta1().ShadowPods(namespace).Get(ctx, name, metav1.GetOptions{})
	return errpod
}

func CreatePod(client kubernetes.Interface, pod *corev1.Pod) *corev1.Pod {
	pod, errpod := client.CoreV1().Pods(pod.GetNamespace()).Create(ctx, pod, metav1.CreateOptions{})
	ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
	return pod
}

func CreateShadowPod(client liqoclient.Interface, pod *offloadingv1beta1.ShadowPod) *offloadingv1beta1.ShadowPod {
	pod, errpod := client.OffloadingV1beta1().ShadowPods(pod.GetNamespace()).Create(ctx, pod, metav1.CreateOptions{})
	ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
	return pod
}

func UpdatePod(client kubernetes.Interface, pod *corev1.Pod) *corev1.Pod {
	pod, errpod := client.CoreV1().Pods(pod.GetNamespace()).Update(ctx, pod, metav1.UpdateOptions{})
	ExpectWithOffset(1, errpod).ToNot(HaveOccurred())
	return pod
}

func CreateServiceAccountSecret(client kubernetes.Interface, namespace, name, saName string) *corev1.Secret {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: namespace, Labels: map[string]string{corev1.ServiceAccountNameKey: saName}}}
	secret, errsecret := client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	ExpectWithOffset(1, errsecret).ToNot(HaveOccurred())
	return secret
}

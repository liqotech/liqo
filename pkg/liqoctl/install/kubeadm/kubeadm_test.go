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

package kubeadm

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

var (
	ctx       = context.Background()
	generator = func(suffix string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-controller-manager-" + suffix,
				Namespace: "kube-system",
				Labels:    kubeControllerManagerLabels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "kube-controller-manager",
						Image: "k8s.gcr.io/kube-controller-manager:v1.20.1",
						Command: []string{
							"kube-controller-manager",
							"--allocate-node-cidrs=true",
							"--authentication-kubeconfig=/etc/kubernetes/controller-manager.conf",
							"--authorization-kubeconfig=/etc/kubernetes/controller-manager.conf",
							"--bind-address=127.0.0.1",
							"--client-ca-file=/etc/kubernetes/pki/ca.crt",
							"--cluster-cidr=10.244.0.0/16",
							"--cluster-name=kubernetes",
							"--cluster-signing-cert-file=/etc/kubernetes/pki/ca.crt",
							"--cluster-signing-key-file=/etc/kubernetes/pki/ca.key",
							"--controllers=*,bootstrapsigner,tokencleaner",
							"--kubeconfig=/etc/kubernetes/controller-manager.conf",
							"--leader-elect=true",
							"--port=0",
							"--requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt",
							"--root-ca-file=/etc/kubernetes/pki/ca.crt",
							"--service-account-private-key-file=/etc/kubernetes/pki/sa.key",
							"--service-cluster-ip-range=10.96.0.0/12",
							"--use-service-account-credentials=true",
						},
					},
				},
			},
		}
	}
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Parameters Fetching")
}

var _ = Describe("Extract elements from apiServer", func() {
	var (
		options Options
		pods    []runtime.Object
	)

	JustBeforeEach(func() {
		options = Options{Options: &install.Options{
			CommonOptions: &install.CommonOptions{
				Factory: &factory.Factory{
					KubeClient: fake.NewSimpleClientset(pods...),
					Printer:    output.NewFakePrinter(GinkgoWriter),
				},
			},
		}}
	})

	When("no kube-controller-manager pods is present", func() {
		BeforeEach(func() { pods = []runtime.Object{} })

		It("should return an error", func() {
			Expect(options.Initialize(ctx)).To(HaveOccurred())
		})
	})

	When("a single kube-controller-manager pod is present", func() {
		BeforeEach(func() { pods = []runtime.Object{generator("1")} })

		It("should retrieve the appropriate parameters", func() {
			Expect(options.Initialize(ctx)).ToNot(HaveOccurred())
			Expect(options.PodCIDR).To(Equal("10.244.0.0/16"))
			Expect(options.ServiceCIDR).To(Equal("10.96.0.0/12"))
		})
	})

	When("multiple kube-controller-manager pods are present", func() {
		BeforeEach(func() { pods = []runtime.Object{generator("1"), generator("2"), generator("3")} })

		It("should retrieve the appropriate parameters", func() {
			Expect(options.Initialize(ctx)).ToNot(HaveOccurred())
			Expect(options.PodCIDR).To(Equal("10.244.0.0/16"))
			Expect(options.ServiceCIDR).To(Equal("10.96.0.0/12"))
		})
	})

})

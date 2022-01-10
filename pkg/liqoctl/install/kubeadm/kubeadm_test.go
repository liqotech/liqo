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

package kubeadm

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	ctx = context.Background()
	p   = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-controller-manager-test",
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
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Parameters Fetching")
}

var _ = Describe("Extract elements from apiServer", func() {

	It("Retrieve parameters from kube-controller-manager pod", func() {

		c := fake.NewSimpleClientset(p)
		podCIDR, serviceCIDR, err := retrieveClusterParameters(ctx, c)
		Expect(err).ToNot(HaveOccurred())
		Expect(podCIDR).To(Equal("10.244.0.0/16"))
		Expect(serviceCIDR).To(Equal("10.96.0.0/12"))
	})
})

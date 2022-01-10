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

package foreigncluster

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/pkg/consts"
)

const liqoNamespace = "liqo"

var (
	ctx  context.Context
	node *corev1.Node
)

var _ = Describe("Test fetching Liqo Auth service name", func() {
	BeforeEach(func() {
		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node",
			},
			Spec: corev1.NodeSpec{},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "195.195.195.195",
					},
					{
						Type:    corev1.NodeHostName,
						Address: "the-node",
					},
				},
			},
		}
	})
	DescribeTable("Ensure correct resources are created or an exception is raised",
		func(svc *corev1.Service, expectedResult string) {
			client := fake.NewClientBuilder().WithRuntimeObjects(svc, node).Build()
			endpoint, err := GetHomeAuthURL(ctx, client, "", "", liqoNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoint).To(BeEquivalentTo(expectedResult))
		},
		Entry("NodePort Service", &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      consts.AuthServiceName,
				Namespace: liqoNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "https",
						Port:     443,
						NodePort: 32000,
					},
				},
				ClusterIP: "10.0.0.1",
				Type:      "NodePort",
			},
		}, "https://195.195.195.195:32000"),
		Entry("LoadBalancer Service with IP and Hostname", &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      consts.AuthServiceName,
				Namespace: liqoNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "https",
						Port:     443,
						NodePort: 32000,
					},
				},
				ClusterIP: "10.0.0.1",
				Type:      "LoadBalancer",
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							IP:       "193.193.193.193",
							Hostname: "svc-hostname.com",
						},
					},
				},
			},
		}, "https://svc-hostname.com"),
		Entry("LoadBalancer Service with hostname only", &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      consts.AuthServiceName,
				Namespace: liqoNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "https",
						Port:     443,
						NodePort: 32000,
					},
				},
				ClusterIP: "10.0.0.1",
				Type:      "LoadBalancer",
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							Hostname: "svc-hostname.com",
						},
					},
				},
			},
		}, "https://svc-hostname.com"))
})

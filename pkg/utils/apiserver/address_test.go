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

package apiserver_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/pkg/utils/apiserver"
)

var _ = Describe("Address", func() {

	type addressTestcase struct {
		node            *corev1.Node
		expectedAddress string
	}

	ForgeNode := func(label string) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "master-1",
				Labels: map[string]string{label: ""},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeExternalIP,
						Address: "1.2.3.4",
					},
				},
			},
		}
	}

	DescribeTable("Address table",
		func(c addressTestcase) {
			ctx := context.Background()
			cl := fake.NewFakeClient()

			err := cl.Create(ctx, c.node)
			Expect(err).To(Succeed())

			address, err := apiserver.GetAddressFromMasterNode(ctx, cl)
			Expect(err).To(Succeed())

			Expect(address).To(Equal(c.expectedAddress))

			Expect(cl.Delete(ctx, c.node)).To(Succeed())
		},

		Entry("master node", addressTestcase{
			node:            ForgeNode("node-role.kubernetes.io/master"),
			expectedAddress: "https://1.2.3.4:6443",
		}),

		Entry("control plane node", addressTestcase{
			node:            ForgeNode("node-role.kubernetes.io/control-plane"),
			expectedAddress: "https://1.2.3.4:6443",
		}),

		Entry("RKE control plane node", addressTestcase{
			node:            ForgeNode("node-role.kubernetes.io/controlplane"),
			expectedAddress: "https://1.2.3.4:6443",
		}),
	)
})

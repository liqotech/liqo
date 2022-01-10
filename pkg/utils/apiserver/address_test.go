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

package apiserver_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/liqotech/liqo/pkg/utils/apiserver"
)

var _ = Describe("Address", func() {

	type addressTestcase struct {
		node            *v1.Node
		expectedAddress string
	}

	DescribeTable("Address table",

		func(c addressTestcase) {
			ctx := context.Background()
			client := fake.NewSimpleClientset()

			node, err := client.CoreV1().Nodes().Create(ctx, c.node, metav1.CreateOptions{})
			Expect(err).To(Succeed())
			node.Status = *c.node.Status.DeepCopy()
			node, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			Expect(err).To(Succeed())

			address, err := apiserver.GetAddressFromMasterNode(ctx, client)
			Expect(err).To(Succeed())

			Expect(address).To(Equal(c.expectedAddress))

			Expect(client.CoreV1().Nodes().Delete(ctx, node.Name, metav1.DeleteOptions{})).To(Succeed())
		},

		Entry("master node", addressTestcase{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/master": "",
					},
				},
				Spec: v1.NodeSpec{},
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{
							Type:    v1.NodeExternalIP,
							Address: "1.2.3.4",
						},
					},
				},
			},
			expectedAddress: "https://1.2.3.4:6443",
		}),

		Entry("control plane node", addressTestcase{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
				Spec: v1.NodeSpec{},
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{
							Type:    v1.NodeExternalIP,
							Address: "1.2.3.4",
						},
					},
				},
			},
			expectedAddress: "https://1.2.3.4:6443",
		}),
	)
})

// Copyright 2019-2026 The Liqo Authors
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

package check

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("GetNodeAddress", func() {
	forgeNode := func(addrs ...corev1.NodeAddress) *corev1.Node {
		return &corev1.Node{Status: corev1.NodeStatus{Addresses: addrs}}
	}

	It("should return the IPv4 internal IP of an IPv6-primary dual-stack node", func() {
		node := forgeNode(
			corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "fd00::5"},
			corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.5"},
			corev1.NodeAddress{Type: corev1.NodeHostName, Address: "node-1"},
		)
		Expect(GetNodeAddress(node)).To(Equal("10.0.0.5"))
	})

	It("should return the IPv4 external IP, preferring the external type over the internal one", func() {
		node := forgeNode(
			corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.5"},
			corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "2001:db8::5"},
			corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "203.0.113.5"},
		)
		Expect(GetNodeAddress(node)).To(Equal("203.0.113.5"))
	})

	It("should keep returning the IPv4 address on IPv4 single-stack nodes", func() {
		node := forgeNode(corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.5"})
		Expect(GetNodeAddress(node)).To(Equal("10.0.0.5"))
	})

	It("should fall back to the first address of the type when no IPv4 address is present", func() {
		node := forgeNode(corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "fd00::5"})
		Expect(GetNodeAddress(node)).To(Equal("fd00::5"))
	})

	It("should return the first address of non-IP types", func() {
		node := forgeNode(corev1.NodeAddress{Type: corev1.NodeHostName, Address: "node-1"})
		Expect(GetNodeAddress(node)).To(Equal("node-1"))
	})

	It("should return an empty string when the node has no address", func() {
		Expect(GetNodeAddress(forgeNode())).To(BeEmpty())
	})
})

var _ = Describe("nodeURL", func() {
	It("should wrap IPv6 addresses in square brackets", func() {
		Expect(nodeURL("10.0.0.5", 30080)).To(Equal("http://10.0.0.5:30080"))
		Expect(nodeURL("fd00::5", 30080)).To(Equal("http://[fd00::5]:30080"))
	})
})

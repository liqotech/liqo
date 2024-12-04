// Copyright 2019-2024 The Liqo Authors
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

package ipamcore

import (
	"fmt"
	"math"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
)

// node represents a node in the binary tree.
type node struct {
	prefix   netip.Prefix
	acquired bool
	left     *node
	right    *node

	ips    []netip.Addr
	lastip netip.Addr
}

type nodeDirection string

const (
	leftDirection  nodeDirection = "left"
	rightDirection nodeDirection = "right"
)

func newNode(prefix netip.Prefix) node {
	return node{prefix: prefix}
}

func allocateNetwork(size int, node *node) *netip.Prefix {
	if node.acquired || node.prefix.Bits() > size {
		return nil
	}
	if node.prefix.Bits() == size {
		if !node.isSplitted() {
			node.acquired = true
			return &node.prefix
		}
		return nil
	}

	if !node.isSplitted() {
		node.split()
	}

	first, second := node.bestDirection()

	if prefix := allocateNetwork(size, node.next(first)); prefix != nil {
		return prefix
	}
	return allocateNetwork(size, node.next(second))
}

func allocateNetworkWithPrefix(prefix netip.Prefix, node *node) *netip.Prefix {
	if node.acquired || !node.prefix.Overlaps(prefix) {
		return nil
	}
	if node.prefix.Addr().Compare(prefix.Addr()) == 0 && node.prefix.Bits() == prefix.Bits() {
		if !node.acquired && node.left == nil && node.right == nil {
			node.acquired = true
			return &node.prefix
		}
		return nil
	}

	if !node.isSplitted() {
		node.split()
	}

	if node.left != nil && node.left.prefix.Overlaps(prefix) {
		return allocateNetworkWithPrefix(prefix, node.left)
	}
	if node.right != nil && node.right.prefix.Overlaps(prefix) {
		return allocateNetworkWithPrefix(prefix, node.right)
	}

	return nil
}

func networkRelease(prefix netip.Prefix, node *node) *netip.Prefix {
	var result *netip.Prefix

	if node == nil {
		return nil
	}

	if node.prefix.Addr().Compare(prefix.Addr()) == 0 && node.prefix.Bits() == prefix.Bits() {
		if node.acquired {
			node.acquired = false
			return &node.prefix
		}
		return nil
	}

	if node.left != nil && node.left.prefix.Overlaps(prefix) {
		result = networkRelease(prefix, node.left)
	}
	if node.right != nil && node.right.prefix.Overlaps(prefix) {
		result = networkRelease(prefix, node.right)
	}

	node.merge()
	return result
}

func networkIsAvailable(prefix netip.Prefix, node *node) bool {
	if node.prefix.Addr().Compare(prefix.Addr()) == 0 && node.prefix.Bits() == prefix.Bits() {
		if node.left != nil && node.left.left.isSplitted() {
			return false
		}
		if node.right != nil && node.right.isSplitted() {
			return false
		}

		// If node children are not splitted and node is not acquired, then network is available
		return !node.acquired
	}

	if node.left == nil && node.right == nil {
		return true
	}

	if node.left != nil && node.left.prefix.Overlaps(prefix) && !node.left.acquired {
		return networkIsAvailable(prefix, node.left)
	}
	if node.right != nil && node.right.prefix.Overlaps(prefix) && !node.right.acquired {
		return networkIsAvailable(prefix, node.right)
	}

	return false
}

func listNetworks(node *node) []netip.Prefix {
	if node == nil {
		return nil
	}

	if node.acquired {
		return []netip.Prefix{node.prefix}
	}

	var networks []netip.Prefix
	if node.left != nil {
		networks = append(networks, listNetworks(node.left)...)
	}
	if node.right != nil {
		networks = append(networks, listNetworks(node.right)...)
	}

	return networks
}

func (n *node) isAllocatedIP(ip netip.Addr) bool {
	for i := range n.ips {
		if n.ips[i].Compare(ip) == 0 {
			return true
		}
	}
	return false
}

func (n *node) ipAcquire() *netip.Addr {
	if !n.acquired {
		return nil
	}

	size := int(math.Pow(2, float64(n.prefix.Addr().BitLen()-n.prefix.Bits())))

	// If the lastip is not initialized, set it to the first address of the prefix.
	if !n.lastip.IsValid() {
		n.lastip = n.prefix.Addr()
	}

	addr := n.lastip

	if n.lastip.Compare(n.prefix.Addr()) != 0 {
		addr = addr.Next()
	}

	for i := 0; i < size; i++ {
		// we need to check if the address is contained in the prefix.
		// If it is not, we need to set it to the first address of the prefix to prevent overflow.
		if !n.prefix.Contains(addr) {
			addr = n.prefix.Addr()
		}
		if !n.isAllocatedIP(addr) {
			n.ips = append(n.ips, addr)
			n.lastip = addr
			return &addr
		}
		addr = addr.Next()
	}
	return nil
}

func (n *node) allocateIPWithAddr(addr netip.Addr) *netip.Addr {
	if !n.acquired {
		return nil
	}

	if !n.prefix.Contains(addr) {
		return nil
	}

	for i := range n.ips {
		if n.ips[i].Compare(addr) == 0 {
			return nil
		}
	}

	n.ips = append(n.ips, addr)

	return &n.ips[len(n.ips)-1]
}

func (n *node) ipRelease(ip netip.Addr) *netip.Addr {
	if !n.acquired {
		return nil
	}

	for i, addr := range n.ips {
		if addr.Compare(ip) == 0 {
			n.ips = append(n.ips[:i], n.ips[i+1:]...)
			return &addr
		}
	}
	return nil
}

func search(prefix netip.Prefix, node *node) *node {
	if node == nil {
		return nil
	}

	if node.prefix.Addr().Compare(prefix.Addr()) == 0 && node.prefix.Bits() == prefix.Bits() {
		return node
	}

	if node.left != nil && node.left.prefix.Overlaps(prefix) {
		return search(prefix, node.left)
	}
	if node.right != nil && node.right.prefix.Overlaps(prefix) {
		return search(prefix, node.right)
	}

	return nil
}

func (n *node) split() {
	if n.isSplitted() {
		return
	}
	left, right := splitNetworkPrefix(n.prefix)
	n.insert(leftDirection, left)
	n.insert(rightDirection, right)
}

func (n *node) merge() {
	if n.left.isLeaf() && n.right.isLeaf() && !n.left.acquired && !n.right.acquired {
		n.left = nil
		n.right = nil
	}
}

func (n *node) insert(nd nodeDirection, prefix netip.Prefix) {
	newNode := &node{prefix: prefix}
	switch nd {
	case leftDirection:
		n.left = newNode
		return
	case rightDirection:
		n.right = newNode
		return
	default:
		return
	}
}

func (n *node) bestDirection() (first, second nodeDirection) {
	if n.left.isSplitted() {
		return leftDirection, rightDirection
	}
	if n.right.isSplitted() {
		return rightDirection, leftDirection
	}
	return leftDirection, rightDirection
}

func (n *node) isSplitted() bool {
	if n.left != nil && n.right != nil {
		return true
	}
	return false
}

func (n *node) isLeaf() bool {
	if n.left == nil && n.right == nil {
		return true
	}
	return false
}

func (n *node) next(direction nodeDirection) *node {
	switch direction {
	case leftDirection:
		return n.left
	case rightDirection:
		return n.right
	default:
		return nil
	}
}

func (n *node) toGraphviz() error {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	n.toGraphvizRecursive(&sb)
	sb.WriteString("}\n")

	if _, err := os.Stat("./graphviz"); os.IsNotExist(err) {
		if err := os.Mkdir("./graphviz", 0o700); err != nil {
			return err
		}
	}

	filePath := filepath.Clean("./graphviz/" + strings.NewReplacer("/", "_", ".", "_").Replace(n.prefix.String()) + ".dot")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(sb.String())
	return err
}

func (n *node) toGraphvizRecursive(sb *strings.Builder) {
	if n == nil {
		return
	}
	label := n.prefix.String()
	if len(n.ips) > 0 {
		ipsString := []string{}
		for i := range n.ips {
			ipsString = append(ipsString, n.ips[i].String())
		}
		label += "\\n" + strings.Join(ipsString, "\\n")
	}
	if n.acquired {
		fmt.Fprintf(sb, "  %q [label=\"%s\", style=filled, color=\"#57cc99\"];\n", n.prefix, label)
	}
	if n.left != nil {
		fmt.Fprintf(sb, "  %q -> %q;\n", n.prefix, n.left.prefix)
		n.left.toGraphvizRecursive(sb)
	}
	if n.right != nil {
		fmt.Fprintf(sb, "  %q -> %q;\n", n.prefix, n.right.prefix)
		n.right.toGraphvizRecursive(sb)
	}
}

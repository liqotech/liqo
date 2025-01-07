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

package ipamcore

import (
	"fmt"
	"math"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// nodeIP represents an IP address acquired by a node.
type nodeIP struct {
	addr              netip.Addr
	creationTimestamp time.Time
}

// node represents a node in the binary tree.
type node struct {
	lastUpdateTimestamp time.Time

	prefix   netip.Prefix
	acquired bool
	left     *node
	right    *node

	ips    []nodeIP
	lastip netip.Addr
}

type nodeDirection string

const (
	leftDirection  nodeDirection = "left"
	rightDirection nodeDirection = "right"

	graphvizFolder = "./graphviz"
)

func newNode(prefix netip.Prefix) node {
	return node{prefix: prefix, lastUpdateTimestamp: time.Now()}
}

func allocateNetwork(size int, node *node) *netip.Prefix {
	if node.acquired || node.prefix.Bits() > size {
		return nil
	}
	if node.prefix.Bits() == size {
		if !node.isSplitted() {
			node.acquired = true
			node.lastUpdateTimestamp = time.Now()
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
	if node.acquired || !isPrefixChildOf(node.prefix, prefix) {
		return nil
	}
	if node.prefix.Addr().Compare(prefix.Addr()) == 0 && node.prefix.Bits() == prefix.Bits() {
		if !node.acquired && node.left == nil && node.right == nil {
			node.acquired = true
			node.lastUpdateTimestamp = time.Now()
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

	// This should never happen
	return nil
}

func networkRelease(prefix netip.Prefix, node *node, gracePeriod time.Duration) *netip.Prefix {
	var result *netip.Prefix

	if node.prefix.Addr().Compare(prefix.Addr()) == 0 && node.prefix.Bits() == prefix.Bits() &&
		node.lastUpdateTimestamp.Add(gracePeriod).Before(time.Now()) {
		if node.acquired {
			node.acquired = false
			node.lastUpdateTimestamp = time.Now()
			return &node.prefix
		}
		return nil
	}
	if node.left != nil && isPrefixChildOf(node.left.prefix, prefix) {
		result = networkRelease(prefix, node.left, gracePeriod)
	}
	if node.right != nil && isPrefixChildOf(node.right.prefix, prefix) {
		result = networkRelease(prefix, node.right, gracePeriod)
	}

	node.merge(gracePeriod)
	return result
}

func networkIsAvailable(prefix netip.Prefix, node *node) bool {
	if node.prefix.Addr().Compare(prefix.Addr()) == 0 && node.prefix.Bits() == prefix.Bits() {
		if node.left != nil && (node.left.isSplitted() || node.left.acquired) {
			return false
		}
		if node.right != nil && (node.right.isSplitted() || node.right.acquired) {
			return false
		}

		// If node children are not splitted and node is not acquired, then network is available
		return !node.acquired
	}

	if node.left == nil && node.right == nil {
		return !node.acquired
	}

	if node.left != nil && isPrefixChildOf(node.left.prefix, prefix) && !node.left.acquired {
		return networkIsAvailable(prefix, node.left)
	}
	if node.right != nil && isPrefixChildOf(node.right.prefix, prefix) && !node.right.acquired {
		return networkIsAvailable(prefix, node.right)
	}

	return false
}

func listNetworks(node *node) []netip.Prefix {
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
		if n.ips[i].addr.Compare(ip) == 0 {
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
			n.ips = append(n.ips, nodeIP{addr: addr, creationTimestamp: time.Now()})
			n.lastip = addr
			n.lastUpdateTimestamp = time.Now()
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
		if n.ips[i].addr.Compare(addr) == 0 {
			return nil
		}
	}

	n.ips = append(n.ips, nodeIP{addr: addr, creationTimestamp: time.Now()})
	n.lastUpdateTimestamp = time.Now()

	return &n.ips[len(n.ips)-1].addr
}

func (n *node) ipRelease(ip netip.Addr, gracePeriod time.Duration) *netip.Addr {
	if !n.acquired {
		return nil
	}

	for i, nodeIP := range n.ips {
		if !nodeIP.creationTimestamp.Add(gracePeriod).Before(time.Now()) {
			continue
		}
		if nodeIP.addr.Compare(ip) == 0 {
			n.ips = append(n.ips[:i], n.ips[i+1:]...)
			n.lastUpdateTimestamp = time.Now()
			return &nodeIP.addr
		}
	}
	return nil
}

func search(prefix netip.Prefix, node *node) *node {
	if node.prefix.Addr().Compare(prefix.Addr()) == 0 && node.prefix.Bits() == prefix.Bits() {
		return node
	}

	if node.left != nil && isPrefixChildOf(node.left.prefix, prefix) {
		return search(prefix, node.left)
	}
	if node.right != nil && isPrefixChildOf(node.right.prefix, prefix) {
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

func (n *node) merge(gracePeriod time.Duration) {
	if n.left == nil || n.right == nil {
		return
	}
	if !n.left.lastUpdateTimestamp.Add(gracePeriod).Before(time.Now()) || !n.right.lastUpdateTimestamp.Add(gracePeriod).Before(time.Now()) {
		return // grace period not expired
	}
	if !n.left.isLeaf() || !n.right.isLeaf() {
		return
	}
	if n.left.acquired || n.right.acquired {
		return
	}

	n.left = nil
	n.right = nil
	n.lastUpdateTimestamp = time.Now()
}

func (n *node) insert(nd nodeDirection, prefix netip.Prefix) {
	newNode := newNode(prefix)
	switch nd {
	case leftDirection:
		n.left = &newNode
		return
	case rightDirection:
		n.right = &newNode
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
	}
	return nil
}

func (n *node) toGraphviz() error {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	n.toGraphvizRecursive(&sb)
	sb.WriteString("}\n")

	if _, err := os.Stat(graphvizFolder + ""); os.IsNotExist(err) {
		if err := os.Mkdir(graphvizFolder+"", 0o700); err != nil {
			return err
		}
	}

	filePath := filepath.Clean(graphvizFolder + "/" + strings.NewReplacer("/", "_", ".", "_").Replace(n.prefix.String()) + ".dot")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(sb.String())
	return err
}

func (n *node) toGraphvizRecursive(sb *strings.Builder) {
	label := n.prefix.String()
	if len(n.ips) > 0 {
		ipsString := []string{}
		for i := range n.ips {
			ipsString = append(ipsString, n.ips[i].addr.String())
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

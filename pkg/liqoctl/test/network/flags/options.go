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

package flags

import (
	"fmt"
	"strings"

	"github.com/liqotech/liqo/pkg/liqoctl/test"
)

// NodePortNodes represents the type of nodes to target in NodePort tests.
type NodePortNodes string

const (
	// NodePortNodesAll represents the value to target all nodes.
	NodePortNodesAll NodePortNodes = "all"
	// NodePortNodesWorkers represents the value to target worker nodes.
	NodePortNodesWorkers NodePortNodes = "workers"
	// NodePortNodesControlPlanes represents the value to target control plane nodes.
	NodePortNodesControlPlanes NodePortNodes = "control-planes"
)

// NodePortNodesValues contains the possible values for NodePortNodes.
var NodePortNodesValues = []string{
	string(NodePortNodesAll),
	string(NodePortNodesWorkers),
	string(NodePortNodesControlPlanes),
}

// String returns the string representation of the NodePortNodes.
func (npn *NodePortNodes) String() string {
	return string(*npn)
}

// Set sets the NodePortNodes value.
func (npn *NodePortNodes) Set(s string) error {
	if s != "" && s != string(NodePortNodesAll) && s != string(NodePortNodesWorkers) && s != string(NodePortNodesControlPlanes) {
		return fmt.Errorf("valid values are %s", strings.Join(NodePortNodesValues, ","))
	}
	if s == "" {
		*npn = NodePortNodesAll
		return nil
	}
	*npn = NodePortNodes(s)
	return nil
}

// Type returns the enum type.
func (npn *NodePortNodes) Type() string {
	return "string"
}

// NewOptions returns a new Options struct.
func NewOptions(topts *test.Options) *Options {
	return &Options{
		Topts:         topts,
		NodePortNodes: NodePortNodesAll,
	}
}

// Options contains the options for the network tests.
type Options struct {
	Topts *test.Options

	RemoteKubeconfigs []string
	Info              bool
	RemoveNamespace   bool

	// Basic
	Basic bool
	// Enable curl from external to nodeport service
	NodePortExt bool
	// Select nodes type for NodePort tests.
	// It has 2 possible values:
	// all: curl from all nodes
	// workers: curl from worker nodes
	NodePortNodes NodePortNodes
	// Enable curl from external to loadbalancer service
	LoadBalancer bool
	// PodToNodePort
	PodToNodePort bool
	// IpRemapping
	IPRemapping bool
}

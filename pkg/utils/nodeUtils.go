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

package utils

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

var preferOrder = []corev1.NodeAddressType{
	corev1.NodeExternalDNS,
	corev1.NodeExternalIP,
	corev1.NodeInternalDNS,
	corev1.NodeInternalIP,
	corev1.NodeHostName,
}

// GetAddressFromNodeList returns an address from a Node pool.
func GetAddressFromNodeList(nodes []corev1.Node) (string, error) {
	for _, addrType := range preferOrder {
		for i := range nodes {
			addr, err := getAddressByType(&nodes[i], addrType)
			if err != nil {
				klog.V(4).Info(err.Error())
				continue
			}

			klog.V(4).Infof("found address %v with type %v", addr, addrType)
			return addr, nil
		}
	}
	return "", fmt.Errorf("no address found")
}

// GetAddress returns an address for a Node.
func GetAddress(node *corev1.Node) (string, error) {
	return GetAddressFromNodeList([]corev1.Node{
		*node,
	})
}

func getAddressByType(node *corev1.Node, addrType corev1.NodeAddressType) (string, error) {
	for _, addr := range node.Status.Addresses {
		if addr.Type == addrType {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("no address with type %v found in node %v", addrType, node.Name)
}

// IsNodeReady returns true if the passed node has the NodeReady condition = True, false otherwise.
func IsNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// IsVirtualNode returns true if the passed node is a virtual node, false otherwise.
func IsVirtualNode(node *corev1.Node) bool {
	nodeType, found := node.Labels[liqoconst.TypeLabel]
	return found && nodeType == liqoconst.TypeNode
}

// MergeNodeSelector merges two nodeSelectors.
// Every MatchExpression of the first one must be merged with all the MatchExpressions of the second one:
// n first MatchExpressions.
// m second MatchExpressions.
// m * n MergedNodeSelector MatchExpressions.
// For each term in the first selector, AND each term of the second selector:
// (A || B) && (C || D) -> (A && C) || (A && D) || (B && C) || (B && D).
func MergeNodeSelector(ns1, ns2 *corev1.NodeSelector) corev1.NodeSelector {
	mergedNodeSelector := corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}}
	for i := range ns1.NodeSelectorTerms {
		for j := range ns2.NodeSelectorTerms {
			newMatchExpression := ns1.NodeSelectorTerms[i].DeepCopy().MatchExpressions
			newMatchExpression = append(newMatchExpression, ns2.NodeSelectorTerms[j].MatchExpressions...)
			mergedNodeSelector.NodeSelectorTerms = append(mergedNodeSelector.NodeSelectorTerms, corev1.NodeSelectorTerm{
				MatchExpressions: newMatchExpression,
			})
		}
	}
	return mergedNodeSelector
}

// GetNodeClusterID returns the clusterID given a virtual node.
func GetNodeClusterID(selectedNode *corev1.Node) (string, bool) {
	remoteClusterID, ok := selectedNode.Labels[liqoconst.RemoteClusterID]
	if !ok || remoteClusterID == "" {
		return "", false
	}

	return remoteClusterID, true
}

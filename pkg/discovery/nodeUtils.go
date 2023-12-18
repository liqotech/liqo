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

package discovery

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

var preferOrder = []corev1.NodeAddressType{
	corev1.NodeExternalDNS,
	corev1.NodeExternalIP,
	corev1.NodeInternalDNS,
	corev1.NodeInternalIP,
	corev1.NodeHostName,
}

var preferOrderInternal = []corev1.NodeAddressType{
	corev1.NodeInternalIP,
	corev1.NodeExternalIP,
	corev1.NodeInternalDNS,
	corev1.NodeExternalDNS,
	corev1.NodeHostName,
}

// GetAddressFromNodeList returns an address from a Node pool.
func GetAddressFromNodeList(nodes []corev1.Node) (string, error) {
	for _, addrType := range preferOrder {
		for i := range nodes {
			addr, err := GetAddressByType(&nodes[i], addrType)
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

// GetInternalAddress returns an internal address for a Node.
func GetInternalAddress(node *corev1.Node) (string, error) {
	for _, addrType := range preferOrderInternal {
		addr, err := GetAddressByType(node, addrType)
		if err != nil {
			klog.V(4).Info(err.Error())
			continue
		}
		klog.V(4).Infof("found address %v with type %v", addr, addrType)
		return addr, nil
	}
	return "", fmt.Errorf("no internal address found")
}

// GetAddressByType returns an address of a specific type for a Node.
func GetAddressByType(node *corev1.Node, addrType corev1.NodeAddressType) (string, error) {
	for _, addr := range node.Status.Addresses {
		if addr.Type == addrType {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("no address with type %v found in node %v", addrType, node.Name)
}

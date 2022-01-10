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

package discovery

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

var preferOrder = []v1.NodeAddressType{
	v1.NodeExternalDNS,
	v1.NodeExternalIP,
	v1.NodeInternalDNS,
	v1.NodeInternalIP,
	v1.NodeHostName,
}

// GetAddressFromNodeList returns an address from a Node pool.
func GetAddressFromNodeList(nodes []v1.Node) (string, error) {
	for _, addrType := range preferOrder {
		for i := range nodes {
			if addr, err := getAddressByType(&nodes[i], addrType); err != nil {
				klog.V(4).Info(err.Error())
				continue
			} else {
				klog.V(4).Infof("found address %v with type %v", addr, addrType)
				return addr, nil
			}
		}
	}
	return "", fmt.Errorf("no address found")
}

// GetAddress returns an address for a Node.
func GetAddress(node *v1.Node) (string, error) {
	return GetAddressFromNodeList([]v1.Node{
		*node,
	})
}

func getAddressByType(node *v1.Node, addrType v1.NodeAddressType) (string, error) {
	for _, addr := range node.Status.Addresses {
		if addr.Type == addrType {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("no address with type %v found in node %v", addrType, node.Name)
}

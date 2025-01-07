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

package apiserver

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/utils"
)

// GetURL retrieves the API server URL either from the configuration or selecting the IP address of a master node (with port 6443).
func GetURL(ctx context.Context, cl client.Client, addressOverride string) (string, error) {
	if addressOverride != "" {
		if !strings.HasPrefix(addressOverride, "https://") {
			addressOverride = fmt.Sprintf("https://%v", addressOverride)
		}
		return addressOverride, nil
	}

	return GetAddressFromMasterNode(ctx, cl)
}

// GetAddressFromMasterNode returns the API Server address using the IP of the
// master node of this cluster. The port is always defaulted to 6443.
func GetAddressFromMasterNode(ctx context.Context, cl client.Client) (address string, err error) {
	nodes, err := getMasterNodes(ctx, cl)
	if err != nil {
		return "", err
	}
	host, err := utils.GetAddressFromNodeList(nodes.Items)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%v:6443", host), nil
}

func getMasterNodes(ctx context.Context, cl client.Client) (*v1.NodeList, error) {
	labelSelectors := []string{
		"node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/master",
		// Apparently used by RKE:
		// https://github.com/rancher/rke/blob/f3f7320a445d0f075f62781d14e71bef03cf5222/cluster/hosts.go#L23
		"node-role.kubernetes.io/controlplane",
	}

	var nodes v1.NodeList
	var err error
	for _, selector := range labelSelectors {
		if err = cl.List(ctx, &nodes, client.HasLabels{selector}); err != nil {
			return nil, err
		}
		if len(nodes.Items) != 0 {
			break
		}
	}

	if len(nodes.Items) == 0 {
		err = fmt.Errorf("no ApiServer.Address variable provided and no master node found, one of the two values must be present")
		return &nodes, err
	}
	return &nodes, nil
}

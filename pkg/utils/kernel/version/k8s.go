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

package version

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
)

// CheckKernelVersionFromNodes checks if the current kernel version satisfies the minimum requirements from a list of nodes.
func CheckKernelVersionFromNodes(ctx context.Context, cl client.Client, minimum *KernelVersion) error {
	current, err := GetLowerKernelVersionFromNodes(ctx, cl)
	if err != nil {
		return fmt.Errorf("failed to get the current kernel version: %w", err)
	}

	if !current.CheckRequirements(minimum) {
		return fmt.Errorf("kernel version %s does not satisfy the minimum requirements %s", current.String(), minimum.String())
	}
	return nil
}

// GetLowerKernelVersionFromNodes gets the lower kernel version from a list of nodes.
func GetLowerKernelVersionFromNodes(ctx context.Context, cl client.Client) (*KernelVersion, error) {
	nodes := corev1.NodeList{}
	if err := cl.List(ctx, &nodes); err != nil {
		return nil, err
	}

	var lowerKernelVersion *KernelVersion
	for i := range nodes.Items {
		node := nodes.Items[i]
		if node.Status.NodeInfo.KernelVersion == "" {
			continue
		}
		if l, ok := node.Labels[consts.TypeLabel]; ok && l == consts.VirtualNodeLabel {
			continue
		}
		nodeKernelVersion, err := GetKernelVersionFromNode(&node)
		if err != nil {
			return nil, err
		}

		if lowerKernelVersion == nil || Compare(nodeKernelVersion, lowerKernelVersion) < 0 {
			lowerKernelVersion = nodeKernelVersion
		}
	}

	return lowerKernelVersion, nil
}

// GetKernelVersionFromNode gets the current kernel version from a node.
func GetKernelVersionFromNode(node *corev1.Node) (*KernelVersion, error) {
	return ParseRelease(node.Status.NodeInfo.KernelVersion)
}

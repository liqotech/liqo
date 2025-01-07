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

package setup

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
)

// IsNodeControlPlane checks if the node is a control plane node.
func IsNodeControlPlane(taints []corev1.Taint) bool {
	for _, taint := range taints {
		if taint.Key == ControlPlaneTaintKey {
			return true
		}
	}
	return false
}

func getReplicas(ctx context.Context, cl ctrlclient.Client) (int32, error) {
	var replicas int32

	nodes := corev1.NodeList{}
	if err := cl.List(ctx, &nodes); err != nil {
		return 0, err
	}
	for i := range nodes.Items {
		if nodes.Items[i].Labels[consts.TypeLabel] == consts.TypeNode {
			continue
		}
		if IsNodeControlPlane(nodes.Items[i].Spec.Taints) {
			continue
		}
		replicas++
	}

	return replicas, nil
}

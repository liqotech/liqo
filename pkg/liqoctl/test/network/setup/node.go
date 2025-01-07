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

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
)

// AddConsumerNodeLabels adds the consumer node labels.
func AddConsumerNodeLabels(ctx context.Context, cl *client.Client) error {
	nodes := corev1.NodeList{}
	if err := cl.Consumer.List(ctx, &nodes); err != nil {
		return err
	}
	for i := range nodes.Items {
		if nodes.Items[i].Labels[consts.TypeLabel] == consts.TypeNode {
			continue
		}
		if IsNodeControlPlane(nodes.Items[i].Spec.Taints) {
			continue
		}
		nodes.Items[i].Labels[consts.RemoteClusterID] = cl.ConsumerName
		if err := cl.Consumer.Update(ctx, &nodes.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

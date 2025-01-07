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

package geneve

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// GetGeneveTunnelID returns the geneve tunnel id for the given internal fabric and node.
func GetGeneveTunnelID(ctx context.Context, cl client.Client,
	internalFabricName, internalNodeName string) (uint32, error) {
	list, err := getters.ListGeneveTunnelsByLabels(ctx, cl, labels.SelectorFromSet(
		labels.Set{
			consts.InternalFabricName: internalFabricName,
			consts.InternalNodeName:   internalNodeName,
		},
	))
	if err != nil {
		return 0, err
	}

	switch len(list.Items) {
	case 0:
		return 0, fmt.Errorf("no geneve tunnel found for internalfabric %s and internalnode %s",
			internalFabricName, internalNodeName)
	case 1:
		return list.Items[0].Spec.ID, nil
	default:
		return 0, fmt.Errorf("multiple geneve tunnels found for internalfabric %s and internalnode %s",
			internalFabricName, internalNodeName)
	}
}

// Copyright 2019-2026 The Liqo Authors
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

package gatewayapi

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// IsSharedGateway returns whether the given Gateway is offered as shared Gateway to the consumer clusters.
func IsSharedGateway(gateway metav1.Object) bool {
	switch gateway.GetLabels()[consts.SharedGatewayLabel] {
	case consts.SharedGatewayLabelValue, consts.SharedGatewayLabelDefaultValue:
		return true
	default:
		return false
	}
}

// ListSharedGateways returns the Gateways offered as shared Gateways to the consumer clusters, sorted by namespaced name.
func ListSharedGateways(ctx context.Context, cl client.Reader) ([]liqov1beta1.SharedGatewayType, error) {
	var gateways gwv1.GatewayList
	if err := cl.List(ctx, &gateways, client.HasLabels{consts.SharedGatewayLabel}); err != nil {
		return nil, fmt.Errorf("failed to list the shared Gateways: %w", err)
	}

	shared := make([]liqov1beta1.SharedGatewayType, 0, len(gateways.Items))
	for i := range gateways.Items {
		gateway := &gateways.Items[i]
		if !IsSharedGateway(gateway) {
			continue
		}
		shared = append(shared, liqov1beta1.SharedGatewayType{
			Namespace: gateway.GetNamespace(), Name: gateway.GetName(),
			Default: gateway.GetLabels()[consts.SharedGatewayLabel] == consts.SharedGatewayLabelDefaultValue,
		})
	}

	sort.Slice(shared, func(i, j int) bool {
		if shared[i].Namespace != shared[j].Namespace {
			return shared[i].Namespace < shared[j].Namespace
		}
		return shared[i].Name < shared[j].Name
	})
	return shared, nil
}

// DefaultSharedGateway returns the shared Gateway the routes reflected from the consumer clusters are attached to, that is,
// the one marked as default, or the first one (sorted by namespaced name) if none. It returns nil if no Gateway is shared.
func DefaultSharedGateway(shared []liqov1beta1.SharedGatewayType) *types.NamespacedName {
	if len(shared) == 0 {
		return nil
	}

	selected := shared[0]
	for i := range shared {
		if shared[i].Default {
			selected = shared[i]
			break
		}
	}
	return &types.NamespacedName{Namespace: selected.Namespace, Name: selected.Name}
}

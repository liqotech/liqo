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

package shadowendpointslicectrl

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamips "github.com/liqotech/liqo/pkg/utils/ipam/mapping"
)

// MapEndpointsWithConfiguration maps the endpoints of the shadowendpointslice.
func MapEndpointsWithConfiguration(ctx context.Context, cl client.Client,
	clusterID liqov1beta1.ClusterID, endpoints []discoveryv1.Endpoint) error {
	for i := range endpoints {
		for j := range endpoints[i].Addresses {
			addr := endpoints[i].Addresses[j]

			rAddr, err := ipamips.MapAddress(ctx, cl, clusterID, addr)
			if err != nil {
				return err
			}

			endpoints[i].Addresses[j] = rAddr
		}
	}

	return nil
}

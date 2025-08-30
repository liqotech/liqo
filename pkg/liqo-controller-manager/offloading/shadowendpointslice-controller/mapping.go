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
	klog "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	directconnectioninfo "github.com/liqotech/liqo/pkg/utils/directconnection"
	ipamips "github.com/liqotech/liqo/pkg/utils/ipam/mapping"
)

// MapEndpointsWithConfiguration maps the endpoints of the shadowendpointslice.
//
// The last parameter is needed to
func MapEndpointsWithConfiguration(ctx context.Context, cl client.Client,
	clusterID liqov1beta1.ClusterID, endpoints []discoveryv1.Endpoint,
	list directconnectioninfo.InfoList,
) error {
	for i := range endpoints {
		for j := range endpoints[i].Addresses {
			addr := endpoints[i].Addresses[j]

			addrHasBeenRemapped := false

			// If data is passed, check if mapping can be made manually
			if len(list.Items) != 0 {
				clusterID, ip, addressFound := list.GetConnectionDataByIP(addr)

				if addressFound {
					rAddr, err := ipamips.ForceMapAddressWithConfiguration(ctx, cl, liqov1beta1.ClusterID(clusterID), ip)

					if err == nil {
						endpoints[i].Addresses[j] = rAddr.String()
						addrHasBeenRemapped = true
					} else {
						klog.Errorf("error while mapping address %q using the cidr from clusterID %q: %v", ip, clusterID, err)
					}
				}
				if addrHasBeenRemapped {
					break
				}
			}

			// Regular mapping is performed
			if !addrHasBeenRemapped {
				rAddr, err := ipamips.MapAddress(ctx, cl, clusterID, addr)
				if err != nil {
					return err
				}
				endpoints[i].Addresses[j] = rAddr
			}
		}
	}

	return nil
}

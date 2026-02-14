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
	directconnectioninfo "github.com/liqotech/liqo/pkg/utils/directconnection"
	ipamips "github.com/liqotech/liqo/pkg/utils/ipam/mapping"

	klog "k8s.io/klog/v2"
)

// MapEndpointsWithConfiguration maps the endpoints of the shadowendpointslice.
// 
// It also maps the addresses using a different clusterID in case address is found in the DirectConnectionIndex.
func MapEndpointsWithConfiguration(ctx context.Context, cl client.Client,
	localClusterID liqov1beta1.ClusterID, endpoints []discoveryv1.Endpoint,
	index *directconnectioninfo.DirectConnectionIndex,
) error {
	for i := range endpoints {
		for j := range endpoints[i].Addresses {
			addr := endpoints[i].Addresses[j]

			mappingClusterID := localClusterID
			if index != nil {
				if directClusterID, found := index.LookupClusterID(addr); found {
					mappingClusterID = liqov1beta1.ClusterID(directClusterID)
				}
			}

			var rAddr string
			var err error
			// if address is found, then the remapping is forced using the clusterID instead of the local one.
			if mappingClusterID != localClusterID {
				rAddr, err = ipamips.ForceMapAddressWithConfiguration(ctx, cl, mappingClusterID, addr)
			} else{
				rAddr, err = ipamips.MapAddress(ctx, cl, localClusterID, addr)
			}
			klog.V(4).Infof("Mapped address %q using clusterID %q; result is: %q", addr, mappingClusterID, rAddr)
			if err != nil {
				return err
			}
			endpoints[i].Addresses[j] = rAddr
		}
	}

	return nil
}

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

package shadowendpointslicectrl

import (
	"context"
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	klog "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	directconnectioninfo "github.com/liqotech/liqo/pkg/utils/directconnection"
	ipamips "github.com/liqotech/liqo/pkg/utils/ipam/mapping"
)

// MapEndpointsWithConfiguration maps the endpoints of the shadowendpointslice through the
// Configuration of the cluster they are reached through: the local one, or, for the addresses found
// in the direct connection index, the one of the provider hosting them.
//
// A failure to remap a direct-connection address concerns that endpoint alone (for instance, an
// address outside the pod CIDRs the provider advertises): it is returned in failures, keyed by the
// endpoint position, and the other endpoints are mapped anyway, so that a single bad address cannot
// freeze the whole slice. A failure on the local Configuration concerns every endpoint alike, and is
// returned as an error.
func MapEndpointsWithConfiguration(ctx context.Context, cl client.Client,
	localClusterID liqov1beta1.ClusterID, endpoints []discoveryv1.Endpoint,
	index *directconnectioninfo.AddressIndex,
) (failures map[int]error, err error) {
	failures = make(map[int]error)
	for i := range endpoints {
		for j := range endpoints[i].Addresses {
			addr := endpoints[i].Addresses[j]

			if directClusterID, found := index.LookupClusterID(addr); found {
				// The address is remapped through the Configuration of the provider hosting it.
				rAddr, ferr := ipamips.ForceMapAddressWithConfiguration(ctx, cl, liqov1beta1.ClusterID(directClusterID), addr)
				if ferr != nil {
					failures[i] = fmt.Errorf("address %q of cluster %q: %w", addr, directClusterID, ferr)
					break
				}
				klog.V(4).Infof("Mapped address %q using clusterID %q; result is: %q", addr, directClusterID, rAddr)
				endpoints[i].Addresses[j] = rAddr
				continue
			}

			rAddr, err := ipamips.MapAddress(ctx, cl, localClusterID, addr)
			if err != nil {
				return nil, err
			}
			klog.V(4).Infof("Mapped address %q using clusterID %q; result is: %q", addr, localClusterID, rAddr)
			endpoints[i].Addresses[j] = rAddr
		}
	}

	return failures, nil
}

// MapOnlyDirectConnectionEndpoints remaps only the endpoint addresses that are found in the direct connection index.
//
// Unlike MapEndpointsWithConfiguration, it does not fall back to a local-cluster remapping for addresses
// not present in the index — those are left unchanged. This is intended for the case where networking
// between the consumer and the provider is disabled, but a direct provider-to-provider connection exists.
// As in MapEndpointsWithConfiguration, the endpoints whose address cannot be remapped are returned in
// failures, keyed by their position, without preventing the others from being mapped.
func MapOnlyDirectConnectionEndpoints(ctx context.Context, cl client.Client,
	endpoints []discoveryv1.Endpoint,
	index *directconnectioninfo.AddressIndex,
) (failures map[int]error) {
	failures = make(map[int]error)
	for i := range endpoints {
		for j := range endpoints[i].Addresses {
			addr := endpoints[i].Addresses[j]

			directClusterID, found := index.LookupClusterID(addr)
			if !found {
				// Address is not a direct-connection address; leave it unchanged.
				continue
			}

			rAddr, err := ipamips.ForceMapAddressWithConfiguration(ctx, cl, liqov1beta1.ClusterID(directClusterID), addr)
			if err != nil {
				failures[i] = fmt.Errorf("address %q of cluster %q: %w", addr, directClusterID, err)
				break
			}
			klog.V(4).Infof("Mapped direct-connection address %q using clusterID %q; result is: %q", addr, directClusterID, rAddr)
			endpoints[i].Addresses[j] = rAddr
		}
	}

	return failures
}

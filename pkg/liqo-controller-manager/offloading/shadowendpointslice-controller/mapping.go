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

// MapEndpointsWithConfiguration maps the endpoints of the shadowendpointslice.
//
// It also maps the addresses using a different clusterID in case address is found in the DirectConnectionIndex.
func MapEndpointsWithConfiguration(ctx context.Context, cl client.Client,
	localClusterID liqov1beta1.ClusterID, endpoints []discoveryv1.Endpoint,
	index *directconnectioninfo.AddressIndex,
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
			} else {
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

// FilterOutDirectConnectionEndpoints returns the subset of endpoints that do not reference any
// direct-connection address. An endpoint is dropped entirely if at least one of its addresses is
// found in the direct connection index, i.e. it refers to a pod hosted on another provider and
// reachable only through a direct provider-to-provider connection.
func FilterOutDirectConnectionEndpoints(endpoints []discoveryv1.Endpoint,
	index *directconnectioninfo.AddressIndex,
) []discoveryv1.Endpoint {
	if index == nil {
		return endpoints
	}

	filtered := make([]discoveryv1.Endpoint, 0, len(endpoints))
	for i := range endpoints {
		isDirect := false
		for _, addr := range endpoints[i].Addresses {
			if _, found := index.LookupClusterID(addr); found {
				isDirect = true
				break
			}
		}
		if isDirect {
			klog.V(4).Infof("Dropping endpoint with addresses %v: direct connections are denied", endpoints[i].Addresses)
			continue
		}
		filtered = append(filtered, endpoints[i])
	}

	return filtered
}

// MapOnlyDirectConnectionEndpoints remaps only the endpoint addresses that are found in the direct connection index.
//
// Unlike MapEndpointsWithConfiguration, it does not fall back to a local-cluster remapping for addresses
// not present in the index — those are left unchanged. This is intended for the case where networking
// between the consumer and the provider is disabled, but a direct provider-to-provider connection exists.
func MapOnlyDirectConnectionEndpoints(ctx context.Context, cl client.Client,
	endpoints []discoveryv1.Endpoint,
	index *directconnectioninfo.AddressIndex,
) error {
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
				return fmt.Errorf("error in forcing the mapping of address %v: %w", addr, err)
			}
			klog.V(4).Infof("Mapped direct-connection address %q using clusterID %q; result is: %q", addr, directClusterID, rAddr)
			endpoints[i].Addresses[j] = rAddr
		}
	}

	return nil
}

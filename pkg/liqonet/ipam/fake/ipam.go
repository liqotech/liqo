// Copyright 2019-2022 The Liqo Authors
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

package fake

import (
	"context"
	"fmt"

	grpc "google.golang.org/grpc"

	"github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

// IPAMClient provides a mock implementation of the IPAMClient interface for testing purposes.
type IPAMClient struct {
	localRemappedPodCIDR  string
	remoteRemappedPodCIDR string
	enforceSingleRequest  bool

	pods      map[string]string
	endpoints map[string]string
}

// NewIPAMClient returns a new fake IPAMClient.
func NewIPAMClient(localRemappedPodCIDR, remoteRemappedPodCIDR string, enforceSingleRequest bool) *IPAMClient {
	return &IPAMClient{
		localRemappedPodCIDR:  localRemappedPodCIDR,
		remoteRemappedPodCIDR: remoteRemappedPodCIDR,
		enforceSingleRequest:  true,

		pods:      make(map[string]string),
		endpoints: make(map[string]string),
	}
}

// MapEndpointIP mocks the corresponding IPAMClient function.
func (mock *IPAMClient) MapEndpointIP(_ context.Context, req *ipam.MapRequest, _ ...grpc.CallOption) (*ipam.MapResponse, error) {
	// Check first if the translation has already been computed.
	if translation, found := mock.endpoints[req.GetIp()]; found {
		if mock.enforceSingleRequest {
			return nil, fmt.Errorf("mapping for IP %v already requested", req.GetIp())
		}
		return &ipam.MapResponse{Ip: translation}, nil
	}

	ip, err := utils.MapIPToNetwork(mock.localRemappedPodCIDR, req.GetIp())
	if err != nil {
		return nil, err
	}
	mock.endpoints[req.GetIp()] = ip
	return &ipam.MapResponse{Ip: ip}, nil
}

// UnmapEndpointIP mocks the corresponding IPAMClient function.
func (mock *IPAMClient) UnmapEndpointIP(_ context.Context, req *ipam.UnmapRequest, _ ...grpc.CallOption) (*ipam.UnmapResponse, error) {
	// Check first if the translation has already been removed.
	if _, found := mock.endpoints[req.GetIp()]; !found && mock.enforceSingleRequest {
		return nil, fmt.Errorf("unmapping for IP %v already requested", req.GetIp())
	}
	delete(mock.endpoints, req.GetIp())
	return &ipam.UnmapResponse{}, nil
}

// IsEndpointTranslated returns whether the given endpoint has a valid translation.
func (mock *IPAMClient) IsEndpointTranslated(ip string) bool {
	_, found := mock.endpoints[ip]
	return found
}

// GetHomePodIP mocks the corresponding IPAMClient function.
func (mock *IPAMClient) GetHomePodIP(_ context.Context, req *ipam.GetHomePodIPRequest, _ ...grpc.CallOption) (*ipam.GetHomePodIPResponse, error) {
	// Check first if the translation has already been computed.
	if translation, found := mock.pods[req.GetIp()]; found {
		if mock.enforceSingleRequest {
			return nil, fmt.Errorf("mapping for IP %v already requested", req.GetIp())
		}
		return &ipam.GetHomePodIPResponse{HomeIP: translation}, nil
	}

	homeIP, err := utils.MapIPToNetwork(mock.remoteRemappedPodCIDR, req.GetIp())
	if err != nil {
		return nil, err
	}
	mock.pods[req.GetIp()] = homeIP
	return &ipam.GetHomePodIPResponse{HomeIP: homeIP}, nil
}

// BelongsToPodCIDR mocks the corresponding IPAMClient function.
func (mock *IPAMClient) BelongsToPodCIDR(context.Context, *ipam.BelongsRequest,
	...grpc.CallOption) (*ipam.BelongsResponse, error) {
	return &ipam.BelongsResponse{Belongs: true}, nil
}

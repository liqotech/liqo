// Copyright 2019-2021 The Liqo Authors
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

package test

import (
	"context"

	grpc "google.golang.org/grpc"

	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

// MockIpam mocks the IPAM module.
type MockIpam struct {
	LocalRemappedPodCIDR  string
	RemoteRemappedPodCIDR string
}

// MapEndpointIP mocks the corresponding func in IPAM.
func (mock *MockIpam) MapEndpointIP(
	ctx context.Context,
	in *liqonetIpam.MapRequest,
	opts ...grpc.CallOption) (*liqonetIpam.MapResponse, error) {
	oldIP := in.GetIp()
	newIP, err := utils.MapIPToNetwork(mock.LocalRemappedPodCIDR, oldIP)
	if err != nil {
		return &liqonetIpam.MapResponse{}, err
	}
	return &liqonetIpam.MapResponse{Ip: newIP}, nil
}

// UnmapEndpointIP mocks the corresponding func in IPAM.
func (mock *MockIpam) UnmapEndpointIP(
	ctx context.Context,
	in *liqonetIpam.UnmapRequest,
	opts ...grpc.CallOption) (*liqonetIpam.UnmapResponse, error) {
	return &liqonetIpam.UnmapResponse{}, nil
}

// GetHomePodIP mocks the corresponding func in IPAM.
func (mock *MockIpam) GetHomePodIP(
	ctx context.Context,
	in *liqonetIpam.GetHomePodIPRequest,
	opts ...grpc.CallOption) (*liqonetIpam.GetHomePodIPResponse, error) {
	homeIP, err := utils.MapIPToNetwork(mock.RemoteRemappedPodCIDR, in.GetIp())
	if err != nil {
		return &liqonetIpam.GetHomePodIPResponse{}, err
	}
	return &liqonetIpam.GetHomePodIPResponse{HomeIP: homeIP}, nil
}

package test

import (
	"context"

	grpc "google.golang.org/grpc"

	"github.com/liqotech/liqo/pkg/liqonet"
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
	in *liqonet.MapRequest,
	opts ...grpc.CallOption) (*liqonet.MapResponse, error) {
	oldIP := in.GetIp()
	newIP, err := utils.MapIPToNetwork(mock.LocalRemappedPodCIDR, oldIP)
	if err != nil {
		return &liqonet.MapResponse{}, err
	}
	return &liqonet.MapResponse{Ip: newIP}, nil
}

// UnmapEndpointIP mocks the corresponding func in IPAM.
func (mock *MockIpam) UnmapEndpointIP(
	ctx context.Context,
	in *liqonet.UnmapRequest,
	opts ...grpc.CallOption) (*liqonet.UnmapResponse, error) {
	return &liqonet.UnmapResponse{}, nil
}

// GetHomePodIP mocks the corresponding func in IPAM.
func (mock *MockIpam) GetHomePodIP(
	ctx context.Context,
	in *liqonet.GetHomePodIPRequest,
	opts ...grpc.CallOption) (*liqonet.GetHomePodIPResponse, error) {
	homeIP, err := utils.MapIPToNetwork(mock.RemoteRemappedPodCIDR, in.GetIp())
	if err != nil {
		return &liqonet.GetHomePodIPResponse{}, err
	}
	return &liqonet.GetHomePodIPResponse{HomeIP: homeIP}, nil
}

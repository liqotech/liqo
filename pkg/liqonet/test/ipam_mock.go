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

// GetHomePodIP mocks the corresponding func in IPAM.
func (mock *MockIpam) GetRemotePodIP(
	ctx context.Context,
	in *liqonetIpam.GetRemotePodIPRequest,
	opts ...grpc.CallOption) (*liqonetIpam.GetRemotePodIPResponse, error) {
	remoteIP, err := utils.MapIPToNetwork(mock.RemoteRemappedPodCIDR, in.GetIp())
	if err != nil {
		return &liqonetIpam.GetRemotePodIPResponse{}, err
	}
	return &liqonetIpam.GetRemotePodIPResponse{RemoteIP: remoteIP}, nil
}

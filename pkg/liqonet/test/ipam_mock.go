package test

import (
	"context"

	grpc "google.golang.org/grpc"

	"github.com/liqotech/liqo/pkg/liqonet"
)

// MockIpam mocks the IPAM module.
type MockIpam struct {
	RemappedPodCIDR string
}

// MapEndpointIP mocks the corresponding func in IPAM.
func (mock *MockIpam) MapEndpointIP(
	ctx context.Context,
	in *liqonet.MapRequest,
	opts ...grpc.CallOption) (*liqonet.MapResponse, error) {
	oldIP := in.GetIp()
	newIP, err := liqonet.MapIPToNetwork(mock.RemappedPodCIDR, oldIP)
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

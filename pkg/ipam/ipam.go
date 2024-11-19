// Copyright 2019-2024 The Liqo Authors
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

package ipam

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LiqoIPAM is the struct implementing the IPAM interface.
type LiqoIPAM struct {
	UnimplementedIPAMServer
	HealthServer *health.Server

	client.Client

	opts          *ServerOptions
	cacheNetworks map[string]networkInfo
	cacheIPs      map[string]ipInfo
	mutex         sync.Mutex
}

// ServerOptions contains the options to configure the IPAM server.
type ServerOptions struct {
	Port          int
	SyncFrequency time.Duration
}

// New creates a new instance of the LiqoIPAM.
func New(ctx context.Context, cl client.Client, opts *ServerOptions) (*LiqoIPAM, error) {
	hs := health.NewServer()
	hs.SetServingStatus(IPAM_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	lipam := &LiqoIPAM{
		HealthServer: hs,

		Client: cl,

		opts:          opts,
		cacheNetworks: make(map[string]networkInfo),
		cacheIPs:      make(map[string]ipInfo),
	}

	// Initialize the IPAM instance
	if err := lipam.initialize(ctx); err != nil {
		return nil, err
	}

	// Launch sync routine
	go lipam.sync(ctx, opts.SyncFrequency)

	hs.SetServingStatus(IPAM_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)

	return lipam, nil
}

// IPAcquire acquires a free IP from a given CIDR.
func (lipam *LiqoIPAM) IPAcquire(_ context.Context, req *IPAcquireRequest) (*IPAcquireResponse, error) {
	remappedIP, err := lipam.acquireIP(req.GetCidr())
	if err != nil {
		return &IPAcquireResponse{}, err
	}

	return &IPAcquireResponse{Ip: remappedIP}, nil
}

// IPRelease releases an IP from a given CIDR.
func (lipam *LiqoIPAM) IPRelease(_ context.Context, req *IPReleaseRequest) (*IPReleaseResponse, error) {
	lipam.freeIP(ipCidr{ip: req.GetIp(), cidr: req.GetCidr()})

	return &IPReleaseResponse{}, nil
}

// NetworkAcquire acquires a network. If it is already reserved, it allocates and reserves a new free one with the same prefix length.
func (lipam *LiqoIPAM) NetworkAcquire(_ context.Context, req *NetworkAcquireRequest) (*NetworkAcquireResponse, error) {
	remappedCidr, err := lipam.acquireNetwork(req.GetCidr(), uint(req.GetPreAllocated()), req.GetImmutable())
	if err != nil {
		return &NetworkAcquireResponse{}, err
	}

	return &NetworkAcquireResponse{Cidr: remappedCidr}, nil
}

// NetworkRelease releases a network.
func (lipam *LiqoIPAM) NetworkRelease(_ context.Context, req *NetworkReleaseRequest) (*NetworkReleaseResponse, error) {
	lipam.freeNetwork(network{cidr: req.GetCidr()})

	return &NetworkReleaseResponse{}, nil
}

// NetworkIsAvailable checks if a network is available.
func (lipam *LiqoIPAM) NetworkIsAvailable(_ context.Context, req *NetworkAvailableRequest) (*NetworkAvailableResponse, error) {
	available := lipam.isNetworkAvailable(network{cidr: req.GetCidr()})

	return &NetworkAvailableResponse{Available: available}, nil
}

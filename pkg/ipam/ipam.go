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

package ipam

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ipamcore "github.com/liqotech/liqo/pkg/ipam/core"
)

// LiqoIPAM is the struct implementing the IPAM interface.
type LiqoIPAM struct {
	UnimplementedIPAMServer

	IpamCore *ipamcore.Ipam
	mutex    sync.Mutex

	HealthServer *health.Server
	client.Client
	opts *ServerOptions
}

// ServerOptions contains the options to configure the IPAM server.
type ServerOptions struct {
	Pools           []string
	Port            int
	SyncInterval    time.Duration
	SyncGracePeriod time.Duration
	GraphvizEnabled bool
}

// New creates a new instance of the LiqoIPAM.
func New(ctx context.Context, cl client.Client, opts *ServerOptions) (*LiqoIPAM, error) {
	hs := health.NewServer()
	hs.SetServingStatus(IPAM_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	prefixRoots := make([]netip.Prefix, len(opts.Pools))
	for i, r := range opts.Pools {
		p, err := netip.ParsePrefix(r)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pool with prefix %q: %w", r, err)
		}
		prefixRoots[i] = p
	}

	ipam, err := ipamcore.NewIpam(prefixRoots)
	if err != nil {
		return nil, err
	}

	lipam := &LiqoIPAM{
		IpamCore: ipam,

		HealthServer: hs,
		Client:       cl,
		opts:         opts,
	}

	// Initialize the IPAM instance
	if err := lipam.initialize(ctx); err != nil {
		return nil, err
	}

	// Launch sync routine
	go lipam.sync(ctx, opts.SyncInterval)

	hs.SetServingStatus(IPAM_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)

	return lipam, nil
}

// IPAcquire acquires a free IP from a given CIDR.
func (lipam *LiqoIPAM) IPAcquire(_ context.Context, req *IPAcquireRequest) (*IPAcquireResponse, error) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	prefix, err := netip.ParsePrefix(req.GetCidr())
	if err != nil {
		return &IPAcquireResponse{}, fmt.Errorf("failed to parse prefix %q: %w", req.GetCidr(), err)
	}

	if !lipam.isInPool(prefix) {
		return nil, fmt.Errorf("prefix %q is not in the pool %q", req.GetCidr(), strings.Join(lipam.opts.Pools, ","))
	}

	remappedIP, err := lipam.ipAcquire(prefix)
	if err != nil {
		return &IPAcquireResponse{}, err
	}

	return &IPAcquireResponse{Ip: remappedIP.String()}, nil
}

// IPRelease releases an IP from a given CIDR.
func (lipam *LiqoIPAM) IPRelease(_ context.Context, req *IPReleaseRequest) (*IPReleaseResponse, error) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	addr, err := netip.ParseAddr(req.GetIp())
	if err != nil {
		return &IPReleaseResponse{}, fmt.Errorf("failed to parse address %q: %w", req.GetIp(), err)
	}

	prefix, err := netip.ParsePrefix(req.GetCidr())
	if err != nil {
		return &IPReleaseResponse{}, fmt.Errorf("failed to parse prefix %q: %w", req.GetCidr(), err)
	}

	if err := lipam.ipRelease(addr, prefix, 0); err != nil {
		return &IPReleaseResponse{}, err
	}

	return &IPReleaseResponse{}, nil
}

// NetworkAcquire acquires a network. If it is already reserved, it allocates and reserves a new free one with the same prefix length.
func (lipam *LiqoIPAM) NetworkAcquire(_ context.Context, req *NetworkAcquireRequest) (*NetworkAcquireResponse, error) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	var remappedCidr *netip.Prefix
	var err error

	prefix, err := netip.ParsePrefix(req.GetCidr())
	if err != nil {
		return &NetworkAcquireResponse{}, fmt.Errorf("failed to parse prefix %q: %w", req.GetCidr(), err)
	}

	if !lipam.isInPool(prefix) {
		return &NetworkAcquireResponse{}, fmt.Errorf("prefix %q is not in the pool %q", req.GetCidr(), strings.Join(lipam.opts.Pools, ","))
	}

	if req.GetImmutable() {
		remappedCidr, err = lipam.networkAcquireSpecific(prefix)
		if err != nil {
			return &NetworkAcquireResponse{}, err
		}
	} else {
		remappedCidr, err = lipam.networkAcquire(prefix)
		if err != nil {
			return &NetworkAcquireResponse{}, err
		}
	}

	if err := lipam.acquirePreallocatedIPs(*remappedCidr, req.GetPreAllocated()); err != nil {
		return &NetworkAcquireResponse{}, errors.Join(err, lipam.networkRelease(*remappedCidr, 0))
	}

	return &NetworkAcquireResponse{Cidr: remappedCidr.String()}, nil
}

// NetworkRelease releases a network.
func (lipam *LiqoIPAM) NetworkRelease(_ context.Context, req *NetworkReleaseRequest) (*NetworkReleaseResponse, error) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	prefix, err := netip.ParsePrefix(req.GetCidr())
	if err != nil {
		return &NetworkReleaseResponse{}, fmt.Errorf("failed to parse prefix %q: %w", req.GetCidr(), err)
	}

	if err := lipam.networkRelease(prefix, 0); err != nil {
		return &NetworkReleaseResponse{}, err
	}

	return &NetworkReleaseResponse{}, nil
}

// NetworkIsAvailable checks if a network is available.
func (lipam *LiqoIPAM) NetworkIsAvailable(_ context.Context, req *NetworkAvailableRequest) (*NetworkAvailableResponse, error) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	prefix, err := netip.ParsePrefix(req.GetCidr())
	if err != nil {
		return &NetworkAvailableResponse{}, fmt.Errorf("failed to parse prefix %q: %w", req.GetCidr(), err)
	}

	if !lipam.isInPool(prefix) {
		return &NetworkAvailableResponse{}, fmt.Errorf("prefix %q is not in the pool %q", req.GetCidr(), strings.Join(lipam.opts.Pools, ","))
	}

	available := lipam.networkIsAvailable(prefix)

	return &NetworkAvailableResponse{Available: available}, nil
}

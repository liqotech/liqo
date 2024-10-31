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
	"time"

	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LiqoIPAM is the struct implementing the IPAM interface.
type LiqoIPAM struct {
	UnimplementedIPAMServer

	Options *Options
}

// Options contains the options to configure the IPAM.
type Options struct {
	Port   int
	Config *rest.Config
	Client client.Client

	EnableLeaderElection    bool
	LeaderElectionNamespace string
	LeaderElectionName      string
	LeaseDuration           time.Duration
	RenewDeadline           time.Duration
	RetryPeriod             time.Duration
	PodName                 string

	HealthServer *health.Server
}

// New creates a new instance of the LiqoIPAM.
func New(ctx context.Context, opts *Options) (*LiqoIPAM, error) {
	opts.HealthServer.SetServingStatus(IPAM_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	lipam := &LiqoIPAM{
		Options: opts,
	}

	if err := lipam.initialize(ctx); err != nil {
		return nil, err
	}

	opts.HealthServer.SetServingStatus(IPAM_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	return lipam, nil
}

// IPAcquire acquires a free IP from a given CIDR.
func (lipam *LiqoIPAM) IPAcquire(_ context.Context, _ *IPAcquireRequest) (*IPAcquireResponse, error) {
	panic("implement me")
}

// IPRelease releases an IP from a given CIDR.
func (lipam *LiqoIPAM) IPRelease(_ context.Context, _ *IPReleaseRequest) (*IPReleaseResponse, error) {
	panic("implement me")
}

// NetworkAcquire acquires a network. If it is already reserved, it allocates and reserves a new free one with the same prefix length.
func (lipam *LiqoIPAM) NetworkAcquire(_ context.Context, _ *NetworkAcquireRequest) (*NetworkAcquireResponse, error) {
	panic("implement me")
}

// NetworkRelease releases a network.
func (lipam *LiqoIPAM) NetworkRelease(_ context.Context, _ *NetworkReleaseRequest) (*NetworkReleaseResponse, error) {
	panic("implement me")
}

// NetworkIsAvailable checks if a network is available.
func (lipam *LiqoIPAM) NetworkIsAvailable(_ context.Context, _ *NetworkAvailableRequest) (*NetworkAvailableResponse, error) {
	panic("implement me")
}

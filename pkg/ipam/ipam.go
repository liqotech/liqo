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
	"time"

	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// LiqoIPAM is the struct implementing the IPAM interface.
type LiqoIPAM struct {
	UnimplementedIPAMServer
}

// Options contains the options to configure the IPAM.
type Options struct {
	Port int

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
func New(opts *Options) *LiqoIPAM {
	opts.HealthServer.SetServingStatus(IPAM_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// TODO: add here the initialization logic

	opts.HealthServer.SetServingStatus(IPAM_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	return &LiqoIPAM{}
}

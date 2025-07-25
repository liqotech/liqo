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

package liqocontrollermanager

import (
	"time"

	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/utils/args"
)

// Options holds all configuration flags for the liqo-controller-manager.
type Options struct {
	// Cluster-wide modules enable/disable flags
	NetworkingEnabled     bool
	AuthenticationEnabled bool
	OffloadingEnabled     bool

	// Manager flags
	WebhookPort    int
	MetricsAddr    string
	ProbeAddr      string
	LeaderElection bool

	// Global parameters
	ResyncPeriod               time.Duration
	ClusterIDFlags             *args.ClusterIDFlags
	LiqoNamespace              string
	ForeignClusterWorkers      int
	ForeignClusterPingInterval time.Duration
	ForeignClusterPingTimeout  time.Duration
	DefaultLimitsEnforcement   string

	// Networking module
	IPAMServer                     string
	GatewayServerResources         args.StringList
	GatewayClientResources         args.StringList
	WgGatewayServerClusterRoleName string
	WgGatewayClientClusterRoleName string
	FabricFullMasqueradeEnabled    bool
	GwmasqbypassEnabled            bool
	NetworkWorkers                 int
	IPWorkers                      int
	GenevePort                     uint16

	// Authentication module
	APIServerAddressOverride string
	CAOverride               string
	TrustedCA                bool
	AWSConfig                *identitymanager.LocalAwsConfig
	ClusterLabels            args.StringMap
	IngressClasses           args.ClassNameList
	LoadBalancerClasses      args.ClassNameList
	DefaultNodeResources     args.ResourceMap
	GlobalLabels             args.StringMap
	GlobalAnnotations        args.StringMap

	// Offloading module
	EnableStorage               bool
	VirtualStorageClassName     string
	RealStorageClassName        string
	StorageNamespace            string
	EnableNodeFailureController bool
	ShadowPodWorkers            int
	ShadowEndpointSliceWorkers  int

	// Cross module
	EnableAPIServerIPRemapping bool
}

// NewOptions creates a new Options struct with default values.
func NewOptions() *Options {
	return &Options{
		AWSConfig: &identitymanager.LocalAwsConfig{},
	}
}

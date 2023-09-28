// Copyright 2019-2023 The Liqo Authors
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

package wireguard

import (
	"net"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/liqotech/liqo/pkg/gateway/tunnel/common"
)

// Options contains the options for the wireguard interface.
type Options struct {
	Name            string
	Namespace       string
	RemoteClusterID string
	GatewayUID      string

	Mode            common.Mode
	MTU             int
	PrivateKey      wgtypes.Key
	InterfaceName   string
	InterfaceIP     string
	ListenPort      int
	EndpointAddress string
	EndpointPort    int

	EndpointIP      net.IP
	EndpointIPMutex *sync.Mutex

	DNSCheckInterval time.Duration

	LeaderElection              bool
	LeaderElectionLeaseDuration time.Duration
	LeaderElectionRenewDeadline time.Duration
	LeaderElectionRetryPeriod   time.Duration

	MetricsAddress string
	ProbeAddr      string
}

// NewOptions returns a new Options struct.
func NewOptions() *Options {
	return &Options{
		EndpointIPMutex: &sync.Mutex{},
	}
}

// GenerateResourceName generates the name used for the resources created by the gateway.
// This will help if a suffix will be added to the name of the resources in future.
func GenerateResourceName(name string) string {
	return name
}

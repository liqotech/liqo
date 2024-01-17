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

package gateway

import (
	"fmt"
	"time"
)

// Options contains the options for the wireguard interface.
type Options struct {
	Name            string
	Namespace       string
	RemoteClusterID string

	GatewayUID string

	Mode Mode

	LeaderElection              bool
	LeaderElectionLeaseDuration time.Duration
	LeaderElectionRenewDeadline time.Duration
	LeaderElectionRetryPeriod   time.Duration

	MetricsAddress string
	ProbeAddr      string
}

// NewOptions returns a new Options struct.
func NewOptions() *Options {
	return &Options{}
}

// Mode is the mode in which the wireguard interface is configured.
type Mode string

const (
	// ModeServer is the mode in which the wireguard interface is configured as a server.
	ModeServer Mode = "server"
	// ModeClient is the mode in which the wireguard interface is configured as a client.
	ModeClient Mode = "client"
)

// String returns the string representation of the mode.
func (m Mode) String() string {
	return string(m)
}

// Set sets the value of the mode.
func (m *Mode) Set(value string) error {
	if value == "" {
		return fmt.Errorf("mode cannot be empty")
	}
	if value != ModeServer.String() && value != ModeClient.String() {
		return fmt.Errorf("invalid mode %q", value)
	}
	*m = Mode(value)
	return nil
}

// Type returns the type of the mode.
func (m *Mode) Type() string {
	return "string"
}

// GenerateResourceName generates the name used for the resources created by the gateway.
// This will help if a suffix will be added to the name of the resources in future.
func GenerateResourceName(name string) string {
	return name
}

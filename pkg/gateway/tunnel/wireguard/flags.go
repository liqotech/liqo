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
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/liqotech/liqo/pkg/gateway"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// FlagNameGatewayUID is the UID of the wireguard gateway.
	FlagNameGatewayUID FlagName = "gateway-uid"

	// FlagNameMTU is the MTU for the wireguard interface.
	FlagNameMTU FlagName = "mtu"
	// FlagNameListenPort is the listen port for the wireguard interface.
	FlagNameListenPort FlagName = "listen-port"
	// FlagNameInterfaceName is the name of the wireguard interface.
	FlagNameInterfaceName FlagName = "interface-name"
	// FlagNameInterfaceIP is the IP of the wireguard interface.
	FlagNameInterfaceIP FlagName = "interface-ip"
	// FlagNameEndpointAddress is the address of the endpoint for the wireguard interface.
	FlagNameEndpointAddress FlagName = "endpoint-address"
	// FlagNameEndpointPort is the port of the endpoint for the wireguard interface.
	FlagNameEndpointPort FlagName = "endpoint-port"

	// FlagNameDNSCheckInterval is the interval between two DNS checks.
	FlagNameDNSCheckInterval FlagName = "dns-check-interval"
)

// RequiredFlags contains the list of the mandatory flags.
var RequiredFlags = []FlagName{
	FlagNameGatewayUID,
}

// ClientRequiredFlags contains the list of the mandatory flags for the client mode.
var ClientRequiredFlags = []FlagName{
	FlagNameEndpointAddress,
}

// InitFlags initializes the flags for the wireguard tunnel.
func InitFlags(flagset *pflag.FlagSet, opts *Options) {
	flagset.StringVar(&opts.GatewayUID, FlagNameGatewayUID.String(), "", "Parent gateway resource UID")

	flagset.IntVar(&opts.MTU, FlagNameMTU.String(), 1420, "MTU for the interface")
	flagset.StringVar(&opts.InterfaceName, FlagNameInterfaceName.String(), "liqo-tunnel", "Name for the tunnel interface")
	flagset.IntVar(&opts.ListenPort, FlagNameListenPort.String(), 51820, "Listen port (server only)")
	flagset.StringVar(&opts.EndpointAddress, FlagNameEndpointAddress.String(), "", "Endpoint address (client only)")
	flagset.IntVar(&opts.EndpointPort, FlagNameEndpointPort.String(), 51820, "Endpoint port (client only)")

	flagset.DurationVar(&opts.DNSCheckInterval, FlagNameDNSCheckInterval.String(), 5*time.Minute, "Interval between two DNS checks")
}

// MarkFlagsRequired marks the flags as required.
func MarkFlagsRequired(cmd *cobra.Command, opts *Options) error {
	for _, flag := range RequiredFlags {
		if err := cmd.MarkFlagRequired(flag.String()); err != nil {
			return err
		}
	}
	if opts.GwOptions.Mode == gateway.ModeClient {
		for _, flag := range ClientRequiredFlags {
			if err := cmd.MarkFlagRequired(flag.String()); err != nil {
				return err
			}
		}
	}
	return nil
}

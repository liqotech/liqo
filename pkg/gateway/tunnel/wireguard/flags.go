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

package wireguard

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// FlagNameMTU is the MTU for the wireguard interface.
	FlagNameMTU FlagName = "mtu"
	// FlagNameListenPort is the listen port for the wireguard interface.
	FlagNameListenPort FlagName = "listen-port"
	// FlagNameInterfaceIP is the IP of the wireguard interface.
	FlagNameInterfaceIP FlagName = "interface-ip"
	// FlagNameEndpointAddress is the address of the endpoint for the wireguard interface.
	FlagNameEndpointAddress FlagName = "endpoint-address"
	// FlagNameEndpointPort is the port of the endpoint for the wireguard interface.
	FlagNameEndpointPort FlagName = "endpoint-port"
	// FlagNameKeysDir is the directory where the keys are stored.
	FlagNameKeysDir FlagName = "keys-dir"

	// FlagNameDNSCheckInterval is the interval between two DNS checks.
	FlagNameDNSCheckInterval FlagName = "dns-check-interval"

	// FlagNameImplementation is the implementation of the wireguard interface.
	FlagNameImplementation FlagName = "implementation"
)

// ClientRequiredFlags contains the list of the mandatory flags for the client mode.
var ClientRequiredFlags = []FlagName{
	FlagNameEndpointAddress,
}

// InitFlags initializes the flags for the wireguard tunnel.
func InitFlags(flagset *pflag.FlagSet, opts *Options) {
	flagset.IntVar(&opts.MTU, FlagNameMTU.String(), forge.DefaultMTU, "MTU for the interface")
	flagset.IntVar(&opts.ListenPort, FlagNameListenPort.String(), forge.DefaultGwServerPort, "Listen port (server only)")
	flagset.StringVar(&opts.EndpointAddress, FlagNameEndpointAddress.String(), "", "Endpoint address (client only)")
	flagset.IntVar(&opts.EndpointPort, FlagNameEndpointPort.String(), forge.DefaultGwServerPort, "Endpoint port (client only)")
	flagset.StringVar(&opts.KeysDir, FlagNameKeysDir.String(), forge.DefaultKeysDir, "Directory where the keys are stored")

	flagset.DurationVar(&opts.DNSCheckInterval, FlagNameDNSCheckInterval.String(), 5*time.Minute, "Interval between two DNS checks")

	flagset.Var(&opts.Implementation, "implementation", "Implementation of the wireguard interface (kernel or userspace)")
}

// MarkFlagsRequired marks the flags as required.
func MarkFlagsRequired(cmd *cobra.Command, opts *Options) error {
	if opts.GwOptions.Mode == gateway.ModeClient {
		for _, flag := range ClientRequiredFlags {
			if err := cmd.MarkFlagRequired(flag.String()); err != nil {
				return err
			}
		}
	}
	return nil
}

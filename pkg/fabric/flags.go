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

package fabric

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/liqotech/liqo/pkg/consts"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// FlagNameNodeName is the name of the node where the pod is scheduled.
	FlagNameNodeName FlagName = "nodename"

	// FlagNameMetricsAddress is the address for the metrics endpoint.
	FlagNameMetricsAddress FlagName = "metrics-address"
	// FlagNameProbeAddr is the address for the health probe endpoint.
	FlagNameProbeAddr FlagName = "health-probe-bind-address"

	// FlagNameDisableARP is the flag to enable ARP.
	FlagNameDisableARP FlagName = "disable-arp"

	// FlagNameEnableNftMonitor is the flag to enable the nftables monitor.
	FlagNameEnableNftMonitor FlagName = "enable-nft-monitor"

	// FlagNameDisableKernelVersionCheck is the flag to enable the kernel version check.
	FlagNameDisableKernelVersionCheck FlagName = "disable-kernel-version-check"
	// FlagNameMinimumKernelVersion is the minimum kernel version required to run the wireguard interface.
	FlagNameMinimumKernelVersion FlagName = "minimum-kernel-version"

	// FlagNameGenevePort is the flag to set the Geneve port.
	FlagNameGenevePort FlagName = "geneve-port"
)

// RequiredFlags contains the list of the mandatory flags.
var RequiredFlags = []FlagName{
	FlagNameNodeName,
}

// InitFlags initializes the flags for the wireguard tunnel.
func InitFlags(flagset *pflag.FlagSet, opts *Options) {
	flagset.StringVar(&opts.NodeName, FlagNameNodeName.String(), "", "Name of the node where the pod is scheduled")
	flagset.StringVar(&opts.PodName, "podname", "", "Name of the pod")

	flagset.StringVar(&opts.MetricsAddress, FlagNameMetricsAddress.String(), ":8082", "Address for the metrics endpoint")
	flagset.StringVar(&opts.ProbeAddr, FlagNameProbeAddr.String(), ":8081", "Address for the health probe endpoint")

	flagset.BoolVar(&opts.DisableARP, FlagNameDisableARP.String(), false, "Disable ARP")
	flagset.BoolVar(&opts.EnableNftMonitor, FlagNameEnableNftMonitor.String(), true, "Enable nftables monitor")

	flagset.BoolVar(&opts.DisableKernelVersionCheck, FlagNameDisableKernelVersionCheck.String(), false, "Disable the kernel version check")
	flagset.Var(&opts.MinimumKernelVersion, string(FlagNameMinimumKernelVersion), "Minimum kernel version required to run the wireguard interface")

	flagset.Uint16Var(&opts.GenevePort, FlagNameGenevePort.String(), consts.DefaultGenevePort, "Geneve port")
}

// MarkFlagsRequired marks the flags as required.
func MarkFlagsRequired(cmd *cobra.Command) error {
	for _, flag := range RequiredFlags {
		if err := cmd.MarkFlagRequired(flag.String()); err != nil {
			return err
		}
	}
	return nil
}

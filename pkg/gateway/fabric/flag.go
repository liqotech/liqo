// Copyright 2019-2026 The Liqo Authors
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
	"time"

	"github.com/spf13/pflag"

	"github.com/liqotech/liqo/pkg/consts"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// FlagNameDisableARP is the flag to enable ARP.
	FlagNameDisableARP FlagName = "disable-arp"
	// FlagNameGenevePort is the flag to set the Geneve port.
	FlagNameGenevePort FlagName = "geneve-port"
	// FlagNameGeneveCleanupInterval is the flag to set the Geneve cleanup interval.
	FlagNameGeneveCleanupInterval FlagName = "geneve-cleanup-interval"

	// FlagNameGenevePingEnabled enables the geneve ping check.
	FlagNameGenevePingEnabled FlagName = "geneve-ping-enabled"
	// FlagNameGenevePingPort is the port for the geneve ping check.
	FlagNameGenevePingPort FlagName = "geneve-ping-port"
	// FlagNameGenevePingInterval is the interval between geneve pings.
	FlagNameGenevePingInterval FlagName = "geneve-ping-interval"
	// FlagNameGenevePingLossThreshold is the number of lost pings before declaring a tunnel down.
	FlagNameGenevePingLossThreshold FlagName = "geneve-ping-loss-threshold"
	// FlagNameGenevePingUpdateStatusInterval is the minimum interval between GeneveTunnel status updates.
	FlagNameGenevePingUpdateStatusInterval FlagName = "geneve-ping-update-status-interval"
	// FlagNameGenevePingLatencyAlpha is the EWMA smoothing factor for geneve tunnel latency.
	FlagNameGenevePingLatencyAlpha FlagName = "geneve-ping-latency-alpha"
)

// InitFlags initializes the flags for the gateway.
func InitFlags(flagset *pflag.FlagSet, opts *Options) {
	flagset.BoolVar(&opts.DisableARP, FlagNameDisableARP.String(), false, "Disable ARP")
	flagset.Uint16Var(&opts.GenevePort, FlagNameGenevePort.String(), consts.DefaultGenevePort, "Geneve port")
	flagset.DurationVar(&opts.GeneveCleanupInterval, FlagNameGeneveCleanupInterval.String(),
		consts.DefaultGeneveCleanupInterval, "Geneve cleanup interval")

	flagset.BoolVar(&opts.PingEnabled, FlagNameGenevePingEnabled.String(), true,
		"Enable geneve tunnel ping health check")
	flagset.IntVar(&opts.ConnCheckOptions.PingPort, FlagNameGenevePingPort.String(), 12346,
		"UDP port used for geneve tunnel ping health check")
	flagset.DurationVar(&opts.ConnCheckOptions.PingInterval, FlagNameGenevePingInterval.String(), 2*time.Second,
		"Interval between geneve tunnel pings")
	flagset.UintVar(&opts.ConnCheckOptions.PingLossThreshold, FlagNameGenevePingLossThreshold.String(), 5,
		"Number of consecutive lost pings before a geneve tunnel is declared down")
	flagset.DurationVar(&opts.PingUpdateStatusInterval, FlagNameGenevePingUpdateStatusInterval.String(), 10*time.Second,
		"Minimum interval between GeneveTunnel status updates")
	flagset.Float64Var(&opts.ConnCheckOptions.PingLatencyAlpha, FlagNameGenevePingLatencyAlpha.String(), 0.1,
		"EWMA smoothing factor for geneve tunnel latency (0 < alpha ≤ 1); lower values produce smoother readings")
}

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

package connection

import (
	"time"

	"github.com/spf13/pflag"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// EnableConnectionControllerFlag is the name of the flag used to enable the connection controller.
	EnableConnectionControllerFlag FlagName = "enable-connection-controller"
	// PingEnabledFlag is the name of the flag used to enable the ping check.
	PingEnabledFlag FlagName = "ping-enabled"
	// PingPortFlag is the name of the flag used to set the ping port.
	PingPortFlag FlagName = "ping-port"
	// PingBufferSizeFlag is the name of the flag used to set the ping buffer size.
	PingBufferSizeFlag FlagName = "ping-buffer-size"
	// PingLossThresholdFlag is the name of the flag used to set the ping loss threshold.
	PingLossThresholdFlag FlagName = "ping-loss-threshold"
	// PingIntervalFlag is the name of the flag used to set the ping interval.
	PingIntervalFlag FlagName = "ping-interval"
	// PingUpdateStatusIntervalFlag is the name of the flag used to set the ping update status interval.
	PingUpdateStatusIntervalFlag FlagName = "ping-update-status-interval"
)

// InitFlags initializes the flags for the wireguard tunnel.
func InitFlags(flagset *pflag.FlagSet, options *Options) {
	flagset.BoolVar(&options.EnableConnectionController, EnableConnectionControllerFlag.String(), true,
		"enable-connection-controller enables the connection controller. It is useful if the tunnel technology implements a connection check.")
	flagset.BoolVar(&options.PingEnabled, PingEnabledFlag.String(), true,
		"ping-enabled enables the ping check. If disabled the connection resource will be always connected and the latency won't be available.")
	flagset.IntVar(&options.ConnCheckOptions.PingPort, PingPortFlag.String(), 12345,
		"ping-port is the port used for the ping check")
	flagset.UintVar(&options.ConnCheckOptions.PingBufferSize, PingBufferSizeFlag.String(), 1024,
		"ping-buffer-size is the size of the buffer used for the ping check")
	flagset.UintVar(&options.ConnCheckOptions.PingLossThreshold, PingLossThresholdFlag.String(), 5,
		"ping-loss-threshold is the number of lost packets after which the connection check is considered as failed.")
	flagset.DurationVar(&options.ConnCheckOptions.PingInterval, PingIntervalFlag.String(), 2*time.Second,
		"ping-interval is the interval between two connection checks")
	flagset.DurationVar(&options.PingUpdateStatusInterval, PingUpdateStatusIntervalFlag.String(), 10*time.Second,
		"ping-update-status-interval is the interval at which the status is updated")
}

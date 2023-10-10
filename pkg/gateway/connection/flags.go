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

package connection

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/liqotech/liqo/pkg/gateway/connection/conncheck"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// PingLossThresholdFlag is the name of the flag used to set the ping loss threshold.
	PingLossThresholdFlag FlagName = "ping-loss-threshold"
	// PingIntervalFlag is the name of the flag used to set the ping interval.
	PingIntervalFlag FlagName = "ping-interval"
	// PingUpdateStatusIntervalFlag is the name of the flag used to set the ping update status interval.
	PingUpdateStatusIntervalFlag FlagName = "ping-update-status-interval"
)

// RequiredFlags contains the list of the mandatory flags.
var RequiredFlags = []FlagName{
	PingLossThresholdFlag,
	PingIntervalFlag,
}

// InitFlags initializes the flags for the wireguard tunnel.
func InitFlags(flagset *pflag.FlagSet) {
	flagset.UintVar(&conncheck.PingLossThreshold, PingLossThresholdFlag.String(), 5,
		"ping-loss-threshold is the number of lost packets after which the connection check is considered as failed.")
	flagset.DurationVar(&conncheck.PingInterval, PingIntervalFlag.String(), 2*time.Second,
		"ping-interval is the interval between two connection checks")
	flagset.DurationVar(&conncheck.PingUpdateStatusInterval, PingUpdateStatusIntervalFlag.String(), 10*time.Second,
		"ping-update-status-interval is the interval at which the status is updated")
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

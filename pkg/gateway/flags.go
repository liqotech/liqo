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

package gateway

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// FlagNameName is the name of the WgGateway resource.
	FlagNameName FlagName = "name"
	// FlagNameNamespace is the namespace WgGateway resource.
	FlagNameNamespace FlagName = "namespace"
	// FlagNameRemoteClusterID is the clusterID of the remote cluster.
	FlagNameRemoteClusterID FlagName = "remote-cluster-id"

	// FlagNameMode is the mode in which the wireguard interface is configured.
	FlagNameMode FlagName = "mode"

	// FlagNameLeaderElection is the flag to enable leader election.
	FlagNameLeaderElection FlagName = "leader-election"
	// FlagNameLeaderElectionLeaseDuration is the lease duration for the leader election.
	FlagNameLeaderElectionLeaseDuration FlagName = "leader-election-lease-duration"
	// FlagNameLeaderElectionRenewDeadline is the renew deadline for the leader election.
	FlagNameLeaderElectionRenewDeadline FlagName = "leader-election-renew-deadline"
	// FlagNameLeaderElectionRetryPeriod is the retry period for the leader election.
	FlagNameLeaderElectionRetryPeriod FlagName = "leader-election-retry-period"

	// FlagNameMetricsAddress is the address for the metrics endpoint.
	FlagNameMetricsAddress FlagName = "metrics-address"
	// FlagNameProbeAddr is the address for the health probe endpoint.
	FlagNameProbeAddr FlagName = "health-probe-bind-address"
)

// RequiredFlags contains the list of the mandatory flags.
var RequiredFlags = []FlagName{
	FlagNameName,
	FlagNameNamespace,
	FlagNameRemoteClusterID,
	FlagNameMode,
}

// InitFlags initializes the flags for the wireguard tunnel.
func InitFlags(flagset *pflag.FlagSet, opts *Options) {
	flagset.StringVar(&opts.Name, FlagNameName.String(), "", "Parent gateway name")
	flagset.StringVar(&opts.Namespace, FlagNameNamespace.String(), "", "Parent gateway namespace")
	flagset.StringVar(&opts.RemoteClusterID, FlagNameRemoteClusterID.String(), "", "ClusterID of the remote cluster")

	flagset.Var(&opts.Mode, FlagNameMode.String(), "Parent gateway mode")

	flagset.BoolVar(&opts.LeaderElection, FlagNameLeaderElection.String(), false, "Enable leader election")
	flagset.DurationVar(&opts.LeaderElectionLeaseDuration, FlagNameLeaderElectionLeaseDuration.String(), 15*time.Second,
		"LeaseDuration for the leader election")
	flagset.DurationVar(&opts.LeaderElectionRenewDeadline, FlagNameLeaderElectionRenewDeadline.String(), 10*time.Second,
		"RenewDeadline for the leader election")
	flagset.DurationVar(&opts.LeaderElectionRetryPeriod, FlagNameLeaderElectionRetryPeriod.String(), 2*time.Second,
		"RetryPeriod for the leader election")

	flagset.StringVar(&opts.MetricsAddress, FlagNameMetricsAddress.String(), ":8080", "Address for the metrics endpoint")
	flagset.StringVar(&opts.ProbeAddr, FlagNameProbeAddr.String(), ":8081", "Address for the health probe endpoint")
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

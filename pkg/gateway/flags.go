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
	// FlagNameName is the name of the Gateway resource.
	FlagNameName FlagName = "name"
	// FlagNameNamespace is the namespace Gateway resource.
	FlagNameNamespace FlagName = "namespace"
	// FlagNameRemoteClusterID is the clusterID of the remote cluster.
	FlagNameRemoteClusterID FlagName = "remote-cluster-id"
	// FlagNameNodeName is the name of the node.
	FlagNameNodeName FlagName = "node-name"
	// FlagNamePodName is the name of the pod.
	FlagNamePodName FlagName = "pod-name"
	// FlagContainerName is the name of the container.
	FlagContainerName FlagName = "container-name"

	// FlagNameGatewayUID is the UID of the Gateway resource.
	FlagNameGatewayUID FlagName = "gateway-uid"

	// FlagNameMode is the mode in which the gateway is configured.
	FlagNameMode FlagName = "mode"

	// FlagConcurrentContainersNames is the names of the containers that the gateway container must wait for.
	FlagConcurrentContainersNames FlagName = "concurrent-containers-names"

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

	// FlagNameDisableKernelVersionCheck is the flag to enable the kernel version check.
	FlagNameDisableKernelVersionCheck FlagName = "disable-kernel-version-check"
	// FlagNameMinimumKernelVersion is the minimum kernel version required by Liqo.
	FlagNameMinimumKernelVersion FlagName = "minimum-kernel-version"
)

// RequiredFlags contains the list of the mandatory flags.
var RequiredFlags = []FlagName{
	FlagNameName,
	FlagNameNamespace,
	FlagNameRemoteClusterID,
	FlagNameMode,
	FlagNameGatewayUID,
	FlagNameNodeName,
	FlagNamePodName,
	FlagContainerName,
}

// InitFlags initializes the flags for the gateway.
func InitFlags(flagset *pflag.FlagSet, opts *Options) {
	flagset.StringVar(&opts.Name, FlagNameName.String(), "", "Parent gateway name")
	flagset.StringVar(&opts.Namespace, FlagNameNamespace.String(), "", "Parent gateway namespace")
	flagset.StringVar(&opts.RemoteClusterID, FlagNameRemoteClusterID.String(), "", "ClusterID of the remote cluster")
	flagset.StringVar(&opts.NodeName, FlagNameNodeName.String(), "", "Node name")
	flagset.StringVar(&opts.PodName, FlagNamePodName.String(), "", "Pod name")
	flagset.StringVar(&opts.ContainerName, FlagContainerName.String(), "", "Container name")

	flagset.StringVar(&opts.GatewayUID, FlagNameGatewayUID.String(), "", "Parent gateway resource UID")

	flagset.Var(&opts.Mode, FlagNameMode.String(), "Parent gateway mode")

	flagset.StringSliceVar(&opts.ConcurrentContainersNames, FlagConcurrentContainersNames.String(),
		[]string{}, "the container list that gateway container must wait for")

	flagset.BoolVar(&opts.LeaderElection, FlagNameLeaderElection.String(), false, "Enable leader election")
	flagset.DurationVar(&opts.LeaderElectionLeaseDuration, FlagNameLeaderElectionLeaseDuration.String(), 15*time.Second,
		"LeaseDuration for the leader election")
	flagset.DurationVar(&opts.LeaderElectionRenewDeadline, FlagNameLeaderElectionRenewDeadline.String(), 10*time.Second,
		"RenewDeadline for the leader election")
	flagset.DurationVar(&opts.LeaderElectionRetryPeriod, FlagNameLeaderElectionRetryPeriod.String(), 2*time.Second,
		"RetryPeriod for the leader election")

	flagset.StringVar(&opts.MetricsAddress, FlagNameMetricsAddress.String(), "0", "Address for the metrics endpoint")
	flagset.StringVar(&opts.ProbeAddr, FlagNameProbeAddr.String(), "0", "Address for the health probe endpoint")

	flagset.BoolVar(&opts.DisableKernelVersionCheck, FlagNameDisableKernelVersionCheck.String(), false, "Disable the kernel version check")
	flagset.Var(&opts.MinimumKernelVersion, FlagNameMinimumKernelVersion.String(), "Minimum kernel version required by Liqo")
}

// MarkFlagsRequired marks the flags as required.
func MarkFlagsRequired(cmd *cobra.Command) error {
	for _, flag := range RequiredFlags {
		if err := cmd.MarkFlagRequired(flag.String()); err != nil {
			return err
		}
	}

	if cmd.Name() == "liqo-gateway" {
		if err := cmd.MarkFlagRequired(FlagConcurrentContainersNames.String()); err != nil {
			return err
		}
	}

	return nil
}

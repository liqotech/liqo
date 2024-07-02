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

package ipam

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
	// FlagNamePodCIDR is the podCIDR of the cluster.
	FlagNamePodCIDR FlagName = "pod-cidr"
	// FlagNameServiceCIDR is the serviceCIDR of the cluster.
	FlagNameServiceCIDR FlagName = "service-cidr"
	// FlagNameReservedPools contains the reserved network cidr of the cluster.
	FlagNameReservedPools FlagName = "reserved-pools"
	// FlagNameAdditionalPools contains additional network cidr used to map a cluster network into another one.
	FlagNameAdditionalPools FlagName = "additional-pools"

	// FlagNameLeaseEnabled is the flag to enable the lease for the IPAM pods.
	FlagNameLeaseEnabled FlagName = "lease-enabled"
	// FlagNameLeaseDuration is the duration that non-leader candidates will wait to force acquire leadership.
	FlagNameLeaseDuration FlagName = "lease-duration"
	// FlagNameLeaseRenewDeadline is the duration that the acting master will retry refreshing leadership before giving up.
	FlagNameLeaseRenewDeadline FlagName = "lease-renew-interval"
	// FlagNameLeaseRetryPeriod is the duration the LeaderElector clients should wait between tries of actions.
	FlagNameLeaseRetryPeriod FlagName = "lease-retry-period"
	// FlagNameLabelLeader is the flag to enable the label of the leader node.
	FlagNameLabelLeader FlagName = "label-leader"
)

// RequiredFlags contains the list of the mandatory flags.
var RequiredFlags = []FlagName{
	FlagNamePodCIDR,
	FlagNameServiceCIDR,
}

// InitFlags initializes the flags for the Options struct.
func InitFlags(flagset *pflag.FlagSet, o *Options) {
	flagset.Var(&o.PodCIDR, FlagNamePodCIDR.String(), "The subnet used by the cluster for the pods, in CIDR notation")
	flagset.Var(&o.ServiceCIDR, FlagNameServiceCIDR.String(), "The subnet used by the cluster for the pods, in services notation")
	flagset.Var(&o.ReservedPools, FlagNameReservedPools.String(),
		"Private CIDRs slices used by the Kubernetes infrastructure, in addition to the pod and service CIDR (e.g., the node subnet).")
	flagset.Var(&o.AdditionalPools, FlagNameAdditionalPools.String(),
		"Network pools used to map a cluster network into another one in order to prevent conflicts, in addition to standard private CIDRs.")

	flagset.BoolVar(&o.LeaseEnabled, FlagNameLeaseEnabled.String(), false,
		"Enable the lease for the IPAM pods. Disabling it will disable IPAM high-availability.")
	flagset.DurationVar(&o.LeaseDuration, FlagNameLeaseDuration.String(), 15*time.Second,
		"The duration that non-leader candidates will wait to force acquire leadership.")
	flagset.DurationVar(&o.LeaseRenewDeadline, FlagNameLeaseRenewDeadline.String(), 10*time.Second,
		"The duration that the acting master will retry refreshing leadership before giving up.")
	flagset.DurationVar(&o.LeaseRetryPeriod, FlagNameLeaseRetryPeriod.String(), 5*time.Second,
		"The duration the LeaderElector clients should wait between tries of actions.")
	flagset.BoolVar(&o.LabelLeader, FlagNameLabelLeader.String(), true,
		"Label the leader node.")
}

// MarkFlagsRequired marks the flags as required.
func MarkFlagsRequired(cmd *cobra.Command, _ *Options) error {
	for _, flag := range RequiredFlags {
		if err := cmd.MarkFlagRequired(flag.String()); err != nil {
			return err
		}
	}
	return nil
}

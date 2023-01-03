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

package peerib

import (
	"context"
	"time"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/inband"
)

// Options encapsulates the arguments of the peer in-band command.
type Options struct {
	LocalFactory  *factory.Factory
	RemoteFactory *factory.Factory

	Bidirectional bool
	Timeout       time.Duration
}

// Run implements the peer in-band command.
func (o *Options) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster 1.
	cluster1 := inband.NewCluster(o.LocalFactory, o.RemoteFactory)
	if err := cluster1.Init(ctx); err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2 := inband.NewCluster(o.RemoteFactory, o.LocalFactory)
	if err := cluster2.Init(ctx); err != nil {
		return err
	}

	// Check whether a ForeignCluster resource already exists in cluster 1 for cluster 2, and perform sanity checks.
	if err := cluster1.CheckForeignCluster(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Check whether a ForeignCluster resource already exists in cluster 2 for cluster 1, and perform sanity checks.
	if err := cluster2.CheckForeignCluster(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// SetUp tenant namespace for cluster 2 in cluster 1.
	if err := cluster1.SetUpTenantNamespace(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}
	cluster2.SetRemTenantNS(cluster1.GetLocTenantNS())

	// SetUp tenant namespace for cluster  in cluster 2.
	if err := cluster2.SetUpTenantNamespace(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}
	cluster1.SetRemTenantNS(cluster2.GetLocTenantNS())

	// Configure network configuration for cluster 1.
	if err := cluster1.ExchangeNetworkCfg(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Configure network configuration for cluster 2.
	if err := cluster2.ExchangeNetworkCfg(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Port-forwarding ipam service for cluster 1.
	if err := cluster1.PortForwardIPAM(ctx); err != nil {
		return err
	}
	defer cluster1.StopPortForwardIPAM()

	// Port-forwarding ipam service for cluster 2.
	if err := cluster2.PortForwardIPAM(ctx); err != nil {
		return err
	}
	defer cluster2.StopPortForwardIPAM()

	// Creating IPAM client for cluster 1.
	ipamClient1, err := cluster1.NewIPAMClient(ctx)
	if err != nil {
		return err
	}

	// Creating IPAM client for cluster 2.
	ipamClient2, err := cluster2.NewIPAMClient(ctx)
	if err != nil {
		return err
	}

	// Mapping proxy's ip in cluster1 for cluster2.
	if err := cluster1.MapProxyIPForCluster(ctx, ipamClient1, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Mapping proxy's ip in cluster2 for cluster2
	if err := cluster2.MapProxyIPForCluster(ctx, ipamClient2, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Mapping auth's ip  in cluster1 for cluster2.
	if err := cluster1.MapAuthIPForCluster(ctx, ipamClient1, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Mapping auth's ip in cluster2 for cluster1
	if err := cluster2.MapAuthIPForCluster(ctx, ipamClient2, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Creating foreign cluster in cluster1 for cluster 2.
	if err := cluster1.EnforceForeignCluster(ctx, cluster2.GetClusterID(), cluster2.GetAuthToken(),
		cluster2.GetAuthURL(), cluster2.GetProxyURL()); err != nil {
		return err
	}

	// Creating foreign cluster in cluster2 for cluster 1.
	if err := cluster2.EnforceForeignCluster(ctx, cluster1.GetClusterID(), cluster1.GetAuthToken(),
		cluster1.GetAuthURL(), cluster1.GetProxyURL()); err != nil {
		return err
	}

	// Setting the foreign cluster outgoing flag in cluster 1 for cluster 2
	// This operation is performed after that both foreign clusters have already been successfully created, to prevent a
	// possible race condition in which the resource request originated by the local foreign cluster is replicated to and
	// reconciled in the remote cluster before we create the corresponding foreign cluster. This would cause an incorrect
	// foreign cluster (i.e., of type OutOfBand) to be automatically created, leading to a broken peering.
	if err := cluster1.EnforceOutgoingPeeringFlag(ctx, cluster2.GetClusterID(), true); err != nil {
		return err
	}

	// Setting the foreign cluster outgoing flag in cluster 2 for cluster 1
	if err := cluster2.EnforceOutgoingPeeringFlag(ctx, cluster1.GetClusterID(), o.Bidirectional); err != nil {
		return err
	}

	// Waiting for VPN connection to be established in cluster 1.
	if err := cluster1.Waiter.ForNetwork(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Waiting for VPN connection to be established in cluster 2.
	if err := cluster2.Waiter.ForNetwork(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Waiting for authentication to complete in cluster 1.
	if err := cluster1.Waiter.ForAuth(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Waiting for authentication to complete in cluster 2.
	if err := cluster2.Waiter.ForAuth(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Waiting for outgoing peering to complete in cluster 1.
	if err := cluster1.Waiter.ForOutgoingPeering(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Waiting for virtual node to be created in cluster 1.
	if err := cluster1.Waiter.ForNode(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	if !o.Bidirectional {
		return nil
	}

	// Waiting for outgoing peering to complete in cluster 2.
	if err := cluster2.Waiter.ForOutgoingPeering(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Waiting for virtual node to be created in cluster 2.
	return cluster2.Waiter.ForNode(ctx, cluster1.GetClusterID())
}

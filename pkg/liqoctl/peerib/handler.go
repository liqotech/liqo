// Copyright 2019-2022 The Liqo Authors
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
	"github.com/liqotech/liqo/pkg/liqoctl/inbound"
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
	cluster1 := inbound.NewCluster(o.LocalFactory, o.RemoteFactory)
	if err := cluster1.Init(ctx); err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2 := inbound.NewCluster(o.RemoteFactory, o.LocalFactory)
	if err := cluster2.Init(ctx); err != nil {
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
		cluster2.GetAuthURL(), cluster2.GetProxyURL(), true); err != nil {
		return err
	}

	// Creating foreign cluster in cluster2 for cluster 1.
	if err := cluster2.EnforceForeignCluster(ctx, cluster1.GetClusterID(), cluster1.GetAuthToken(),
		cluster1.GetAuthURL(), cluster1.GetProxyURL(), o.Bidirectional); err != nil {
		return err
	}

	// Waiting for VPN connection to be established in cluster 1.
	if err := cluster1.WaitForNetwork(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Waiting for VPN connection to be established in cluster 2.
	if err := cluster2.WaitForNetwork(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Waiting for VPN connection to be established in cluster 1.
	if err := cluster1.WaitForNetwork(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Waiting for VPN connection to be established in cluster 2.
	if err := cluster2.WaitForNetwork(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Waiting for authentication to complete in cluster 1.
	if err := cluster1.WaitForAuth(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Waiting for authentication to complete in cluster 2.
	return cluster2.WaitForAuth(ctx, cluster1.GetClusterID())
}

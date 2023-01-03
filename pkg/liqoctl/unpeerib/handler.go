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

package unpeerib

import (
	"context"
	"time"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/inband"
)

// Options encapsulates the arguments of the unpeer in-band command.
type Options struct {
	LocalFactory  *factory.Factory
	RemoteFactory *factory.Factory

	LocalLiqoNamespace  string
	RemoteLiqoNamespace string

	Timeout time.Duration
}

// Run implements the unpeer in-band command.
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

	// Disable peering in cluster 1.
	if err := cluster1.DisablePeering(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Disable peering in cluster 2.
	if err := cluster2.DisablePeering(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Wait to unpeer in cluster 1.
	if err := cluster1.Waiter.ForUnpeering(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Disable peering in cluster 2.
	if err := cluster2.Waiter.ForUnpeering(ctx, cluster1.GetClusterID()); err != nil {
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

	// Unmapping proxy's ip in cluster1 for cluster2.
	if err := cluster1.UnmapProxyIPForCluster(ctx, ipamClient1, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Unmapping proxy's ip in cluster2 for cluster2
	if err := cluster2.UnmapProxyIPForCluster(ctx, ipamClient2, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Unmapping auth's ip in cluster1 for cluster2.
	if err := cluster1.UnmapAuthIPForCluster(ctx, ipamClient1, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Unapping auth's ip in cluster2 for cluster1
	if err := cluster2.UnmapAuthIPForCluster(ctx, ipamClient2, cluster1.GetClusterID()); err != nil {
		return err
	}
	// Delete foreigncluster of cluster2 in cluster1.
	if err := cluster1.DeleteForeignCluster(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Delete foreigncluster of cluster1 in cluster2.
	if err := cluster2.DeleteForeignCluster(ctx, cluster1.GetClusterID()); err != nil {
		return err
	}

	// Clean up tenant namespace in cluster 1.
	if err := cluster1.TearDownTenantNamespace(ctx, cluster2.GetClusterID()); err != nil {
		return err
	}

	// Clean up tenant namespace in cluster 2.
	return cluster2.TearDownTenantNamespace(ctx, cluster1.GetClusterID())
}

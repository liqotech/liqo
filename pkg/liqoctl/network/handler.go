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

package network

import (
	"context"
	"time"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
)

// Options encapsulates the arguments of the network command.
type Options struct {
	LocalFactory  *factory.Factory
	RemoteFactory *factory.Factory

	Timeout time.Duration
	Wait    bool
}

// RunInit initializes the liqo networking between two clusters.
func (o *Options) RunInit(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster 1.
	cluster1 := NewCluster(o.LocalFactory, o.RemoteFactory)
	if err := cluster1.Init(ctx); err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2 := NewCluster(o.RemoteFactory, o.LocalFactory)
	if err := cluster2.Init(ctx); err != nil {
		return err
	}

	// Setup Configurations in cluster 1.
	if err := cluster1.SetupConfiguration(ctx, cluster2.NetworkConfiguration); err != nil {
		return err
	}

	// Setup Configurations in cluster 2.
	if err := cluster2.SetupConfiguration(ctx, cluster1.NetworkConfiguration); err != nil {
		return err
	}

	if o.Wait {
		// Wait for cluster 1 to be ready.
		if err := cluster1.Waiter.ForConfiguration(ctx, cluster2.NetworkConfiguration); err != nil {
			return err
		}

		// Wait for cluster 2 to be ready.
		if err := cluster2.Waiter.ForConfiguration(ctx, cluster1.NetworkConfiguration); err != nil {
			return err
		}
	}

	return nil
}

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

package unauthenticate

import (
	"context"
	"time"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
)

// Options encapsulates the arguments of the unauthenticate command.
type Options struct {
	LocalFactory  *factory.Factory
	RemoteFactory *factory.Factory

	Timeout time.Duration
	Wait    bool
}

// NewOptions returns a new Options struct.
func NewOptions(localFactory *factory.Factory) *Options {
	return &Options{
		LocalFactory: localFactory,
	}
}

// RunUnauthenticate delete an authentication with a provider cluster.
// In the consumer cluster, it deletes the control plane Identity.
// In the provider cluster, it deletes the Tenant.
// The execution is prevented if any ResourceSlice or VirtualNode associated with the provider cluster is found.
func (o *Options) RunUnauthenticate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster consumer.
	consumer := NewCluster(o.LocalFactory)
	if err := consumer.SetLocalClusterID(ctx); err != nil {
		return err
	}

	// Create and initialize cluster provider.
	provider := NewCluster(o.RemoteFactory)
	if err := provider.SetLocalClusterID(ctx); err != nil {
		return err
	}

	// Check if any resourceslice is still present on consumer cluster
	if err := consumer.CheckLeftoverResourceSlices(ctx, provider.localClusterID); err != nil {
		return err
	}

	// Delete control plane Identity on consumer cluster
	if err := consumer.DeleteControlPlaneIdentity(ctx, provider.localClusterID); err != nil {
		return err
	}

	// Delete tenant on provider cluster
	if err := provider.DeleteTenant(ctx, consumer.localClusterID); err != nil {
		return err
	}

	return nil
}

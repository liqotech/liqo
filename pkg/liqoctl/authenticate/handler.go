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

package authenticate

import (
	"context"
	"time"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
)

// Options encapsulates the arguments of the authenticate command.
type Options struct {
	LocalFactory  *factory.Factory
	RemoteFactory *factory.Factory
	Timeout       time.Duration

	InBand   bool
	ProxyURL string
}

// NewOptions returns a new Options struct.
func NewOptions(localFactory *factory.Factory) *Options {
	return &Options{
		LocalFactory: localFactory,
	}
}

// RunAuthenticate initializes the authentication with a provider cluster.
func (o *Options) RunAuthenticate(ctx context.Context) error {
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

	// Ensure that the tenant namespace exists in the consumer cluster.
	if err := consumer.EnsureTenantNamespace(ctx, provider.LocalClusterID); err != nil {
		return err
	}

	// Ensure that the tenant namespace exists in the provider cluster.
	if err := provider.EnsureTenantNamespace(ctx, consumer.LocalClusterID); err != nil {
		return err
	}

	// In the provider cluster, generate and store a nonce for the consumer cluster authentication challenge.
	nonce, err := provider.EnsureNonce(ctx)
	if err != nil {
		return err
	}

	// In the consumer cluster, ensure the nonce is signed and retrieve it.
	signedNonce, err := consumer.EnsureSignedNonce(ctx, nonce)
	if err != nil {
		return err
	}

	if o.InBand && o.ProxyURL == "" {
		// In-band authentication: forge the proxy URL.
		providerAPIServerProxyIP, err := provider.GetAPIServerProxyRemappedIP(ctx)
		if err != nil {
			return err
		}

		remappedIP, err := consumer.RemapIPExternalCIDR(ctx, providerAPIServerProxyIP)
		if err != nil {
			return err
		}

		o.ProxyURL = "http://" + remappedIP + ":8118"
	}

	// In the consumer cluster, forge a tenant resource to be applied on the provider cluster
	tenant, err := consumer.GenerateTenant(ctx, signedNonce, provider.TenantNamespace, &o.ProxyURL)
	if err != nil {
		return err
	}

	// In the provider cluster, apply the tenant resource.
	if err := provider.EnsureTenant(ctx, tenant); err != nil {
		return err
	}

	// In the provider cluster, forge an identity resource to be applied on the consumer cluster.
	identity, err := provider.GenerateIdentity(ctx, consumer.TenantNamespace)
	if err != nil {
		return err
	}

	// In the consumer cluster, apply the identity resource.
	if err := consumer.EnsureIdentity(ctx, identity); err != nil {
		return err
	}

	return nil
}

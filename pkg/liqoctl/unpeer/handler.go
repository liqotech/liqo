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

package unpeer

import (
	"context"
	"fmt"
	"time"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/network"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/unauthenticate"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
)

// Options encapsulates the arguments of the unpeer command.
type Options struct {
	LocalFactory  *factory.Factory
	RemoteFactory *factory.Factory
	waiter        *wait.Waiter

	Timeout time.Duration
	Wait    bool

	consumerClusterID discoveryv1alpha1.ClusterID
	providerClusterID discoveryv1alpha1.ClusterID
}

// NewOptions returns a new Options struct.
func NewOptions(localFactory *factory.Factory) *Options {
	return &Options{
		LocalFactory: localFactory,
		waiter:       wait.NewWaiterFromFactory(localFactory),
	}
}

// RunUnpeer implements the unpeer command.
func (o *Options) RunUnpeer(ctx context.Context) error {
	var err error

	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Get consumer clusterID
	o.consumerClusterID, err = liqoutils.GetClusterIDWithControllerClient(ctx, o.LocalFactory.CRClient, o.LocalFactory.LiqoNamespace)
	if err != nil {
		o.LocalFactory.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving cluster id: %v", output.PrettyErr(err)))
		return err
	}

	// Get provider clusterID
	o.providerClusterID, err = liqoutils.GetClusterIDWithControllerClient(ctx, o.RemoteFactory.CRClient, o.RemoteFactory.LiqoNamespace)
	if err != nil {
		o.RemoteFactory.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving cluster id: %v", output.PrettyErr(err)))
		return err
	}

	// Disable offloading
	if err := o.disableOffloading(ctx); err != nil {
		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to disable offloading: %w", err))
		return err
	}

	// Disable networking
	if err := o.disableNetworking(ctx); err != nil {
		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to disable networking: %w", err))
		return err
	}

	// Disable authentication
	if err := o.disableAuthentication(ctx); err != nil {
		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to disable authentication: %w", err))
		return err
	}

	return nil
}

func (o *Options) disableOffloading(ctx context.Context) error {
	// Delete all resourceslices on consumer cluster
	if err := deleteResourceSlicesByClusterID(ctx, o.LocalFactory, o.providerClusterID, o.Wait); err != nil {
		return err
	}

	// Delete all virtualnodes on consumer cluster
	if err := deleteVirtualNodesByClusterID(ctx, o.LocalFactory, o.providerClusterID, o.Wait); err != nil {
		return err
	}

	return nil
}

func (o *Options) disableNetworking(ctx context.Context) error {
	networkOptions := network.Options{
		LocalFactory:  o.LocalFactory,
		RemoteFactory: o.RemoteFactory,

		Timeout: o.Timeout,
		Wait:    true,
	}

	if err := networkOptions.RunReset(ctx); err != nil {
		return err
	}

	return nil
}

func (o *Options) disableAuthentication(ctx context.Context) error {
	unauthenticateOptions := unauthenticate.Options{
		LocalFactory:  o.LocalFactory,
		RemoteFactory: o.RemoteFactory,

		Timeout: o.Timeout,
		Wait:    true,
	}

	if err := unauthenticateOptions.RunUnauthenticate(ctx); err != nil {
		return err
	}

	return nil
}

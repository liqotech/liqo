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

package network

import (
	"context"
	"fmt"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/check"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/info"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/setup"
)

// Options contains the options for the network tests.
type Options struct {
	Nopts *flags.Options
}

// NewOptions creates a new Options instance.
func NewOptions(o *flags.Options) *Options {
	return &Options{
		Nopts: o,
	}
}

// RunNetworkTest runs the E2E tests.
func (o *Options) RunNetworkTest(ctx context.Context) error {
	printer := o.Nopts.Topts.LocalFactory.Printer

	printer.Logger.Info("Initializing client")
	cl, cfg, err := client.NewClient(ctx, o.Nopts)
	if err != nil {
		return fmt.Errorf("cannot create client: %w", err)
	}
	printer.Logger.Info("Client initialized")

	if o.Nopts.Info {
		if err := info.Info(ctx, cl, printer.Table); err != nil {
			return fmt.Errorf("error getting info: %w", err)
		}
	}

	printer.Logger.Info("Setting up infrastructure")
	totreplicas, err := setup.MakeInfrastructure(ctx, cl, o.Nopts)
	if err != nil {
		return fmt.Errorf("error setting up infrastructure: %w", err)
	}
	printer.Logger.Info("Infrastructure set up")

	if err := check.RunChecks(ctx, cl, cfg, o.Nopts, totreplicas); err != nil {
		return fmt.Errorf("error running checks: %w", err)
	}

	if o.Nopts.RemoveNamespace {
		printer.Logger.Info("Removing namespace")
		if err := setup.RemoveNamespace(ctx, cl); err != nil {
			return fmt.Errorf("error removing namespace: %w", err)
		}
		printer.Logger.Info("Namespace removed")
	}

	return nil
}

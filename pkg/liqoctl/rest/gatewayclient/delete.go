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

package gatewayclient

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
)

const liqoctlDeleteGatewayClientLongHelp = `Delete a GatewayClient.

Examples:
  $ {{ .Executable }} delete gatewayclient my-gateway-client`

// Delete deletes a GatewayClient.
func (o *Options) Delete(ctx context.Context, options *rest.DeleteOptions) *cobra.Command {
	o.deleteOptions = options

	cmd := &cobra.Command{
		Use:     "gatewayclient",
		Aliases: []string{"gatewayclients", "client", "clients", "gwc"},
		Short:   "Delete a GatewayClient",
		Long:    liqoctlDeleteGatewayClientLongHelp,

		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.GatewayClients(ctx, o.deleteOptions.Factory, 1),

		PreRun: func(_ *cobra.Command, args []string) {
			options.Name = args[0]
			o.deleteOptions = options
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleDelete(ctx))
		},
	}

	return cmd
}

func (o *Options) handleDelete(ctx context.Context) error {
	opts := o.deleteOptions
	s := opts.Printer.StartSpinner("Deleting GatewayClient")

	gatewayClient := &networkingv1beta1.GatewayClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
	}
	if err := o.deleteOptions.CRClient.Delete(ctx, gatewayClient); err != nil {
		err = fmt.Errorf("unable to delete GatewayClient: %w", err)
		s.Fail(err)
		return err
	}

	s.Success("GatewayClient deleted")
	return nil
}

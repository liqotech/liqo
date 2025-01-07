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

package virtualnode

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
)

const liqoctlDeleteVirtualNodeLongHelp = `Delete a virtual node.

Examples:
  $ {{ .Executable }} delete virtualnode my-cluster`

// Delete deletes a virtual node.
func (o *Options) Delete(ctx context.Context, options *rest.DeleteOptions) *cobra.Command {
	o.deleteOptions = options

	cmd := &cobra.Command{
		Use:     "virtualnode",
		Aliases: []string{"vn"},
		Short:   "Delete a virtual node",
		Long:    liqoctlDeleteVirtualNodeLongHelp,

		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.VirtualNodes(ctx, o.deleteOptions.Factory, 1),

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
	s := opts.Printer.StartSpinner("Deleting virtual node")

	virtualNode := &offloadingv1beta1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
	}
	if err := o.deleteOptions.CRClient.Delete(ctx, virtualNode); err != nil {
		err = fmt.Errorf("unable to delete virtual node: %w", err)
		s.Fail(err)
		return err
	}

	s.Success("Virtual node deleted")
	return nil
}

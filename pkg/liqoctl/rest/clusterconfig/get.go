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

package clusterconfig

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/args"
)

// Get implements the get command.
func (o *Options) Get(ctx context.Context, options *rest.GetOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "json")

	o.getOptions = options

	cmd := &cobra.Command{
		Use:     "cluster-config",
		Aliases: []string{"cc"},
		Short:   "Get the cluster configuration",
		Long:    "Get the cluster configuration",
		Args:    cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			options.OutputFormat = outputFormat.Value
			o.getOptions = options
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleGet(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting resource. Supported formats: json, yaml")

	cmd.Flags().BoolVar(&o.onlyClusterID, "cluster-id", false, "Print only the cluster ID")

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))

	return cmd
}

func (o *Options) handleGet(ctx context.Context) error {
	localClusterID, err := liqoutils.GetClusterIDWithControllerClient(ctx, o.getOptions.CRClient, o.getOptions.LiqoNamespace)
	if err != nil {
		return err
	}

	clusterConfig := ClusterConfigType{
		ClusterID: localClusterID,
	}

	switch {
	case o.onlyClusterID:
		fmt.Print(clusterConfig.ClusterID)
		return nil
	default:
		return o.output(&clusterConfig)
	}
}

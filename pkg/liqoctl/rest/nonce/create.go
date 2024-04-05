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

package nonce

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	noncecreatorcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/noncecreator-controller"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

const (
	// nonceSecretName is the name of the Secret used to store the Nonce.
	nonceSecretName = "liqo-nonce"
)

const liqoctlCreateNonceLongHelp = `Create a Nonce.

The Nonce secret is used to authenticate the remote cluster to the local cluster.

Examples:
  $ {{ .Executable }} create nonce --remote-cluster-id remote-cluster-id`

// Create creates a Nonce.
func (o *Options) Create(ctx context.Context, options *rest.CreateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	o.createOptions = options

	cmd := &cobra.Command{
		Use:     "nonce",
		Aliases: []string{},
		Short:   "Create a nonce",
		Long:    liqoctlCreateNonceLongHelp,
		Args:    cobra.NoArgs,

		PreRun: func(cmd *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			o.createOptions = options

			o.namespaceManager = tenantnamespace.NewManager(options.KubeClient)
		},

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(o.handleCreate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting Nonce secret, with no additional output. Supported formats: json, yaml")

	cmd.Flags().StringVar(&o.clusterIdentity.ClusterID, "remote-cluster-id", "", "The cluster ID of the remote cluster")
	cmd.Flags().StringVar(&o.clusterIdentity.ClusterName, "remote-cluster-name", "", "The name of the remote cluster")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("remote-cluster-name"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-name", completion.ClusterNames(ctx,
		o.createOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	opts := o.createOptions
	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output())
		return nil
	}

	s := opts.Printer.StartSpinner("Creating nonce")

	namespace, err := o.namespaceManager.CreateNamespace(ctx, o.clusterIdentity)
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to create tenant namespace: %v", output.PrettyErr(err)))
		return err
	}

	nonce := forge.Nonce(nonceSecretName, namespace.Name)
	_, err = controllerutil.CreateOrUpdate(ctx, opts.CRClient, nonce, func() error {
		return forge.MutateNonce(nonce, o.clusterIdentity.ClusterID)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to create nonce: %v", output.PrettyErr(err)))
		return err
	}

	s.Success("Nonce created")

	waiter := wait.NewWaiterFromFactory(opts.Factory)
	if err := waiter.ForNonce(ctx, &o.clusterIdentity); err != nil {
		return err
	}

	s = opts.Printer.StartSpinner("Retrieving nonce")

	nonce, err = getters.GetNonceByClusterID(ctx, opts.CRClient, o.clusterIdentity.ClusterID)
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to get nonce: %v", output.PrettyErr(err)))
		return err
	}

	nonceValue, err := noncecreatorcontroller.GetNonceFromSecret(nonce)
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to get nonce: %v", output.PrettyErr(err)))
		return err
	}

	s.Success(fmt.Sprintf("Nonce retrieved: %s", string(nonceValue)))

	return nil
}

// output implements the logic to output the generated Nonce secret.
func (o *Options) output() error {
	opts := o.createOptions
	var printer printers.ResourcePrinter
	switch opts.OutputFormat {
	case "yaml":
		printer = &printers.YAMLPrinter{}
	case "json":
		printer = &printers.JSONPrinter{}
	default:
		return fmt.Errorf("unsupported output format %q", opts.OutputFormat)
	}

	nonce := forge.Nonce(nonceSecretName, "")
	if err := forge.MutateNonce(nonce, o.clusterIdentity.ClusterID); err != nil {
		return err
	}

	return printer.PrintObj(nonce, os.Stdout)
}

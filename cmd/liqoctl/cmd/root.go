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

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/cmd/util"

	"github.com/liqotech/liqo/pkg/liqoctl/create"
	"github.com/liqotech/liqo/pkg/liqoctl/delete"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/generate"
	"github.com/liqotech/liqo/pkg/liqoctl/get"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/configuration"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayclient"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayserver"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/identity"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/kubeconfig"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/nonce"
	peeringuser "github.com/liqotech/liqo/pkg/liqoctl/rest/peering-user"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/publickey"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/resourceslice"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/tenant"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/virtualnode"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

var liqoctl string

var liqoResources = []rest.APIProvider{
	virtualnode.VirtualNode,
	configuration.Configuration,
	gatewayserver.GatewayServer,
	gatewayclient.GatewayClient,
	publickey.PublicKey,
	tenant.Tenant,
	nonce.Nonce,
	peeringuser.PeeringUser,
	identity.Identity,
	resourceslice.ResourceSlice,
	kubeconfig.Kubeconfig,
}

func init() {
	liqoctl = os.Args[0]

	// Account for the case it is used as a kubectl plugin.
	if strings.HasPrefix(filepath.Base(liqoctl), "kubectl-") {
		liqoctl = strings.ReplaceAll(filepath.Base(liqoctl), "-", " ")
		liqoctl = strings.ReplaceAll(liqoctl, "_", "-")
	}
}

// liqoctlLongHelp contains the long help message for root Liqoctl command.
const liqoctlLongHelp = `{{ .Executable}} is a CLI tool to install and manage Liqo.

Liqo is a platform to enable dynamic and decentralized resource sharing across
Kubernetes clusters, either on-prem or managed. Liqo allows to run pods on a
remote cluster seamlessly and without any modification of Kubernetes and the
applications. With Liqo it is possible to extend the control and data plane of a
Kubernetes cluster across the cluster's boundaries, making multi-cluster native
and transparent: collapse an entire remote cluster to a local virtual node,
enabling workloads offloading, resource management and cross-cluster communication
compliant with the standard Kubernetes approach.
`

// NewRootCommand initializes the tree of commands.
func NewRootCommand(ctx context.Context) *cobra.Command {
	f := factory.NewForLocal()

	// cmd represents the base command when called without any subcommands.
	cmd := &cobra.Command{
		Use:          liqoctl,
		Short:        "A CLI tool to install and manage Liqo",
		Long:         WithTemplate(liqoctlLongHelp),
		Args:         cobra.NoArgs,
		SilenceUsage: true, // Do not show the usage message in case of errors.

		// Initialize the factory with default parameters: thanks to lazy loading, this introduces no overhead,
		// as well as no requirement for a valid kubeconfig if no subsequent API interaction is involved.
		// The behavior can be customized in subcommands defining an appropriate PersistentPreRun function.
		// Yet, the initialization is skipped for the __complete command, as characterized by a peculiar behavior
		// in terms of flags parsing (https://github.com/spf13/cobra/issues/1291#issuecomment-739056690).
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			if cmd != nil && cmd.Name() != cobra.ShellCompRequestCmd {
				singleClusterPersistentPreRun(cmd, f)
			}
		},
	}

	// In case liqoctl is used as a kubectl plugin, let's set a custom usage template with kubectl
	// hardcoded in it, since Cobra does not allow to specify a two word command (i.e., "kubectl liqo")
	if strings.HasPrefix(liqoctl, "kubectl ") {
		cmd.Use = strings.TrimPrefix(liqoctl, "kubectl ")
		cmd.SetUsageTemplate(strings.NewReplacer(
			"{{.UseLine}}", "kubectl {{.UseLine}}",
			"{{.CommandPath}}", "kubectl {{.CommandPath}}").
			Replace(cmd.UsageTemplate()))
	}

	// Add the flags regarding Kubernetes access options.
	f.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)
	cmd.PersistentFlags().BoolVar(&f.SkipConfirm, "skip-confirm", false, "Skip the confirmation prompt (suggested for automation)")
	cmd.PersistentFlags().StringToStringVar(&f.GlobalLabels, "global-labels", nil,
		"Global labels to be added to all created resources (key=value)")
	cmd.PersistentFlags().StringToStringVar(&f.GlobalAnnotations, "global-annotations", nil,
		"Global annotations to be added to all created resources (key=value)")

	cmd.AddCommand(newInstallCommand(ctx, f))
	cmd.AddCommand(newUninstallCommand(ctx, f))
	cmd.AddCommand(newPeerCommand(ctx, f))
	cmd.AddCommand(newUnpeerCommand(ctx, f))
	cmd.AddCommand(newNetworkCommand(ctx, f))
	cmd.AddCommand(newAuthenticateCommand(ctx, f))
	cmd.AddCommand(newUnauthenticateCommand(ctx, f))
	cmd.AddCommand(newOffloadCommand(ctx, f))
	cmd.AddCommand(newUnoffloadCommand(ctx, f))
	cmd.AddCommand(newMoveCommand(ctx, f))
	cmd.AddCommand(newVersionCommand(ctx, f))
	cmd.AddCommand(newDocsCommand(ctx))
	cmd.AddCommand(newActivateCommand(ctx, f))
	cmd.AddCommand(newCordonCommand(ctx, f))
	cmd.AddCommand(newUncordonCommand(ctx, f))
	cmd.AddCommand(newDrainCommand(ctx, f))
	cmd.AddCommand(create.NewCreateCommand(ctx, liqoResources, f))
	cmd.AddCommand(generate.NewGenerateCommand(ctx, liqoResources, f))
	cmd.AddCommand(get.NewGetCommand(ctx, liqoResources, f))
	cmd.AddCommand(delete.NewDeleteCommand(ctx, liqoResources, f))
	cmd.AddCommand(newInfoCommand(ctx, f))
	cmd.AddCommand(newTestCommand(ctx, f))

	return cmd
}

// WithTemplate returns a string that has the liqoctl name templated out with the
// current executable name. WithTemplate templates on the '{{ .Executable }}' variable.
func WithTemplate(str string) string {
	tmpl := template.Must(template.New("liqoctl").Parse(str))
	var buf bytes.Buffer
	util.CheckErr(tmpl.Execute(&buf, struct{ Executable string }{liqoctl}))
	return buf.String()
}

// singleClusterPersistentPreRun initializes the local factory.
func singleClusterPersistentPreRun(_ *cobra.Command, f *factory.Factory, opts ...factory.Options) {
	// Populate the factory fields based on the configured parameters.
	f.Printer.CheckErr(f.Initialize(opts...))
	resource.SetGlobalLabels(f.GlobalLabels)
	resource.SetGlobalAnnotations(f.GlobalAnnotations)
}

// twoClustersPersistentPreRun initializes both the local and the remote factory.
func twoClustersPersistentPreRun(cmd *cobra.Command, local, remote *factory.Factory, opts ...factory.Options) {
	// Initialize the local factory fields based on the configured parameters.
	singleClusterPersistentPreRun(cmd, local, opts...)

	// Populate the remote factory fields based on the configured parameters.
	remote.Printer.CheckErr(remote.Initialize(opts...))

	// Check that local and remote clusters are different.
	if reflect.DeepEqual(local.RESTConfig, remote.RESTConfig) {
		local.Printer.CheckErr(fmt.Errorf("local and remote clusters must be different"))
	}
}

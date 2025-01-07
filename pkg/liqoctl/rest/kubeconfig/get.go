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

package kubeconfig

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

const liqoctlGetKubeconfigLongHelp = `Get a Kubeconfig of an Identity of a remote cluster.

Examples:
  $ {{ .Executable }} get kubeconfig my-identity-name --remote-cluster-id remote-cluster-id`

// Get implements the get command.
func (o *Options) Get(ctx context.Context, options *rest.GetOptions) *cobra.Command {
	o.getOptions = options

	cmd := &cobra.Command{
		Use:     "kubeconfig",
		Aliases: []string{"kc"},
		Short:   "Get a kubeconfig",
		Long:    liqoctlGetKubeconfigLongHelp,
		Args:    cobra.ExactArgs(1),

		PreRun: func(_ *cobra.Command, args []string) {
			o.getOptions = options
			o.identityName = args[0]
			o.namespaceManager = tenantnamespace.NewManager(options.KubeClient, options.CRClient.Scheme())
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleGet(ctx))
		},
	}

	cmd.Flags().Var(&o.remoteClusterID, "remote-cluster-id", "The cluster ID of the remote cluster")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.getOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleGet(ctx context.Context) error {
	opts := o.getOptions

	namespace, err := o.namespaceManager.GetNamespace(ctx, o.remoteClusterID.GetClusterID())
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to get tenant namespace: %v", output.PrettyErr(err)))
		return err
	}

	var identity authv1beta1.Identity
	if err := opts.CRClient.Get(ctx, types.NamespacedName{Name: o.identityName, Namespace: namespace.Name}, &identity); err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to get identity: %v", output.PrettyErr(err)))
		return err
	}

	if identity.Status.KubeconfigSecretRef == nil || identity.Status.KubeconfigSecretRef.Name == "" {
		err := fmt.Errorf("the identity does not have a kubeconfig secret reference")
		opts.Printer.CheckErr(err)
		return err
	}

	var secret corev1.Secret
	if err := opts.CRClient.Get(ctx, types.NamespacedName{Name: identity.Status.KubeconfigSecretRef.Name, Namespace: identity.Namespace},
		&secret); err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to get kubeconfig secret: %v", output.PrettyErr(err)))
		return err
	}

	kubeconfig, ok := secret.Data[consts.KubeconfigSecretField]
	if !ok {
		err := fmt.Errorf("the kubeconfig secret does not contain the kubeconfig field")
		opts.Printer.CheckErr(err)
		return err
	}

	fmt.Println(string(kubeconfig))

	return nil
}

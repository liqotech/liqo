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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

const liqoctlCreateNonceLongHelp = `Create a Nonce.

The Nonce secret is used to authenticate the remote cluster to the local cluster.

Examples:
  $ {{ .Executable }} create nonce --cluster-id my-cluster-id`

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

	cmd.Flags().StringVar(&o.clusterID, "cluster-id", "", "The cluster ID of the remote cluster")

	runtime.Must(cmd.MarkFlagRequired("cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("cluster-id", completion.ClusterIDs(ctx,
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

	namespace, err := o.namespaceManager.CreateNamespace(ctx, discoveryv1alpha1.ClusterIdentity{ClusterID: o.clusterID})
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to create tenant namespace: %v", output.PrettyErr(err)))
		return err
	}

	nonce := o.forgeNonce(consts.NonceSecretName, namespace.Name)
	_, err = controllerutil.CreateOrUpdate(ctx, opts.CRClient, nonce, func() error {
		return o.mutateNonce(nonce)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to create nonce: %v", output.PrettyErr(err)))
		return err
	}

	waiter := wait.NewWaiterFromFactory(opts.Factory)
	if err := waiter.ForNonce(ctx, &discoveryv1alpha1.ClusterIdentity{ClusterID: o.clusterID}); err != nil {
		s.Fail(fmt.Sprintf("Unable to wait for nonce: %v", output.PrettyErr(err)))
		return err
	}

	s.Success("Nonce created")

	s = opts.Printer.StartSpinner("Retrieving nonce")

	nonce, err = getters.GetNonceByClusterID(ctx, opts.CRClient, o.clusterID)
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to get nonce: %v", output.PrettyErr(err)))
		return err
	}

	s.Success(fmt.Sprintf("Nonce retrieved: %s", string(nonce.Data[consts.NonceKey])))

	return nil
}

func (o *Options) forgeNonce(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (o *Options) mutateNonce(nonce *corev1.Secret) error {
	if nonce.Labels == nil {
		nonce.Labels = make(map[string]string)
	}

	nonce.Labels[consts.NonceLabelKey] = "true"
	nonce.Labels[discovery.ClusterIDLabel] = o.clusterID

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

	nonce := o.forgeNonce(consts.NonceSecretName, "")
	if err := o.mutateNonce(nonce); err != nil {
		return err
	}

	return printer.PrintObj(nonce, os.Stdout)
}

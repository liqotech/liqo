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

package resourceslice

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/args"
)

const liqoctlCreateResourceSliceLongHelp = `Create a ResourceSlice.

The ResourceSlice resource is used to represent a slice of resources that can be shared across clusters.

Examples:
  $ {{ .Executable }} create resourceslice my-slice --remote-cluster-id remote-cluster-id \
  --cpu 4 --memory 8Gi --pods 30`

// Create implements the create command.
func (o *Options) Create(ctx context.Context, options *rest.CreateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	o.createOptions = options

	cmd := &cobra.Command{
		Use:     "resourceslice",
		Aliases: []string{"rs", "resourceslices"},
		Short:   "Create a ResourceSlice",
		Long:    liqoctlCreateResourceSliceLongHelp,
		Args:    cobra.ExactArgs(1),

		PreRun: func(cmd *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			options.Name = args[0]
			o.createOptions = options

			o.namespaceManager = tenantnamespace.NewManager(options.Factory.KubeClient)
		},

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(o.handleCreate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting ResourceSlice resource, instead of applying it. Supported formats: json, yaml")

	cmd.Flags().StringVar(&o.remoteClusterID, "remote-cluster-id", "", "The cluster ID of the remote cluster")
	cmd.Flags().StringVar(&o.class, "class", "default", "The class of the ResourceSlice")
	cmd.Flags().StringVar(&o.cpu, "cpu", "", "The amount of CPU requested in the resource slice")
	cmd.Flags().StringVar(&o.memory, "memory", "", "The amount of memory requested in the resource slice")
	cmd.Flags().StringVar(&o.pods, "pods", "", "The amount of pods requested in the resource slice")
	cmd.Flags().BoolVar(&o.disableVirtualNodeCreation, "no-virtual-node", false,
		"Prevent the automatic creation of a VirtualNode for the ResourceSlice. Default: false")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	opts := o.createOptions
	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output(ctx))
		return nil
	}

	s := opts.Printer.StartSpinner("Creating ResourceSlice")

	namespace, err := o.getTenantNamespace(ctx)
	if err != nil {
		s.Fail("Unable to get tenant namespace: %v", output.PrettyErr(err))
		return err
	}

	resourceSlice := forge.ResourceSlice(opts.Name, namespace)
	_, err = controllerutil.CreateOrUpdate(ctx, opts.CRClient, resourceSlice, func() error {
		return forge.MutateResourceSlice(resourceSlice, o.remoteClusterID, &forge.ResourceSliceOptions{
			Class: authv1alpha1.ResourceSliceClass(o.class),
			Resources: map[corev1.ResourceName]string{
				corev1.ResourceCPU:    o.cpu,
				corev1.ResourceMemory: o.memory,
				corev1.ResourcePods:   o.pods,
			},
		}, !o.disableVirtualNodeCreation)
	})
	if err != nil {
		s.Fail("Unable to create ResourceSlice: %v", output.PrettyErr(err))
		return err
	}
	s.Success("ResourceSlice created")

	waiter := wait.NewWaiterFromFactory(opts.Factory)
	if err := waiter.ForResourceSlice(ctx, resourceSlice); err != nil {
		return err
	}

	return nil
}

func (o *Options) getTenantNamespace(ctx context.Context) (string, error) {
	ns, err := o.namespaceManager.GetNamespace(ctx, discoveryv1alpha1.ClusterIdentity{ClusterID: o.remoteClusterID})
	switch {
	case err == nil:
		return ns.Name, nil
	case apierrors.IsNotFound(err):
		return "", fmt.Errorf("tenant namespace not found for cluster %q", o.remoteClusterID)
	default:
		return "", err
	}
}

// output implements the logic to output the generated ResourceSlice resource.
func (o *Options) output(ctx context.Context) error {
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

	namespace, err := o.getTenantNamespace(ctx)
	if err != nil {
		return err
	}

	resourceSlice := forge.ResourceSlice(opts.Name, namespace)
	err = forge.MutateResourceSlice(resourceSlice, o.remoteClusterID, &forge.ResourceSliceOptions{
		Class: authv1alpha1.ResourceSliceClass(o.class),
		Resources: map[corev1.ResourceName]string{
			corev1.ResourceCPU:    o.cpu,
			corev1.ResourceMemory: o.memory,
			corev1.ResourcePods:   o.pods,
		},
	}, !o.disableVirtualNodeCreation)
	if err != nil {
		return err
	}

	return printer.PrintObj(resourceSlice, os.Stdout)
}

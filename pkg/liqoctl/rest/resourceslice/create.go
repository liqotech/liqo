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
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

const liqoctlCreateResourceSliceLongHelp = `Create a ResourceSlice.

The ResourceSlice resource is used to represent a slice of resources that can be shared across clusters.

Examples:
  $ {{ .Executable }} create resourceslice my-slice --remote-cluster-id remote-cluster-id \
  --cpu 4 --memory 8Gi --pods 30`

// Create implements the create command.
func (o *Options) Create(ctx context.Context, options *rest.CreateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	o.CreateOptions = options

	cmd := &cobra.Command{
		Use:     "resourceslice",
		Aliases: []string{"rs", "resourceslices"},
		Short:   "Create a ResourceSlice",
		Long:    liqoctlCreateResourceSliceLongHelp,
		Args:    cobra.ExactArgs(1),

		PreRun: func(_ *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			options.Name = args[0]
			o.CreateOptions = options

			o.NamespaceManager = tenantnamespace.NewManager(options.Factory.KubeClient, options.Factory.CRClient.Scheme())
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.HandleCreate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting ResourceSlice resource, instead of applying it. Supported formats: json, yaml")

	cmd.Flags().Var(&o.RemoteClusterID, "remote-cluster-id", "The cluster ID of the remote cluster")
	cmd.Flags().StringVar(&o.Class, "class", "default", "The class of the ResourceSlice")
	cmd.Flags().StringVar(&o.CPU, "cpu", "", "The amount of CPU requested in the resource slice")
	cmd.Flags().StringVar(&o.Memory, "memory", "", "The amount of memory requested in the resource slice")
	cmd.Flags().StringVar(&o.Pods, "pods", "", "The amount of pods requested in the resource slice")
	cmd.Flags().BoolVar(&o.DisableVirtualNodeCreation, "no-virtual-node", false,
		"Prevent the automatic creation of a VirtualNode for the ResourceSlice. Default: false")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.CreateOptions.Factory, completion.NoLimit)))

	return cmd
}

// HandleCreate implements the logic to create a ResourceSlice resource.
func (o *Options) HandleCreate(ctx context.Context) error {
	opts := o.CreateOptions
	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output(ctx))
		return nil
	}

	s := opts.Printer.StartSpinner("Creating ResourceSlice")

	namespace, err := o.getTenantNamespace(ctx)
	if err != nil {
		s.Fail("Unable to get tenant namespace: ", output.PrettyErr(err))
		return err
	}

	resourceSlice := forge.ResourceSlice(opts.Name, namespace)
	_, err = resource.CreateOrUpdate(ctx, opts.CRClient, resourceSlice, func() error {
		return forge.MutateResourceSlice(resourceSlice, o.RemoteClusterID.GetClusterID(), &forge.ResourceSliceOptions{
			Class: authv1beta1.ResourceSliceClass(o.Class),
			Resources: map[corev1.ResourceName]string{
				corev1.ResourceCPU:    o.CPU,
				corev1.ResourceMemory: o.Memory,
				corev1.ResourcePods:   o.Pods,
			},
		}, !o.DisableVirtualNodeCreation)
	})
	if err != nil {
		s.Fail("Unable to create ResourceSlice: %v", output.PrettyErr(err))
		return err
	}
	s.Success("ResourceSlice created")

	waiter := wait.NewWaiterFromFactory(opts.Factory)
	if err := waiter.ForResourceSliceAuthentication(ctx, resourceSlice); err != nil {
		return err
	}

	// Check if the resources are accepted by the provider cluster.
	// If the resources are not accepted, the provider cluster may have cordoned the tenant or the resourceslice.
	if err := opts.CRClient.Get(ctx, client.ObjectKeyFromObject(resourceSlice), resourceSlice); err != nil {
		return err
	}
	resourcesCondition := authentication.GetCondition(resourceSlice, authv1beta1.ResourceSliceConditionTypeResources)
	if resourcesCondition == nil || resourcesCondition.Status != authv1beta1.ResourceSliceConditionAccepted {
		opts.Printer.Warning.Printfln("ResourceSlice resources not accepted. The provider cluster may have cordoned the tenant or the resourceslice")
		return nil
	}
	opts.Printer.Success.Printfln("ResourceSlice resources: %s", resourcesCondition.Status)

	return nil
}

func (o *Options) getTenantNamespace(ctx context.Context) (string, error) {
	ns, err := o.NamespaceManager.GetNamespace(ctx, o.RemoteClusterID.GetClusterID())
	switch {
	case err == nil:
		return ns.Name, nil
	case apierrors.IsNotFound(err):
		return "", fmt.Errorf("tenant namespace not found for cluster %q", o.RemoteClusterID)
	default:
		return "", err
	}
}

// output implements the logic to output the generated ResourceSlice resource.
func (o *Options) output(ctx context.Context) error {
	opts := o.CreateOptions
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
	err = forge.MutateResourceSlice(resourceSlice, o.RemoteClusterID.GetClusterID(), &forge.ResourceSliceOptions{
		Class: authv1beta1.ResourceSliceClass(o.Class),
		Resources: map[corev1.ResourceName]string{
			corev1.ResourceCPU:    o.CPU,
			corev1.ResourceMemory: o.Memory,
			corev1.ResourcePods:   o.Pods,
		},
	}, !o.DisableVirtualNodeCreation)
	if err != nil {
		return err
	}

	return printer.PrintObj(resourceSlice, os.Stdout)
}

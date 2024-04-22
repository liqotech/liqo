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

package virtualnode

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

const liqoctlCreateVirtualNodeLongHelp = `Create a VirtualNode.

The VirtualNode resource is used to represent a remote cluster in the local cluster.

Examples:
  $ {{ .Executable }} create virtualnode my-cluster --cluster-id remote-cluster-id \
  --cluster-name remote-cluster-name --kubeconfig-secret-name my-cluster-kubeconfig
  Or, if creating a VirtualNode from a ResourceSlice:
  $ {{ .Executable }} create virtualnode my-cluster --cluster-id remote-cluster-id \
  --cluster-name remote-cluster-name --resource-slice-name my-resourceslice`

// Create creates a VirtualNode.
func (o *Options) Create(ctx context.Context, options *rest.CreateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	o.createOptions = options

	cmd := &cobra.Command{
		Use:     "virtualnode",
		Aliases: []string{"vn"},
		Short:   "Create a virtual node",
		Long:    liqoctlCreateVirtualNodeLongHelp,
		Args:    cobra.ExactArgs(1),

		PreRun: func(cmd *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			options.Name = args[0]
			o.createOptions = options

			o.namespaceManager = tenantnamespace.NewManager(options.KubeClient)
		},

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(o.handleCreate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting VirtualNode resource, instead of applying it. Supported formats: json, yaml")

	// TODO: check validity of both cluster-id and cluster-name
	cmd.Flags().StringVar(&o.remoteClusterIdentity.ClusterID, "cluster-id", "", "The cluster ID of the remote cluster")
	cmd.Flags().StringVar(&o.remoteClusterIdentity.ClusterName, "cluster-name", "", "The cluster name of the remote cluster")
	cmd.Flags().BoolVar(&o.createNode, "create-node",
		true, "Create a node to target the remote cluster (and schedule on it)")
	cmd.Flags().StringVar(&o.kubeconfigSecretName, "kubeconfig-secret-name",
		"", "The name of the secret containing the kubeconfig of the remote cluster. Mutually exclusive with --resource-slice-name")
	cmd.Flags().StringVar(&o.resourceSliceName, "resource-slice-name",
		"", "The name of the resourceslice to be used to create the virtual node. Mutually exclusive with --kubeconfig-secret-name")
	cmd.Flags().StringVar(&o.cpu, "cpu", "2", "The amount of CPU available in the virtual node")
	cmd.Flags().StringVar(&o.memory, "memory", "4Gi", "The amount of memory available in the virtual node")
	cmd.Flags().StringVar(&o.pods, "pods", "110", "The amount of pods available in the virtual node")
	cmd.Flags().StringSliceVar(&o.storageClasses, "storage-classes",
		[]string{}, "The storage classes offered by the remote cluster. The first one will be used as default")
	cmd.Flags().StringSliceVar(&o.ingressClasses, "ingress-classes",
		[]string{}, "The ingress classes offered by the remote cluster. The first one will be used as default")
	cmd.Flags().StringSliceVar(&o.loadBalancerClasses, "load-balancer-classes",
		[]string{}, "The load balancer classes offered by the remote cluster. The first one will be used as default")
	cmd.Flags().StringToStringVar(&o.labels, "labels", map[string]string{}, "The labels to be added to the virtual node")

	runtime.Must(cmd.MarkFlagRequired("cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("cluster-name"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("cluster-name", completion.ClusterNames(ctx,
		o.createOptions.Factory, completion.NoLimit)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("kubeconfig-secret-name", completion.KubeconfigSecretNames(ctx,
		o.createOptions.Factory, completion.NoLimit, options.Namespace, authv1alpha1.ResourceSliceIdentityType)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("resource-slice-name", completion.ResourceSliceNames(ctx,
		o.createOptions.Factory, completion.NoLimit, options.Namespace)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	var err error
	var vnOpts *forge.VirtualNodeOptions
	var tenantNamespace string

	opts := o.createOptions

	// Get the tenant namespace.
	tenantNamespace, err = o.getTenantNamespace(ctx)
	if err != nil {
		opts.Printer.CheckErr(err)
		return err
	}

	// Check that exactly one between kubeconfigSecretName and resourceSliceName is set and forge Virtual Node options respectively
	// using command-line flags or the retrieved resourceslice.
	switch {
	case o.kubeconfigSecretName != "" && o.resourceSliceName != "":
		err = fmt.Errorf("only one of --kubeconfig-secret-name and --resource-slice-name can be specified at the same time")
	case o.resourceSliceName != "":
		vnOpts, err = o.forgeVirtualNodeOptionsFromResourceSlice(ctx, opts.CRClient, tenantNamespace)
	case o.kubeconfigSecretName != "":
		vnOpts, err = o.forgeVirtualNodeOptions()
	default:
		err = fmt.Errorf("exactly one of --kubeconfig-secret-name and --resource-slice-name must be specified")
	}
	if err != nil {
		opts.Printer.CheckErr(err)
		return err
	}

	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output(opts.Name, tenantNamespace, vnOpts))
		return nil
	}

	s := opts.Printer.StartSpinner("Creating virtual node")

	virtualNode := forge.VirtualNode(opts.Name, tenantNamespace)
	if _, err := controllerutil.CreateOrUpdate(ctx, opts.CRClient, virtualNode, func() error {
		return forge.MutateVirtualNode(virtualNode, &o.remoteClusterIdentity, vnOpts)
	}); err != nil {
		s.Fail("Unable to create virtual node: %v", output.PrettyErr(err))
		return err
	}
	s.Success("Virtual node created")

	if virtualNode.Spec.CreateNode != nil && *virtualNode.Spec.CreateNode {
		waiter := wait.NewWaiterFromFactory(opts.Factory)
		// TODO: we cannot use the cluster identity here. Review in offloading refactor.
		if err := waiter.ForNode(ctx, virtualNode.Spec.ClusterIdentity); err != nil {
			opts.Printer.CheckErr(err)
			return err
		}
	}

	return nil
}

func (o *Options) forgeVirtualNodeOptionsFromResourceSlice(ctx context.Context,
	cl client.Client, tenantNamespace string) (*forge.VirtualNodeOptions, error) {
	// Get the associated ResourceSlice.
	var resourceSlice authv1alpha1.ResourceSlice
	if err := cl.Get(ctx, client.ObjectKey{Name: o.resourceSliceName, Namespace: tenantNamespace}, &resourceSlice); err != nil {
		return nil, fmt.Errorf("unable to get resourceslice %q: %w", o.resourceSliceName, err)
	}

	// Get the associated Identity for the remote cluster.
	identity, err := getters.GetIdentityFromResourceSlice(ctx, cl, o.remoteClusterIdentity.ClusterID, o.resourceSliceName)
	if err != nil {
		return nil, fmt.Errorf("unable to get the Identity associated to resourceslice %q: %w", o.resourceSliceName, err)
	}

	// Get associated secret
	kubeconfigSecret, err := getters.GetKubeconfigSecretFromIdentity(ctx, cl, identity)
	if err != nil {
		return nil, fmt.Errorf("unable to get the kubeconfig secret from identity %q: %w", identity.Name, err)
	}

	// Forge the VirtualNodeOptions from the ResourceSlice.
	vnOpts := forge.VirtualNodeOptionsFromResourceSlice(&resourceSlice, kubeconfigSecret.Name)

	return vnOpts, nil
}

func (o *Options) forgeVirtualNodeOptions() (*forge.VirtualNodeOptions, error) {
	cpuQnt, err := resource.ParseQuantity(o.cpu)
	if err != nil {
		return nil, fmt.Errorf("unable to parse cpu quantity: %w", err)
	}
	memoryQnt, err := resource.ParseQuantity(o.memory)
	if err != nil {
		return nil, fmt.Errorf("unable to parse memory quantity: %w", err)
	}
	podsQnt, err := resource.ParseQuantity(o.pods)
	if err != nil {
		return nil, fmt.Errorf("unable to parse pod quantity: %w", err)
	}

	storageClasses := make([]sharingv1alpha1.StorageType, len(o.storageClasses))
	for i, storageClass := range o.storageClasses {
		sc := sharingv1alpha1.StorageType{
			StorageClassName: storageClass,
		}
		if i == 0 {
			sc.Default = true
		}
		storageClasses[i] = sc
	}

	ingressClasses := make([]sharingv1alpha1.IngressType, len(o.ingressClasses))
	for i, ingressClass := range o.ingressClasses {
		ic := sharingv1alpha1.IngressType{
			IngressClassName: ingressClass,
		}
		if i == 0 {
			ic.Default = true
		}
		ingressClasses[i] = ic
	}

	loadBalancerClasses := make([]sharingv1alpha1.LoadBalancerType, len(o.loadBalancerClasses))
	for i, loadBalancerClass := range o.loadBalancerClasses {
		lbc := sharingv1alpha1.LoadBalancerType{
			LoadBalancerClassName: loadBalancerClass,
		}
		if i == 0 {
			lbc.Default = true
		}
		loadBalancerClasses[i] = lbc
	}

	return &forge.VirtualNodeOptions{
		KubeconfigSecretRef: corev1.LocalObjectReference{Name: o.kubeconfigSecretName},
		CreateNode:          o.createNode,

		ResourceList: corev1.ResourceList{
			corev1.ResourceCPU:    cpuQnt,
			corev1.ResourceMemory: memoryQnt,
			corev1.ResourcePods:   podsQnt,
		},
		StorageClasses:      storageClasses,
		IngressClasses:      ingressClasses,
		LoadBalancerClasses: loadBalancerClasses,
		NodeLabels:          o.labels,
	}, nil
}

func (o *Options) getTenantNamespace(ctx context.Context) (string, error) {
	ns, err := o.namespaceManager.GetNamespace(ctx, discoveryv1alpha1.ClusterIdentity{ClusterID: o.remoteClusterIdentity.ClusterID})
	switch {
	case err == nil:
		return ns.Name, nil
	case apierrors.IsNotFound(err):
		return "", fmt.Errorf("tenant namespace not found for cluster %q", o.remoteClusterIdentity.ClusterID)
	default:
		return "", err
	}
}

// output implements the logic to output the generated VirtualNode resource.
func (o *Options) output(name, namespace string, vnOpts *forge.VirtualNodeOptions) error {
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

	virtualNode := forge.VirtualNode(name, namespace)
	if err := forge.MutateVirtualNode(virtualNode, &o.remoteClusterIdentity, vnOpts); err != nil {
		return err
	}

	return printer.PrintObj(virtualNode, os.Stdout)
}

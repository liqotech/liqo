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
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

const liqoctlCreateVirtualNodeLongHelp = `Create a VirtualNode.

The VirtualNode resource is used to represent a remote cluster in the local cluster.

Examples:
  $ {{ .Executable }} create virtualnode my-cluster --cluster-id remote-cluster-id \
  --kubeconfig-secret-name my-cluster-kubeconfig
  Or, if creating a VirtualNode from a ResourceSlice:
  $ {{ .Executable }} create virtualnode my-cluster --cluster-id remote-cluster-id \
  --resource-slice-name my-resourceslice`

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

		PreRun: func(_ *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			options.Name = args[0]
			o.createOptions = options

			o.namespaceManager = tenantnamespace.NewManager(options.KubeClient, options.CRClient.Scheme())
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleCreate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting VirtualNode resource, instead of applying it. Supported formats: json, yaml")

	// TODO: check validity of cluster-id
	cmd.Flags().Var(&o.remoteClusterID, "remote-cluster-id", "The cluster ID of the remote cluster")
	cmd.Flags().BoolVar(&o.createNode, "create-node",
		true, "Create a node to target the remote cluster (and schedule on it)")
	cmd.Flags().BoolVar(&o.disableNetworkCheck, "disable-network-check", false, "Disable the network status check")
	cmd.Flags().StringVar(&o.kubeconfigSecretName, "kubeconfig-secret-name",
		"", "The name of the secret containing the kubeconfig of the remote cluster. Mutually exclusive with --resource-slice-name")
	cmd.Flags().StringVar(&o.resourceSliceName, "resource-slice-name",
		"", "The name of the resourceslice to be used to create the virtual node. Mutually exclusive with --kubeconfig-secret-name")
	cmd.Flags().StringVar(&o.vkOptionsTemplate, "vk-options-template",
		"", "Namespaced name of the virtual-kubelet options template. Leave empty to use the default template installed with Liqo")
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
	cmd.Flags().StringToStringVar(&o.nodeSelector, "node-selector", map[string]string{}, "The node selector to be applied to offloaded pods")
	cmd.Flags().StringVar(&o.runtimeClassName, "runtime-class-name", "", "The runtimeClass the pods should have on the target remote cluster")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("kubeconfig-secret-name", completion.KubeconfigSecretNames(ctx,
		o.createOptions.Factory, completion.NoLimit, options.Namespace, authv1beta1.ResourceSliceIdentityType)))
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

	var vkOptionsTemplateRef *corev1.ObjectReference
	if o.vkOptionsTemplate != "" {
		vkOptionsTemplateRef, err = args.GetObjectRefFromNamespacedName(o.vkOptionsTemplate)
		if err != nil {
			opts.Printer.CheckErr(fmt.Errorf("--vk-options-template is not a valid namespaced name: %w", err))
			return err
		}
	}

	// Check that exactly one between kubeconfigSecretName and resourceSliceName is set and forge Virtual Node options respectively
	// using command-line flags or the retrieved resourceslice.
	switch {
	case o.kubeconfigSecretName != "" && o.resourceSliceName != "":
		err = fmt.Errorf("only one of --kubeconfig-secret-name and --resource-slice-name can be specified at the same time")
	case o.resourceSliceName != "":
		vnOpts, err = o.forgeVirtualNodeOptionsFromResourceSlice(ctx, opts.CRClient, tenantNamespace, vkOptionsTemplateRef)
	case o.kubeconfigSecretName != "":
		vnOpts, err = o.forgeVirtualNodeOptions(vkOptionsTemplateRef)
	default:
		err = fmt.Errorf("exactly one of --kubeconfig-secret-name and --resource-slice-name must be specified")
	}
	if err != nil {
		opts.Printer.CheckErr(err)
		return err
	}

	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output(ctx, opts.Name, tenantNamespace, vnOpts))
		return nil
	}

	s := opts.Printer.StartSpinner("Creating virtual node")

	virtualNode := forge.VirtualNode(opts.Name, tenantNamespace)
	if _, err := resource.CreateOrUpdate(ctx, opts.CRClient, virtualNode, func() error {
		return forge.MutateVirtualNode(ctx, opts.CRClient,
			virtualNode, o.remoteClusterID.GetClusterID(), vnOpts, &o.createNode, &o.disableNetworkCheck, &o.runtimeClassName)
	}); err != nil {
		s.Fail("Unable to create virtual node: ", output.PrettyErr(err))
		return err
	}
	s.Success("Virtual node created")

	if virtualNode.Spec.CreateNode != nil && *virtualNode.Spec.CreateNode {
		waiter := wait.NewWaiterFromFactory(opts.Factory)
		if err := waiter.ForNodeReady(ctx, virtualNode.Name); err != nil {
			opts.Printer.CheckErr(err)
			return err
		}
	}

	return nil
}

func (o *Options) forgeVirtualNodeOptionsFromResourceSlice(ctx context.Context,
	cl client.Client, tenantNamespace string, vkOptionsTemplateRef *corev1.ObjectReference) (*forge.VirtualNodeOptions, error) {
	// Get the associated ResourceSlice.
	var resourceSlice authv1beta1.ResourceSlice
	if err := cl.Get(ctx, client.ObjectKey{Name: o.resourceSliceName, Namespace: tenantNamespace}, &resourceSlice); err != nil {
		return nil, fmt.Errorf("unable to get resourceslice %q: %w", o.resourceSliceName, err)
	}

	// Get the associated Identity for the remote cluster.
	identity, err := getters.GetIdentityFromResourceSlice(ctx, cl, o.remoteClusterID.GetClusterID(), o.resourceSliceName)
	if err != nil {
		return nil, fmt.Errorf("unable to get the Identity associated to resourceslice %q: %w", o.resourceSliceName, err)
	}

	// Get associated secret
	kubeconfigSecret, err := getters.GetKubeconfigSecretFromIdentity(ctx, cl, identity)
	if err != nil {
		return nil, fmt.Errorf("unable to get the kubeconfig secret from identity %q: %w", identity.Name, err)
	}

	// Forge the VirtualNodeOptions from the ResourceSlice.
	vnOpts := forge.VirtualNodeOptionsFromResourceSlice(&resourceSlice, kubeconfigSecret.Name, vkOptionsTemplateRef)

	return vnOpts, nil
}

func (o *Options) forgeVirtualNodeOptions(vkOptionsTemplateRef *corev1.ObjectReference) (*forge.VirtualNodeOptions, error) {
	cpuQnt, err := k8sresource.ParseQuantity(o.cpu)
	if err != nil {
		return nil, fmt.Errorf("unable to parse cpu quantity: %w", err)
	}
	memoryQnt, err := k8sresource.ParseQuantity(o.memory)
	if err != nil {
		return nil, fmt.Errorf("unable to parse memory quantity: %w", err)
	}
	podsQnt, err := k8sresource.ParseQuantity(o.pods)
	if err != nil {
		return nil, fmt.Errorf("unable to parse pod quantity: %w", err)
	}

	storageClasses := make([]liqov1beta1.StorageType, len(o.storageClasses))
	for i, storageClass := range o.storageClasses {
		sc := liqov1beta1.StorageType{
			StorageClassName: storageClass,
		}
		if i == 0 {
			sc.Default = true
		}
		storageClasses[i] = sc
	}

	ingressClasses := make([]liqov1beta1.IngressType, len(o.ingressClasses))
	for i, ingressClass := range o.ingressClasses {
		ic := liqov1beta1.IngressType{
			IngressClassName: ingressClass,
		}
		if i == 0 {
			ic.Default = true
		}
		ingressClasses[i] = ic
	}

	loadBalancerClasses := make([]liqov1beta1.LoadBalancerType, len(o.loadBalancerClasses))
	for i, loadBalancerClass := range o.loadBalancerClasses {
		lbc := liqov1beta1.LoadBalancerType{
			LoadBalancerClassName: loadBalancerClass,
		}
		if i == 0 {
			lbc.Default = true
		}
		loadBalancerClasses[i] = lbc
	}

	return &forge.VirtualNodeOptions{
		KubeconfigSecretRef:  corev1.LocalObjectReference{Name: o.kubeconfigSecretName},
		VkOptionsTemplateRef: vkOptionsTemplateRef,

		ResourceList: corev1.ResourceList{
			corev1.ResourceCPU:    cpuQnt,
			corev1.ResourceMemory: memoryQnt,
			corev1.ResourcePods:   podsQnt,
		},
		StorageClasses:      storageClasses,
		IngressClasses:      ingressClasses,
		LoadBalancerClasses: loadBalancerClasses,
		NodeLabels:          o.labels,
		NodeSelector:        o.nodeSelector,
	}, nil
}

func (o *Options) getTenantNamespace(ctx context.Context) (string, error) {
	ns, err := o.namespaceManager.GetNamespace(ctx, o.remoteClusterID.GetClusterID())
	switch {
	case err == nil:
		return ns.Name, nil
	case apierrors.IsNotFound(err):
		return "", fmt.Errorf("tenant namespace not found for cluster %q", o.remoteClusterID.GetClusterID())
	default:
		return "", err
	}
}

// output implements the logic to output the generated VirtualNode resource.
func (o *Options) output(ctx context.Context, name, namespace string, vnOpts *forge.VirtualNodeOptions) error {
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
	if err := forge.MutateVirtualNode(ctx, opts.CRClient,
		virtualNode, o.remoteClusterID.GetClusterID(), vnOpts, &o.createNode, &o.disableNetworkCheck, &o.runtimeClassName); err != nil {
		return err
	}

	return printer.PrintObj(virtualNode, os.Stdout)
}

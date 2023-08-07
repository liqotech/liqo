// Copyright 2019-2023 The Liqo Authors
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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	"github.com/liqotech/liqo/pkg/utils/args"
)

const liqoctlCreateVirtualNodeLongHelp = `Create a VirtualNode.

The VirtualNode resource is used to represent a remote cluster in the local cluster.

Examples:
  $ {{ .Executable }} create virtualnode my-cluster --cluster-id my-cluster-id \
  --cluster-name my-cluster-name --kubeconfig-secret-name my-cluster-kubeconfig --namespace my-cluster`

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
		"", "The name of the secret containing the kubeconfig of the remote cluster")
	cmd.Flags().StringVar(&o.cpu, "cpu", "2", "The amount of CPU available in the virtual node")
	cmd.Flags().StringVar(&o.memory, "memory", "4Gi", "The amount of memory available in the virtual node")
	cmd.Flags().StringVar(&o.pods, "pods", "110", "The amount of pods available in the virtual node")
	cmd.Flags().StringSliceVar(&o.storageClasses, "storage-classes",
		[]string{}, "The storage classes offered by the remote cluster. The first one will be used as default")
	cmd.Flags().StringToStringVar(&o.labels, "labels", map[string]string{}, "The labels to be added to the virtual node")

	runtime.Must(cmd.MarkFlagRequired("cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("cluster-name"))
	runtime.Must(cmd.MarkFlagRequired("kubeconfig-secret-name"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("cluster-name", completion.ClusterNames(ctx,
		o.createOptions.Factory, completion.NoLimit)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("kubeconfig-secret-name", completion.KubeconfigSecretNames(ctx,
		o.createOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	opts := o.createOptions
	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output())
		return nil
	}

	s := opts.Printer.StartSpinner("Creating virtual node")

	virtualNode := o.forgeVirtualNode()
	_, err := controllerutil.CreateOrUpdate(ctx, opts.CRClient, virtualNode, func() error {
		return o.mutateVirtualNode(virtualNode)
	})
	if err != nil {
		s.Fail("Unable to create virtual node: %v", output.PrettyErr(err))
		return err
	}
	s.Success("Virtual node created")

	if virtualNode.Spec.CreateNode != nil && *virtualNode.Spec.CreateNode {
		waiter := wait.NewWaiterFromFactory(opts.Factory)
		// TODO: we cannot use the cluster identity here
		if err := waiter.ForNode(ctx, virtualNode.Spec.ClusterIdentity); err != nil {
			return err
		}
	}

	return nil
}

func (o *Options) forgeVirtualNode() *virtualkubeletv1alpha1.VirtualNode {
	opts := o.createOptions
	return &virtualkubeletv1alpha1.VirtualNode{
		TypeMeta: metav1.TypeMeta{
			APIVersion: virtualkubeletv1alpha1.VirtualNodeGroupVersionResource.GroupVersion().String(),
			Kind:       virtualkubeletv1alpha1.VirtualNodeKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
	}
}

func (o *Options) mutateVirtualNode(virtualNode *virtualkubeletv1alpha1.VirtualNode) error {
	virtualNode.Spec.ClusterIdentity = &o.remoteClusterIdentity
	virtualNode.Spec.CreateNode = &o.createNode
	virtualNode.Spec.KubeconfigSecretRef = &corev1.LocalObjectReference{
		Name: o.kubeconfigSecretName,
	}

	cpuQnt, err := resource.ParseQuantity(o.cpu)
	if err != nil {
		return fmt.Errorf("unable to parse cpu quantity: %w", err)
	}
	memoryQnt, err := resource.ParseQuantity(o.memory)
	if err != nil {
		return fmt.Errorf("unable to parse memory quantity: %w", err)
	}
	podsQnt, err := resource.ParseQuantity(o.pods)
	if err != nil {
		return fmt.Errorf("unable to parse pod quantity: %w", err)
	}
	virtualNode.Spec.ResourceQuota = corev1.ResourceQuotaSpec{
		Hard: corev1.ResourceList{
			corev1.ResourceCPU:    cpuQnt,
			corev1.ResourceMemory: memoryQnt,
			corev1.ResourcePods:   podsQnt,
		},
	}

	virtualNode.Spec.StorageClasses = make([]sharingv1alpha1.StorageType, len(o.storageClasses))
	for i, storageClass := range o.storageClasses {
		sc := sharingv1alpha1.StorageType{
			StorageClassName: storageClass,
		}
		if i == 0 {
			sc.Default = true
		}
		virtualNode.Spec.StorageClasses[i] = sc
	}

	if virtualNode.ObjectMeta.Labels == nil {
		virtualNode.ObjectMeta.Labels = make(map[string]string)
	}
	virtualNode.ObjectMeta.Labels[discovery.ClusterIDLabel] = o.remoteClusterIdentity.ClusterID
	virtualNode.Spec.Labels = o.labels
	virtualNode.Spec.Labels[discovery.ClusterIDLabel] = o.remoteClusterIdentity.ClusterID

	return nil
}

// output implements the logic to output the generated VirtualNode resource.
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

	virtualNode := o.forgeVirtualNode()
	if err := o.mutateVirtualNode(virtualNode); err != nil {
		return err
	}

	return printer.PrintObj(virtualNode, os.Stdout)
}

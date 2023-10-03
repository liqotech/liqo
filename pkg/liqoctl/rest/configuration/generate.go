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

package configuration

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/args"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
)

const liqoctlGenerateConfigHelp = `Generate the local network configuration to be applied to other clusters.`

// Generate generates a Configuration.
func (o *Options) Generate(ctx context.Context, options *rest.GenerateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "yaml")

	o.generateOptions = options

	cmd := &cobra.Command{
		Use:     "configuration",
		Aliases: []string{"config", "configurations"},
		Short:   "Generate a Configuration",
		Long:    liqoctlGenerateConfigHelp,
		Args:    cobra.NoArgs,

		PreRun: func(cmd *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			o.generateOptions = options
		},

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(o.handleGenerate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output format of the resulting Configuration resource. Supported formats: json, yaml")

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))

	return cmd
}

func (o *Options) handleGenerate(ctx context.Context) error {
	opts := o.generateOptions

	conf, err := ForgeLocalConfiguration(ctx, opts.CRClient, opts.Namespace, opts.LiqoNamespace)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to forge local configuration: %w", err))
		return err
	}

	opts.Printer.CheckErr(o.output(conf))
	return nil
}

// ForgeLocalConfiguration creates a local configuration starting from the cluster identity and the IPAM storage.
func ForgeLocalConfiguration(ctx context.Context, cl client.Client, namespace, liqoNamespace string) (*networkingv1alpha1.Configuration, error) {
	clusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, cl, liqoNamespace)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster identity: %w", err)
	}

	ipamStorage, err := liqogetters.GetIPAMStorageByLabel(ctx, cl, labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("unable to get IPAM storage: %w", err)
	}

	cnf := &networkingv1alpha1.Configuration{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1alpha1.ConfigurationKind,
			APIVersion: networkingv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterIdentity.ClusterName,
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: clusterIdentity.ClusterID,
			},
		},
		Spec: networkingv1alpha1.ConfigurationSpec{
			Remote: networkingv1alpha1.ClusterConfig{
				CIDR: networkingv1alpha1.ClusterConfigCIDR{
					Pod:      networkingv1alpha1.CIDR(ipamStorage.Spec.PodCIDR),
					External: networkingv1alpha1.CIDR(ipamStorage.Spec.ExternalCIDR),
				},
			},
		},
	}

	if namespace != "" && namespace != corev1.NamespaceDefault {
		cnf.Namespace = namespace
	}
	return cnf, nil
}

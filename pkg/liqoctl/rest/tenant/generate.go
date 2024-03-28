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

package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	noncesigner "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/nonce-signer"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/args"
)

const liqoctlGenerateConfigHelp = `Generate the Tenant resource to be applied on the remote provider cluster.`

// Generate generates a Tenant.
func (o *Options) Generate(ctx context.Context, options *rest.GenerateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "yaml")

	o.generateOptions = options

	cmd := &cobra.Command{
		Use:     "tenant",
		Aliases: []string{"tenants"},
		Short:   "Generate a Tenant",
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
		"Output format of the resulting Tenant resource. Supported formats: json, yaml")

	cmd.Flags().StringVar(&o.remoteClusterIdentity.ClusterID, "remote-cluster-id", "", "The ID of the remote cluster")
	cmd.Flags().StringVar(&o.remoteClusterIdentity.ClusterName, "remote-cluster-name", "", "The name of the remote cluster")
	cmd.Flags().StringVar(&o.nonce, "nonce", "", "The nonce to sign for the authentication with the remote cluster")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx, o.generateOptions.Factory, completion.NoLimit)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-name", completion.ClusterNames(ctx, o.generateOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleGenerate(ctx context.Context) error {
	var tenantNs *corev1.Namespace
	var nonceSecret *corev1.Secret
	var signedNonce []byte
	var err error

	opts := o.generateOptions
	namespaceManager := tenantnamespace.NewManager(opts.Factory.KubeClient)

	// Ensure tenant namespace exists
	if tenantNs, err = namespaceManager.CreateNamespace(ctx, o.remoteClusterIdentity); err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to create tenant namespace: %w", err))
		return err
	}

	// If nonce is not provided, get it from the secret in the tenant namespace and raise an error if the secret does not exist.
	// If nonce is provided, create nonce secret in the tenant namespace and wait for it to be signed. Raise an error if there is
	// already a nonce secret in the tenant namespace.
	nonceSecret, err = noncesigner.GetSecretNonceConsumer(ctx, opts.CRClient, tenantNs.GetName(), o.remoteClusterIdentity.ClusterID)
	switch {
	case errors.IsNotFound(err):
		// Secret not found. Create it given the provided nonce.
		if o.nonce == "" {
			opts.Printer.CheckErr(fmt.Errorf("nonce not provided and no nonce secret found"))
			return err
		}
		nonceSecret, err = noncesigner.CreateSecretNonceConsumer(ctx, opts.CRClient, tenantNs.GetName(), o.nonce, &o.remoteClusterIdentity)
		if err != nil {
			opts.Printer.CheckErr(fmt.Errorf("unable to create nonce secret: %w", err))
			return err
		}
	case err != nil:
		opts.Printer.CheckErr(fmt.Errorf("unable to get nonce secret: %w", err))
		return err
	default:
		// Secret already exists. Check if the nonce is the same.
		nonce, err := noncesigner.GetNonceFromSecret(nonceSecret)
		if err != nil {
			opts.Printer.CheckErr(fmt.Errorf("unable to extract nonce data from secret %s/%s: %w", nonceSecret.Namespace, nonceSecret.Name, err))
			return err
		}
		if string(nonce) != o.nonce {
			opts.Printer.CheckErr(fmt.Errorf("nonce secret already exists with a different nonce: %s", nonce))
			return err
		}
	}

	// Wait for the nonce to be signed.
	s := opts.Printer.StartSpinner("Waiting for nonce to be signed")
	interval := 1 * time.Second
	if err := wait.PollUntilContextCancel(ctx, interval, false, func(context.Context) (done bool, err error) {
		if nonceSecret, err = noncesigner.GetSecretNonceConsumer(ctx, opts.CRClient, tenantNs.GetName(), o.remoteClusterIdentity.ClusterID); err != nil {
			return false, err
		}
		if signedNonce, err = noncesigner.GetSignedNonceFromSecret(nonceSecret); err != nil {
			return false, err
		}
		return true, nil
	}); err != nil {
		s.Fail("Unable to wait for nonce to be signed: %v", output.PrettyErr(err))
		return err
	}

	// Get the local cluster identity.
	localClusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, opts.CRClient, opts.LiqoNamespace)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while retrieving cluster identity: %v", output.PrettyErr(err)))
		return err
	}

	// Get public key of the local cluster.
	privateKey, publicKey, err := authentication.GetClusterKeys(ctx, opts.CRClient, opts.LiqoNamespace)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to get cluster keys: %w", err))
		return err
	}

	// Generate a CSR for the remote cluster.
	CSR, err := authentication.GenerateCSR(privateKey, authentication.CommonName(localClusterIdentity))
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to generate CSR: %w", err))
		return err
	}

	tenant, err := forge.ForgeTenantForRemoteCluster(ctx, localClusterIdentity, publicKey, CSR, string(signedNonce))
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to forge tenant: %w", err))
		return err
	}

	opts.Printer.CheckErr(o.output(tenant))
	return nil
}

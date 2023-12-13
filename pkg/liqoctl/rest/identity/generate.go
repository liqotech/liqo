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

package identity

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	"github.com/liqotech/liqo/pkg/utils/args"
	csrutil "github.com/liqotech/liqo/pkg/utils/csr"
)

const liqoctlGenerateIdentityHelp = `Generate a local identity to be used by a remote cluster.`

// Generate generates an Identity.
func (o *Options) Generate(ctx context.Context, options *rest.GenerateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{commandLabel, jsonLabel, yamlLabel}, commandLabel)

	o.generateOptions = options

	cmd := &cobra.Command{
		Use:     "identity",
		Aliases: []string{"identities"},
		Short:   "Generate an Identity",
		Long:    liqoctlGenerateIdentityHelp,
		Args:    cobra.NoArgs,

		PreRun: func(cmd *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			o.generateOptions = options
		},

		Run: func(cmd *cobra.Command, args []string) {
			err := o.handleGenerate(ctx)
			if err != nil {
				o.generateOptions.Printer.ExitWithMessage(output.PrettyErr(err))
			}
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output format of the resulting Identity resource. Supported formats: command, json, yaml")
	cmd.Flags().StringVar(&o.remoteClusterID, "remote-cluster-id", "",
		"Cluster ID of the remote cluster.")
	cmd.Flags().StringVar(&o.remoteClusterName, "remote-cluster-name", "",
		"Cluster name of the remote cluster.")

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("remote-cluster-name"))

	return cmd
}

func (o *Options) handleGenerate(ctx context.Context) error {
	opts := o.generateOptions

	// generate request

	localClusterIdentity, err := utils.GetClusterIdentityWithControllerClient(ctx, opts.CRClient, opts.LiqoNamespace)
	if err != nil {
		return err
	}

	remoteClusterIdentity := discoveryv1alpha1.ClusterIdentity{
		ClusterID:   o.remoteClusterID,
		ClusterName: o.remoteClusterName,
	}

	key, csr, err := csrutil.NewKeyAndRequest(remoteClusterIdentity.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to create create identity: %w", err)
	}

	identityRequest := auth.NewCertificateIdentityRequest(remoteClusterIdentity, "", "", csr)

	// handle request

	namespaceManager := tenantnamespace.NewCachedManager(ctx, opts.KubeClient)
	// Note: AWS identity provider is not supported since no certificate is provided
	identityProvider := identitymanager.NewCertificateIdentityProvider(
		ctx, opts.KubeClient, remoteClusterIdentity, namespaceManager)
	idManager := identitymanager.NewCertificateIdentityManager(opts.KubeClient, remoteClusterIdentity, namespaceManager)

	namespace, err := namespaceManager.CreateNamespace(ctx, remoteClusterIdentity)
	if err != nil {
		return err
	}

	// issue certificate request
	identityResponse, err := identityProvider.ApproveSigningRequest(
		remoteClusterIdentity, identityRequest.CertificateSigningRequest)
	if err != nil {
		return err
	}

	peeringPermission, err := peeringroles.GetPeeringPermission(ctx, opts.KubeClient)
	if err != nil {
		return err
	}

	// bind basic permission required to start the peering
	if _, err = namespaceManager.BindClusterRoles(
		ctx, remoteClusterIdentity, peeringPermission.Basic...); err != nil {
		return err
	}

	var apiServerConfig apiserver.Config
	// make the response to send to the remote cluster
	response, err := auth.NewCertificateIdentityResponse(namespace.Name, identityResponse, apiServerConfig)
	if err != nil {
		return err
	}

	// store the identity in a secret
	secret, err := idManager.GenerateIdentitySecret(remoteClusterIdentity, "", key, opts.Namespace, response)
	if err != nil {
		return err
	}

	switch opts.OutputFormat {
	case jsonLabel, yamlLabel:
		opts.Printer.CheckErr(o.output(secret))
	case commandLabel:
		certificate := base64.StdEncoding.EncodeToString(secret.Data["certificate"])
		privateKey := base64.StdEncoding.EncodeToString(secret.Data["private-key"])

		command := strings.Join([]string{
			o.generateOptions.Liqoctl, "update identity",
			"--remote-cluster-id", localClusterIdentity.ClusterID,
			"--remote-cluster-name", localClusterIdentity.ClusterName,
			"--certificate", certificate,
			"--private-key", privateKey,
		}, " ")
		fmt.Println(command)
	}
	return nil
}

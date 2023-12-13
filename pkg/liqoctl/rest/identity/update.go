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

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
)

const liqoctlUpdateIdentityHelp = `Generate a local identity to be used by a remote cluster.`

// Update implements the update command.
func (o *Options) Update(ctx context.Context, options *rest.UpdateOptions) *cobra.Command {
	o.updateOptions = options

	cmd := &cobra.Command{
		Use:     "identity",
		Aliases: []string{"identities"},
		Short:   "Update an Identity",
		Long:    liqoctlUpdateIdentityHelp,
		Args:    cobra.NoArgs,

		PreRun: func(cmd *cobra.Command, args []string) {
			o.updateOptions = options
		},

		Run: func(cmd *cobra.Command, args []string) {
			err := o.handleUpdate(ctx)
			if err != nil {
				o.updateOptions.Printer.ExitWithMessage(output.PrettyErr(err))
			}
		},
	}

	cmd.Flags().StringVar(&o.RemoteClusterIdentity.ClusterID, "remote-cluster-id", "", "The cluster ID of the remote cluster")
	cmd.Flags().StringVar(&o.RemoteClusterIdentity.ClusterName, "remote-cluster-name", "", "The cluster name of the remote cluster")
	cmd.Flags().StringVar(&o.CertificateString, "certificate", "", "The certificate of the remote cluster")
	cmd.Flags().StringVar(&o.PrivateKeyString, "private-key", "", "The private key for the remote cluster")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("remote-cluster-name"))
	runtime.Must(cmd.MarkFlagRequired("certificate"))
	runtime.Must(cmd.MarkFlagRequired("private-key"))

	return cmd
}

func (o *Options) handleUpdate(ctx context.Context) error {
	opts := o.updateOptions

	localClusterIdentity, err := utils.GetClusterIdentityWithControllerClient(ctx, opts.CRClient, opts.LiqoNamespace)
	if err != nil {
		return err
	}

	namespaceManager := tenantnamespace.NewCachedManager(ctx, opts.KubeClient)
	idManager := identitymanager.NewCertificateIdentityManager(opts.KubeClient, localClusterIdentity, namespaceManager)

	secret, err := idManager.GetSecret(o.RemoteClusterIdentity)
	if err != nil {
		return err
	}

	certificate, err := base64.StdEncoding.DecodeString(o.CertificateString)
	if err != nil {
		return fmt.Errorf("failed to decode certificate: %w", err)
	}
	secret.Data["certificate"] = certificate

	privateKey, err := base64.StdEncoding.DecodeString(o.PrivateKeyString)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}
	secret.Data["private-key"] = privateKey

	_, err = opts.KubeClient.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	deployments, err := getDeploymentsToBeRestarted(ctx, opts.CRClient, opts.LiqoNamespace, secret.Namespace)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		if err := utils.RestartDeployment(ctx, opts.CRClient, deployment); err != nil {
			return err
		}
	}

	return nil
}

func getDeploymentsToBeRestarted(ctx context.Context, cl client.Client,
	liqoNamespace, tenantNamespace string) ([]*appsv1.Deployment, error) {
	var deployments appsv1.DeploymentList
	var deploymentsToBeRestarted []*appsv1.Deployment

	// controller manager
	if err := cl.List(ctx, &deployments, client.InNamespace(liqoNamespace), client.MatchingLabels{
		"app.kubernetes.io/name": "controller-manager",
	}); err != nil {
		return nil, err
	}
	for i := range deployments.Items {
		deploymentsToBeRestarted = append(deploymentsToBeRestarted, &deployments.Items[i])
	}

	// crd replicator
	if err := cl.List(ctx, &deployments, client.InNamespace(liqoNamespace), client.MatchingLabels{
		"app.kubernetes.io/name": "crd-replicator",
	}); err != nil {
		return nil, err
	}
	for i := range deployments.Items {
		deploymentsToBeRestarted = append(deploymentsToBeRestarted, &deployments.Items[i])
	}

	// virtual kubelet
	if err := cl.List(ctx, &deployments, client.InNamespace(tenantNamespace), client.MatchingLabels{
		"app.kubernetes.io/name": "virtual-kubelet",
	}); err != nil {
		return nil, err
	}
	for i := range deployments.Items {
		deploymentsToBeRestarted = append(deploymentsToBeRestarted, &deployments.Items[i])
	}

	return deploymentsToBeRestarted, nil
}

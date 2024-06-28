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

package authenticate

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	authutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/utils"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
)

// Cluster contains the information about a cluster.
type Cluster struct {
	local  *factory.Factory
	waiter *wait.Waiter

	localNamespaceManager tenantnamespace.Manager

	LocalClusterID  discoveryv1alpha1.ClusterID
	RemoteClusterID discoveryv1alpha1.ClusterID

	TenantNamespace string
}

// NewCluster returns a new Cluster struct.
func NewCluster(local *factory.Factory) *Cluster {
	return &Cluster{
		local:  local,
		waiter: wait.NewWaiterFromFactory(local),

		localNamespaceManager: tenantnamespace.NewManager(local.KubeClient),
	}
}

// SetLocalClusterID set the local cluster id retrieving it from the Liqo configmaps.
func (c *Cluster) SetLocalClusterID(ctx context.Context) error {
	// Get local cluster id.
	clusterID, err := liqoutils.GetClusterIDWithControllerClient(ctx, c.local.CRClient, c.local.LiqoNamespace)
	if err != nil {
		c.local.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving cluster id: %v", output.PrettyErr(err)))
		return err
	}
	c.LocalClusterID = clusterID

	return nil
}

// EnsureTenantNamespace ensure the presence of the tenant namespace on the local cluster given a remote cluster id.
func (c *Cluster) EnsureTenantNamespace(ctx context.Context, remoteClusterID discoveryv1alpha1.ClusterID) error {
	s := c.local.Printer.StartSpinner("Ensuring tenant namespace")

	c.RemoteClusterID = remoteClusterID

	if _, err := c.localNamespaceManager.CreateNamespace(ctx, c.RemoteClusterID); err != nil {
		s.Fail(fmt.Sprintf("An error occurred while ensuring tenant namespace: %v", output.PrettyErr(err)))
		return err
	}
	c.TenantNamespace = tenantnamespace.GetNameForNamespace(c.RemoteClusterID)

	s.Success("Tenant namespace correctly ensured")

	return nil
}

// EnsureNonce ensure the presence of a secret containing the nonce for the authentication challenge
// of a consumer cluster.
func (c *Cluster) EnsureNonce(ctx context.Context) ([]byte, error) {
	var err error

	// Ensure the presence of the nonce secret.
	s := c.local.Printer.StartSpinner("Ensuring nonce secret")
	if err := authutils.EnsureNonceSecret(ctx, c.local.CRClient, c.RemoteClusterID, c.TenantNamespace); err != nil {
		s.Fail(fmt.Sprintf("Unable to create nonce secret: %v", output.PrettyErr(err)))
		return nil, err
	}
	s.Success("Nonce secret ensured")

	// Wait for secret to be filled with the nonce.
	if err := c.waiter.ForNonce(ctx, c.RemoteClusterID, false); err != nil {
		return nil, err
	}

	// Retrieve nonce from secret.
	s = c.local.Printer.StartSpinner("Retrieving nonce")
	nonceValue, err := authutils.RetrieveNonce(ctx, c.local.CRClient, c.RemoteClusterID)
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to retrieve nonce: %v", output.PrettyErr(err)))
		return nil, err
	}
	s.Success("Nonce retrieved")

	return nonceValue, nil
}

// EnsureSignedNonce ensure the presence of a secret containing the signed nonce of the authentication challenge
// and return the signed nonce.
func (c *Cluster) EnsureSignedNonce(ctx context.Context, nonce []byte) ([]byte, error) {
	var err error

	// Ensure the presence of the signed nonce secret.
	s := c.local.Printer.StartSpinner("Ensuring signed nonce")
	err = authutils.EnsureSignedNonceSecret(ctx, c.local.CRClient, c.RemoteClusterID, c.TenantNamespace, ptr.To(string(nonce)))
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to ensure signed nonce secret: %v", err))
		return nil, err
	}
	s.Success("Signed nonce secret ensured")

	// Wait for secret to be filled with the signed nonce.
	if err := c.waiter.ForSignedNonce(ctx, c.RemoteClusterID, false); err != nil {
		return nil, err
	}

	// Retrieve signed nonce from secret.
	s = c.local.Printer.StartSpinner("Retrieving signed nonce")
	signedNonceValue, err := authutils.RetrieveSignedNonce(ctx, c.local.CRClient, c.RemoteClusterID)
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to retrieve signed nonce: %v", output.PrettyErr(err)))
		return nil, err
	}
	s.Success("Signed nonce retrieved")

	return signedNonceValue, nil
}

// GenerateTenant generate the tenant resource to be applied on the provider cluster.
func (c *Cluster) GenerateTenant(ctx context.Context, signedNonce []byte, proxyURL *string) (*authv1alpha1.Tenant, error) {
	s := c.local.Printer.StartSpinner("Generating tenant")
	tenant, err := authutils.GenerateTenant(ctx, c.local.CRClient, c.LocalClusterID, c.local.LiqoNamespace, signedNonce, proxyURL)
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to generate tenant: %v", output.PrettyErr(err)))
		return nil, err
	}
	s.Success("Tenant correctly generated")

	return tenant, nil
}

// EnsureTenant apply the tenant resource on the provider cluster and wait for the status to be updated.
func (c *Cluster) EnsureTenant(ctx context.Context, tenant *authv1alpha1.Tenant) error {
	s := c.local.Printer.StartSpinner("Applying tenant on provider cluster")
	if _, err := controllerutil.CreateOrUpdate(ctx, c.local.CRClient, tenant, func() error {
		return nil
	}); err != nil {
		s.Fail(fmt.Sprintf("Unable to apply tenant on provider cluster: %v", output.PrettyErr(err)))
		return err
	}
	s.Success("Tenant correctly applied on provider cluster")

	// Wait for the tenant status to be updated.
	if err := c.waiter.ForTenantStatus(ctx, c.RemoteClusterID); err != nil {
		return err
	}

	return nil
}

// GenerateIdentity generate the identity resource to be applied on the consumer cluster.
func (c *Cluster) GenerateIdentity(ctx context.Context, remoteTenantNamespace string) (*authv1alpha1.Identity, error) {
	s := c.local.Printer.StartSpinner("Generating identity")
	identity, err := authutils.GenerateIdentityControlPlane(ctx, c.local.CRClient,
		c.RemoteClusterID, remoteTenantNamespace, c.LocalClusterID)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while generating identity: %v", output.PrettyErr(err)))
		return nil, err
	}
	s.Success("Identity correctly generated")

	return identity, nil
}

// EnsureIdentity apply the identity resource on the consumer cluster and wait for the status to be updated.
func (c *Cluster) EnsureIdentity(ctx context.Context, identity *authv1alpha1.Identity) error {
	s := c.local.Printer.StartSpinner("Applying identity on consumer cluster")
	if _, err := controllerutil.CreateOrUpdate(ctx, c.local.CRClient, identity, func() error {
		return nil
	}); err != nil {
		s.Fail(fmt.Sprintf("Unable to apply identity on consumer cluster: %v", output.PrettyErr(err)))
		return err
	}
	s.Success("Identity correctly applied on consumer cluster")

	// Wait for the identity status to be updated.
	if err := c.waiter.ForIdentityStatus(ctx, c.RemoteClusterID); err != nil {
		return err
	}

	return nil
}

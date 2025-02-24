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

package unauthenticate

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// Cluster contains the information about a cluster.
type Cluster struct {
	local  *factory.Factory
	waiter *wait.Waiter

	tenantNamespaceManager tenantnamespace.Manager

	localClusterID liqov1beta1.ClusterID
}

// NewCluster returns a new Cluster struct.
func NewCluster(local *factory.Factory) *Cluster {
	return &Cluster{
		local:  local,
		waiter: wait.NewWaiterFromFactory(local),

		tenantNamespaceManager: tenantnamespace.NewManager(local.KubeClient, local.CRClient.Scheme()),
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
	c.localClusterID = clusterID

	return nil
}

// CheckLeftoverResourceSlices checks if there are any ResourceSlice associated with the provider cluster.
func (c *Cluster) CheckLeftoverResourceSlices(ctx context.Context, providerClusterID liqov1beta1.ClusterID) error {
	s := c.local.Printer.StartSpinner("Checking for leftover ResourceSlices")

	rsSelector := labels.Set{
		consts.ReplicationRequestedLabel:   consts.ReplicationRequestedLabelValue,
		consts.ReplicationDestinationLabel: string(providerClusterID),
	}

	resourceSlices, err := getters.ListResourceSlicesByLabel(ctx, c.local.CRClient,
		corev1.NamespaceAll, labels.SelectorFromSet(rsSelector))
	if err != nil {
		s.Fail("Error while retrieving resourceslices on consumer cluster: ", output.PrettyErr(err))
		return err
	}

	if len(resourceSlices) > 0 {
		err := fmt.Errorf("resourceslices are still present on consumer cluster")
		s.Fail(err)
		return err
	}

	s.Success("No leftover resourceslices on consumer cluster")

	return nil
}

// DeleteControlPlaneIdentity deletes the control plane Identity on a consumer cluster given the provider cluster id.
func (c *Cluster) DeleteControlPlaneIdentity(ctx context.Context, providerClusterID liqov1beta1.ClusterID) error {
	s := c.local.Printer.StartSpinner("Deleting identity control plane")

	identity, err := getters.GetControlPlaneIdentityByClusterID(ctx, c.local.CRClient, providerClusterID)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail("Error while retrieving identity control plane: ", output.PrettyErr(err))
		return err
	case apierrors.IsNotFound(err):
		s.Success("Identity control plane already deleted")
	default:
		if err := client.IgnoreNotFound(c.local.CRClient.Delete(ctx, identity)); err != nil {
			s.Fail("Error while deleting Identity control plane: ", output.PrettyErr(err))
			return err
		}
		s.Success("Identity control plane correctly deleted")
	}

	return nil
}

// DeleteTenant deletes a tenant on a provider cluster given the consumer cluster id.
func (c *Cluster) DeleteTenant(ctx context.Context, consumerClusterID liqov1beta1.ClusterID) error {
	s := c.local.Printer.StartSpinner("Deleting tenant")
	tenantNamespace, err := c.tenantNamespaceManager.GetNamespace(ctx, consumerClusterID)
	if err != nil {
		s.Fail("Error while retrieving tenant namespace: ", output.PrettyErr(err))
		return err
	}

	tenant, err := getters.GetTenantByClusterID(ctx, c.local.CRClient, consumerClusterID, tenantNamespace.Name)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail("error while retrieving tenant: ", output.PrettyErr(err))
		return err
	case apierrors.IsNotFound(err):
		s.Success("Tenant already deleted")
	default:
		if err := client.IgnoreNotFound(c.local.CRClient.Delete(ctx, tenant)); err != nil {
			s.Fail("Error while deleting tenant: ", output.PrettyErr(err))
			return err
		}
		s.Success("Tenant correctly deleted")
	}

	return nil
}

// DeleteTenantNamespace deletes a tenant namespace given the remote cluster id.
func (c *Cluster) DeleteTenantNamespace(ctx context.Context, remoteClusterID liqov1beta1.ClusterID, waitForActualDeletion bool) error {
	s := c.local.Printer.StartSpinner("Deleting tenant namespace")

	tenantNamespace, err := c.tenantNamespaceManager.GetNamespace(ctx, remoteClusterID)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail("Error while retrieving tenant namespace: ", output.PrettyErr(err))
		return err
	case apierrors.IsNotFound(err):
		s.Success("Tenant namespace already deleted")
		return nil
	default:
		if err := client.IgnoreNotFound(c.local.CRClient.Delete(ctx, tenantNamespace)); err != nil {
			s.Fail("Error while deleting tenant namespace: ", output.PrettyErr(err))
			return err
		}
		s.Success("Tenant namespace deleted")
	}

	if waitForActualDeletion {
		if err := c.waiter.ForTenantNamespaceAbsence(ctx, remoteClusterID); err != nil {
			return err
		}
	}

	return nil
}

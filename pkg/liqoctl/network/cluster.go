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

package network

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/configuration"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayclient"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayserver"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/publickey"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
)

// Cluster contains the information about a cluster.
type Cluster struct {
	local                  *factory.Factory
	remote                 *factory.Factory
	localNamespaceManager  tenantnamespace.Manager
	remoteNamespaceManager tenantnamespace.Manager
	Waiter                 *wait.Waiter

	clusterIdentity *discoveryv1alpha1.ClusterIdentity

	networkConfiguration *networkingv1alpha1.Configuration
}

// NewCluster returns a new Cluster struct.
func NewCluster(local, remote *factory.Factory) *Cluster {
	return &Cluster{
		local:                  local,
		remote:                 remote,
		localNamespaceManager:  tenantnamespace.NewManager(local.KubeClient),
		remoteNamespaceManager: tenantnamespace.NewManager(remote.KubeClient),
		Waiter:                 wait.NewWaiterFromFactory(local),
	}
}

// Init initializes the cluster struct.
func (c *Cluster) Init(ctx context.Context) error {
	// Set cluster identity.
	if err := c.SetClusterIdentity(ctx); err != nil {
		return err
	}

	// Set local and remote namespaces.
	return c.SetNamespaces(ctx)
}

// SetClusterIdentity set cluster identities of both local and remote clusters retrieving it from the Liqo configmaps.
func (c *Cluster) SetClusterIdentity(ctx context.Context) error {
	// Get cluster identity.
	s := c.local.Printer.StartSpinner("Retrieving cluster identity")

	clusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, c.local.CRClient, c.local.LiqoNamespace)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while retrieving cluster identity: %v", output.PrettyErr(err)))
		return err
	}
	c.clusterIdentity = &clusterIdentity

	s.Success("Cluster identity correctly retrieved")

	return nil
}

// SetNamespaces sets the local and remote namespaces to the liqo-tenants namespaces (creating them if necessary),
// unless the user has explicitly set custom namespaces with the `--namespace` and/or `--remote-namespace` flags.
// All the external network resources will be created in these namespaces in their respective clusters.
func (c *Cluster) SetNamespaces(ctx context.Context) error {
	remoteClusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, c.remote.CRClient, c.remote.LiqoNamespace)
	if err != nil {
		return err
	}
	if c.local.Namespace == "" || c.local.Namespace == corev1.NamespaceDefault {
		if _, err := c.localNamespaceManager.CreateNamespace(ctx, remoteClusterIdentity); err != nil {
			return err
		}
		c.local.Namespace = tenantnamespace.GetNameForNamespace(remoteClusterIdentity)
	}

	localClusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, c.local.CRClient, c.local.LiqoNamespace)
	if err != nil {
		return err
	}
	if c.remote.Namespace == "" || c.remote.Namespace == corev1.NamespaceDefault {
		if _, err := c.remoteNamespaceManager.CreateNamespace(ctx, localClusterIdentity); err != nil {
			return err
		}
		c.remote.Namespace = tenantnamespace.GetNameForNamespace(localClusterIdentity)
	}

	return nil
}

// SetLocalConfiguration forges and set a local Configuration to be applied on remote clusters.
func (c *Cluster) SetLocalConfiguration(ctx context.Context) error {
	// Get network configuration.
	s := c.local.Printer.StartSpinner("Retrieving network configuration")
	conf, err := configuration.ForgeLocalConfiguration(ctx, c.local.CRClient, c.local.Namespace, c.local.LiqoNamespace)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while retrieving network configuration: %v", output.PrettyErr(err)))
		return err
	}
	c.networkConfiguration = conf
	s.Success("Network configuration correctly retrieved")

	return nil
}

// SetupConfiguration sets up the network configuration.
func (c *Cluster) SetupConfiguration(ctx context.Context, conf *networkingv1alpha1.Configuration) error {
	s := c.local.Printer.StartSpinner("Setting up network configuration")
	conf.Namespace = c.local.Namespace
	confCopy := conf.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, c.local.CRClient, conf, func() error {
		if conf.Labels == nil {
			conf.Labels = make(map[string]string)
		}
		if confCopy.Labels != nil {
			if cID, ok := confCopy.Labels[consts.RemoteClusterID]; ok {
				conf.Labels[consts.RemoteClusterID] = cID
			}
		}
		conf.Spec.Remote = confCopy.Spec.Remote
		return nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while setting up network configuration: %v", output.PrettyErr(err)))
		return err
	}

	s.Success("Network configuration correctly set up")
	return nil
}

// EnsureGatewayServer create or updates a GatewayServer.
func (c *Cluster) EnsureGatewayServer(ctx context.Context, name string, opts *gatewayserver.ForgeOptions) (*networkingv1alpha1.GatewayServer, error) {
	s := c.local.Printer.StartSpinner("Setting up Gateway Server")
	gwServer, err := gatewayserver.ForgeGatewayServer(name, c.local.Namespace, opts)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while forging gatewayserver: %v", output.PrettyErr(err)))
		return nil, err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c.local.CRClient, gwServer, func() error {
		return gatewayserver.MutateGatewayServer(gwServer, opts)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while setting up gatewayserver: %v", output.PrettyErr(err)))
		return nil, err
	}

	s.Success("Gatewayserver correctly set up")
	return gwServer, nil
}

// EnsureGatewayClient create or updates a GatewayClient.
func (c *Cluster) EnsureGatewayClient(ctx context.Context, name string, opts *gatewayclient.ForgeOptions) (*networkingv1alpha1.GatewayClient, error) {
	s := c.local.Printer.StartSpinner("Setting up Gateway Client")
	gwClient, err := gatewayclient.ForgeGatewayClient(name, c.local.Namespace, opts)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while forging gatewayclient: %v", output.PrettyErr(err)))
		return nil, err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c.local.CRClient, gwClient, func() error {
		return gatewayclient.MutateGatewayClient(gwClient, opts)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while setting up gatewayclient: %v", output.PrettyErr(err)))
		return nil, err
	}

	s.Success("Gatewayclient correctly set up")
	return gwClient, nil
}

// EnsurePublicKey create or updates a PublicKey.
func (c *Cluster) EnsurePublicKey(ctx context.Context, remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity,
	key []byte, ownerGateway metav1.Object) error {
	s := c.local.Printer.StartSpinner("Creating PublicKey")
	pubKey, err := publickey.ForgePublicKey(remoteClusterIdentity.ClusterName, c.local.Namespace, remoteClusterIdentity.ClusterID, key)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while forging publickey: %v", output.PrettyErr(err)))
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c.local.CRClient, pubKey, func() error {
		if err := publickey.MutatePublicKey(pubKey, remoteClusterIdentity.ClusterID, key); err != nil {
			return err
		}
		return controllerutil.SetOwnerReference(ownerGateway, pubKey, c.local.CRClient.Scheme())
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while creating publickey: %v", output.PrettyErr(err)))
		return err
	}

	s.Success("PublicKey correctly created")
	return nil
}

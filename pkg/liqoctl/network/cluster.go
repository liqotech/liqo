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

package network

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	gwforge "github.com/liqotech/liqo/pkg/gateway/forge"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/configuration"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
)

// Cluster contains the information about a cluster.
type Cluster struct {
	local  *factory.Factory
	remote *factory.Factory
	waiter *wait.Waiter

	localNamespaceManager  tenantnamespace.Manager
	remoteNamespaceManager tenantnamespace.Manager

	localClusterID  liqov1beta1.ClusterID
	remoteClusterID liqov1beta1.ClusterID

	networkConfiguration *networkingv1beta1.Configuration
}

// NewCluster returns a new Cluster struct.
func NewCluster(ctx context.Context, local, remote *factory.Factory, createTenantNs bool) (*Cluster, error) {
	cluster := Cluster{
		local:  local,
		remote: remote,
		waiter: wait.NewWaiterFromFactory(local),

		localNamespaceManager:  tenantnamespace.NewManager(local.KubeClient, local.CRClient.Scheme()),
		remoteNamespaceManager: tenantnamespace.NewManager(remote.KubeClient, remote.CRClient.Scheme()),
	}

	if err := cluster.SetClusterIDs(ctx); err != nil {
		return nil, err
	}

	if err := cluster.SetNamespaces(ctx, createTenantNs); err != nil {
		return nil, err
	}

	return &cluster, nil
}

// SetClusterIDs set the local and remote cluster id retrieving it from the Liqo configmaps.
func (c *Cluster) SetClusterIDs(ctx context.Context) error {
	// Get local cluster id.
	clusterID, err := liqoutils.GetClusterIDWithControllerClient(ctx, c.local.CRClient, c.local.LiqoNamespace)
	if err != nil {
		c.local.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving cluster id: %v", output.PrettyErr(err)))
		return err
	}
	c.localClusterID = clusterID

	// Get remote cluster id.
	clusterID, err = liqoutils.GetClusterIDWithControllerClient(ctx, c.remote.CRClient, c.remote.LiqoNamespace)
	if err != nil {
		c.remote.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving cluster id: %v", output.PrettyErr(err)))
		return err
	}
	c.remoteClusterID = clusterID

	return nil
}

// SetNamespaces sets the local and remote namespaces to the liqo-tenants namespaces (creating them if specified),
// unless the user has explicitly set custom namespaces with the `--namespace` and/or `--remote-namespace` flags.
// All the external network resources will be created in these namespaces in their respective clusters.
func (c *Cluster) SetNamespaces(ctx context.Context, createTenantNs bool) error {
	if c.localClusterID == "" || c.remoteClusterID == "" {
		if err := c.SetClusterIDs(ctx); err != nil {
			return err
		}
	}

	if c.local.Namespace == "" || c.local.Namespace == corev1.NamespaceDefault {
		if createTenantNs {
			if _, err := c.localNamespaceManager.CreateNamespace(ctx, c.remoteClusterID); err != nil {
				c.local.Printer.CheckErr(fmt.Errorf("an error occurred while creating tenant namespace: %v", output.PrettyErr(err)))
				return err
			}
		}
		c.local.Namespace = tenantnamespace.GetNameForNamespace(c.remoteClusterID)
	}

	if c.remote.Namespace == "" || c.remote.Namespace == corev1.NamespaceDefault {
		if createTenantNs {
			if _, err := c.remoteNamespaceManager.CreateNamespace(ctx, c.localClusterID); err != nil {
				c.remote.Printer.CheckErr(fmt.Errorf("an error occurred while creating tenant namespace: %v", output.PrettyErr(err)))
				return err
			}
		}
		c.remote.Namespace = tenantnamespace.GetNameForNamespace(c.localClusterID)
	}

	return nil
}

// SetLocalConfiguration forges and set a local Configuration to be applied on remote clusters.
func (c *Cluster) SetLocalConfiguration(ctx context.Context) error {
	// Get network configuration.
	s := c.local.Printer.StartSpinner("Retrieving network configuration")
	conf, err := forge.ConfigurationForRemoteCluster(ctx, c.local.CRClient, c.local.Namespace, c.local.LiqoNamespace)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while retrieving network configuration: %v", output.PrettyErr(err)))
		return err
	}
	c.networkConfiguration = conf
	s.Success("Network configuration correctly retrieved")

	return nil
}

// SetupConfiguration sets up the network configuration.
func (c *Cluster) SetupConfiguration(ctx context.Context, conf *networkingv1beta1.Configuration) error {
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

// CheckNetworkInitialized checks if the network is initialized correctly.
func (c *Cluster) CheckNetworkInitialized(ctx context.Context, remoteClusterID liqov1beta1.ClusterID) error {
	s := c.local.Printer.StartSpinner("Checking network is initialized correctly")

	confReady, err := configuration.IsConfigurationStatusSet(ctx, c.local.CRClient,
		forge.DefaultConfigurationName(remoteClusterID), c.local.Namespace)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail(fmt.Sprintf("An error occurred while checking network Configuration: %v", output.PrettyErr(err)))
		return err
	case apierrors.IsNotFound(err):
		s.Fail(fmt.Sprintf("Network Configuration not found. Initialize the network first with `liqoctl network init`: %v", output.PrettyErr(err)))
		return err
	case !confReady:
		err := fmt.Errorf("network Configuration status is not set yet. Retry later or initialize the network again with `liqoctl network init`")
		s.Fail(err)
		return err
	}

	s.Success("Network correctly initialized")
	return nil
}

// GetGatewayServer retrieves a GatewayServer.
func (c *Cluster) GetGatewayServer(ctx context.Context, name string) (*networkingv1beta1.GatewayServer, error) {
	gwServer := &networkingv1beta1.GatewayServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.local.Namespace,
		},
	}
	if err := c.local.CRClient.Get(ctx, client.ObjectKeyFromObject(gwServer), gwServer); err != nil {
		return nil, err
	}
	return gwServer, nil
}

// GetGatewayClient retrieves a GatewayClient.
func (c *Cluster) GetGatewayClient(ctx context.Context, name string) (*networkingv1beta1.GatewayClient, error) {
	gwClient := &networkingv1beta1.GatewayClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.local.Namespace,
		},
	}
	if err := c.local.CRClient.Get(ctx, client.ObjectKeyFromObject(gwClient), gwClient); err != nil {
		return nil, err
	}
	return gwClient, nil
}

func endpointHasChanged(endpoint *networkingv1beta1.Endpoint, service *corev1.Service) bool {
	if endpoint.ServiceType != service.Spec.Type {
		return true
	}
	if len(service.Spec.Ports) > 0 {
		if endpoint.Port != service.Spec.Ports[0].Port {
			return true
		}

		if endpoint.NodePort != nil && *endpoint.NodePort != service.Spec.Ports[0].NodePort {
			return true
		}
	}
	if endpoint.LoadBalancerIP != nil && *endpoint.LoadBalancerIP != service.Spec.LoadBalancerIP ||
		endpoint.LoadBalancerIP == nil && service.Spec.LoadBalancerIP != "" {
		return true
	}
	return false
}

// EnsureGatewayServer create or updates a GatewayServer.
func (c *Cluster) EnsureGatewayServer(ctx context.Context, name string, opts *forge.GwServerOptions) (*networkingv1beta1.GatewayServer, error) {
	s := c.local.Printer.StartSpinner("Setting up gateway server")
	gwServer, err := forge.GatewayServer(name, c.local.Namespace, opts)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while forging gateway server: %v", output.PrettyErr(err)))
		return nil, err
	}

	// If the forged server endpoint has different parameters from the existing server service (if present),
	// we delete the existing gateway server so that the client can correctly connect to the new endpoint.
	var service corev1.Service
	svcNsName := types.NamespacedName{Namespace: gwServer.Namespace, Name: gwforge.GatewayResourceName(gwServer.Name)}
	err = c.local.CRClient.Get(ctx, svcNsName, &service)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("An error occurred while retrieving gateway server service: %v", output.PrettyErr(err)))
		return nil, err
	} else if err == nil {
		// Server service already exists. Check if endpoint has changed parameters.
		if endpointHasChanged(&gwServer.Spec.Endpoint, &service) {
			if err := c.local.CRClient.Delete(ctx, gwServer); err != nil {
				s.Fail(fmt.Sprintf("An error occurred while deleting gateway server: %v", output.PrettyErr(err)))
				return nil, err
			}
			s.Success("Deleted existing gateway server")
		}
	}

	_, err = controllerutil.CreateOrUpdate(ctx, c.local.CRClient, gwServer, func() error {
		return forge.MutateGatewayServer(gwServer, opts)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while setting up gateway server: %v", output.PrettyErr(err)))
		return nil, err
	}

	s.Success("Gateway server correctly set up")
	return gwServer, nil
}

// EnsureGatewayClient create or updates a GatewayClient.
func (c *Cluster) EnsureGatewayClient(ctx context.Context, name string, opts *forge.GwClientOptions) (*networkingv1beta1.GatewayClient, error) {
	s := c.local.Printer.StartSpinner("Setting up gateway client")
	gwClient, err := forge.GatewayClient(name, c.local.Namespace, opts)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while forging gateway client: %v", output.PrettyErr(err)))
		return nil, err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c.local.CRClient, gwClient, func() error {
		return forge.MutateGatewayClient(gwClient, opts)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while setting up gateway client: %v", output.PrettyErr(err)))
		return nil, err
	}

	s.Success("Gateway client correctly set up")
	return gwClient, nil
}

// EnsurePublicKey create or updates a PublicKey.
func (c *Cluster) EnsurePublicKey(ctx context.Context, remoteClusterID liqov1beta1.ClusterID,
	key []byte, ownerGateway metav1.Object) error {
	s := c.local.Printer.StartSpinner("Creating public key")
	pubKey, err := forge.PublicKey(forge.DefaultPublicKeyName(remoteClusterID), c.local.Namespace,
		remoteClusterID, key)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while forging public key: %v", output.PrettyErr(err)))
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c.local.CRClient, pubKey, func() error {
		if err := forge.MutatePublicKey(pubKey, remoteClusterID, key); err != nil {
			return err
		}
		return controllerutil.SetOwnerReference(ownerGateway, pubKey, c.local.CRClient.Scheme())
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while creating public key: %v", output.PrettyErr(err)))
		return err
	}

	s.Success("Public key correctly created")
	return nil
}

// DeleteConfiguration deletes a Configuration.
func (c *Cluster) DeleteConfiguration(ctx context.Context, name string) error {
	s := c.local.Printer.StartSpinner("Deleting network configuration")

	conf := &networkingv1beta1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.local.Namespace,
		},
	}

	err := c.local.CRClient.Delete(ctx, conf)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail("An error occurred while deleting network configuration: ", output.PrettyErr(err))
		return err
	case apierrors.IsNotFound(err):
		s.Success("Network configuration already deleted")
	default:
		s.Success("Network configuration correctly deleted")
	}

	return nil
}

// DeleteGatewayServer deletes a GatewayServer.
func (c *Cluster) DeleteGatewayServer(ctx context.Context, name string) error {
	s := c.local.Printer.StartSpinner("Deleting gateway server")

	gwServer := &networkingv1beta1.GatewayServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.local.Namespace,
		},
	}

	err := c.local.CRClient.Delete(ctx, gwServer)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail("An error occurred while deleting gateway server: ", output.PrettyErr(err))
		return err
	case apierrors.IsNotFound(err):
		s.Success("Gateway server already deleted")
	default:
		s.Success("Gateway server correctly deleted")
	}

	return nil
}

// DeleteGatewayClient deletes a GatewayClient.
func (c *Cluster) DeleteGatewayClient(ctx context.Context, name string) error {
	s := c.local.Printer.StartSpinner("Deleting gateway client")

	gwClient := &networkingv1beta1.GatewayClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.local.Namespace,
		},
	}

	err := c.local.CRClient.Delete(ctx, gwClient)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail("An error occurred while deleting gateway client: ", output.PrettyErr(err))
		return err
	case apierrors.IsNotFound(err):
		s.Success("Gateway client already deleted")
	default:
		s.Success("Gateway client correctly deleted")
	}

	return nil
}

// CheckAlreadyEstablishedForGwServer checks if a GatewayServer is already established.
func (c *Cluster) CheckAlreadyEstablishedForGwServer(ctx context.Context) (bool, error) {
	_, err := c.GetGatewayServer(ctx, forge.DefaultGatewayServerName(c.remoteClusterID))
	if err == nil {
		return true, nil
	} else if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	return false, nil
}

// CheckAlreadyEstablishedForGwClient checks if a GatewayClient is already established.
func (c *Cluster) CheckAlreadyEstablishedForGwClient(ctx context.Context) (bool, error) {
	_, err := c.GetGatewayClient(ctx, forge.DefaultGatewayClientName(c.remoteClusterID))
	if err == nil {
		return true, nil
	} else if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	return false, nil
}

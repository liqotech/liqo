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
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/configuration"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayclient"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayserver"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/publickey"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// Options encapsulates the arguments of the network command.
type Options struct {
	LocalFactory  *factory.Factory
	RemoteFactory *factory.Factory

	Timeout time.Duration
	Wait    bool

	ServerGatewayType       string
	ServerTemplateName      string
	ServerTemplateNamespace string
	ServerServiceType       *argsutils.StringEnum
	ServerPort              int32
	ServerNodePort          int32
	ServerLoadBalancerIP    string

	ClientGatewayType       string
	ClientTemplateName      string
	ClientTemplateNamespace string

	MTU                int
	DisableSharingKeys bool
	Proxy              bool
}

// NewOptions returns a new Options struct.
func NewOptions(localFactory *factory.Factory) *Options {
	return &Options{
		LocalFactory: localFactory,
		ServerServiceType: argsutils.NewEnum(
			[]string{string(v1.ServiceTypeLoadBalancer), string(v1.ServiceTypeNodePort)}, string(gatewayserver.DefaultServiceType)),
	}
}

// RunInit initializes the liqo networking between two clusters.
func (o *Options) RunInit(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster 1.
	cluster1 := NewCluster(o.LocalFactory, o.RemoteFactory)
	if err := cluster1.Init(ctx); err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2 := NewCluster(o.RemoteFactory, o.LocalFactory)
	if err := cluster2.Init(ctx); err != nil {
		return err
	}

	// Forges the local Configuration of cluster 1 to be applied on remote clusters.
	if err := cluster1.SetLocalConfiguration(ctx); err != nil {
		return err
	}

	// Forges the local Configuration of cluster 2 to be applied on remote clusters.
	if err := cluster2.SetLocalConfiguration(ctx); err != nil {
		return err
	}

	// Setup Configurations in cluster 1.
	if err := cluster1.SetupConfiguration(ctx, cluster2.networkConfiguration); err != nil {
		return err
	}

	// Setup Configurations in cluster 2.
	if err := cluster2.SetupConfiguration(ctx, cluster1.networkConfiguration); err != nil {
		return err
	}

	if o.Wait {
		// Wait for cluster 1 to be ready.
		if err := cluster1.Waiter.ForConfiguration(ctx, cluster2.networkConfiguration); err != nil {
			return err
		}

		// Wait for cluster 2 to be ready.
		if err := cluster2.Waiter.ForConfiguration(ctx, cluster1.networkConfiguration); err != nil {
			return err
		}
	}

	return nil
}

// RunReset reset the liqo networking between two clusters.
func (o *Options) RunReset(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster 1.
	cluster1 := NewCluster(o.LocalFactory, o.RemoteFactory)
	if err := cluster1.Init(ctx); err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2 := NewCluster(o.RemoteFactory, o.LocalFactory)
	if err := cluster2.Init(ctx); err != nil {
		return err
	}

	// If the clusters are still connected through the gateways, disconnect them before removing network Configurations.
	gwServer, err := cluster1.GetGatewayServer(ctx, gatewayserver.DefaultGatewayServerName(cluster2.clusterIdentity))
	switch {
	case client.IgnoreNotFound(err) != nil:
		return err
	case err == nil:
		if err := cluster1.DeleteGatewayServer(ctx, gwServer.Name); err != nil {
			return err
		}
	}

	gwClient, err := cluster2.GetGatewayClient(ctx, gatewayclient.DefaultGatewayClientName(cluster1.clusterIdentity))
	switch {
	case client.IgnoreNotFound(err) != nil:
		return err
	case err == nil:
		if err := cluster2.DeleteGatewayClient(ctx, gwClient.Name); err != nil {
			return err
		}
	}

	// Delete Configuration on cluster 1
	if err := cluster1.DeleteConfiguration(ctx, configuration.DefaultConfigurationName(cluster2.clusterIdentity)); err != nil {
		return err
	}

	// Delete Configuration on cluster 2
	return cluster2.DeleteConfiguration(ctx, configuration.DefaultConfigurationName(cluster1.clusterIdentity))
}

// RunConnect connect two clusters using liqo networking.
func (o *Options) RunConnect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster 1.
	cluster1 := NewCluster(o.LocalFactory, o.RemoteFactory)
	if err := cluster1.Init(ctx); err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2 := NewCluster(o.RemoteFactory, o.LocalFactory)
	if err := cluster2.Init(ctx); err != nil {
		return err
	}

	// Check if the Networking is initialized on cluster 1
	if err := cluster1.CheckNetworkInitialized(ctx, cluster2.clusterIdentity); err != nil {
		return err
	}

	// Check if the Networking is initialized on cluster 2
	if err := cluster2.CheckNetworkInitialized(ctx, cluster1.clusterIdentity); err != nil {
		return err
	}

	// Create gateway server on cluster 1
	gwServer, err := cluster1.EnsureGatewayServer(ctx,
		gatewayserver.DefaultGatewayServerName(cluster2.clusterIdentity),
		o.newGatewayServerForgeOptions(o.LocalFactory.KubeClient, cluster2.clusterIdentity.ClusterID))
	if err != nil {
		return err
	}

	// Wait for the gateway pod to be ready
	if err := cluster1.Waiter.ForGatewayPodReady(ctx, gwServer); err != nil {
		return err
	}

	// Wait for the endpoint status of the gateway server to be set
	if err := cluster1.Waiter.ForGatewayServerStatusEndpoint(ctx, gwServer); err != nil {
		return err
	}

	// Create gateway client on cluster 2
	gwClient, err := cluster2.EnsureGatewayClient(ctx,
		gatewayclient.DefaultGatewayClientName(cluster1.clusterIdentity),
		o.newGatewayClientForgeOptions(o.RemoteFactory.KubeClient, cluster1.clusterIdentity.ClusterID, gwServer.Status.Endpoint))
	if err != nil {
		return err
	}

	// Wait for the gateway pod to be ready
	if err := cluster2.Waiter.ForGatewayPodReady(ctx, gwClient); err != nil {
		return err
	}

	// If sharing keys is disabled, return immediately
	if o.DisableSharingKeys {
		return nil
	}

	// Wait for gateway server to set secret reference (containing the server public key) in the status
	err = cluster1.Waiter.ForGatewayServerSecretRef(ctx, gwServer)
	if err != nil {
		return err
	}
	keyServer, err := publickey.ExtractKeyFromSecretRef(ctx, cluster1.local.CRClient, gwServer.Status.SecretRef)
	if err != nil {
		return err
	}

	// Create PublicKey of gateway server on cluster 2
	if err := cluster2.EnsurePublicKey(ctx, cluster1.clusterIdentity, keyServer, gwClient); err != nil {
		return err
	}

	// Wait for gateway client to set secret reference (containing the client public key) in the status
	err = cluster2.Waiter.ForGatewayClientSecretRef(ctx, gwClient)
	if err != nil {
		return err
	}
	keyClient, err := publickey.ExtractKeyFromSecretRef(ctx, cluster2.local.CRClient, gwClient.Status.SecretRef)
	if err != nil {
		return err
	}

	// Create PublicKey of gateway client on cluster 1
	if err := cluster1.EnsurePublicKey(ctx, cluster2.clusterIdentity, keyClient, gwServer); err != nil {
		return err
	}

	if o.Wait {
		// Wait for Connections on both cluster to be created.
		conn1, err := cluster1.Waiter.ForConnection(ctx, gwServer.Namespace, cluster2.clusterIdentity)
		if err != nil {
			return err
		}
		conn2, err := cluster2.Waiter.ForConnection(ctx, gwClient.Namespace, cluster1.clusterIdentity)
		if err != nil {
			return err
		}

		// Wait for Connections on both cluster cluster to be established
		if err := cluster1.Waiter.ForConnectionEstablished(ctx, conn1); err != nil {
			return err
		}
		if err := cluster2.Waiter.ForConnectionEstablished(ctx, conn2); err != nil {
			return err
		}
	}

	return nil
}

// RunDisconnect disconnects two clusters.
func (o *Options) RunDisconnect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster 1.
	cluster1 := NewCluster(o.LocalFactory, o.RemoteFactory)
	if err := cluster1.Init(ctx); err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2 := NewCluster(o.RemoteFactory, o.LocalFactory)
	if err := cluster2.Init(ctx); err != nil {
		return err
	}

	// Delete gateway server on cluster 1
	if err := cluster1.DeleteGatewayServer(ctx, gatewayserver.DefaultGatewayServerName(cluster2.clusterIdentity)); err != nil {
		return err
	}

	// Delete gateway client on cluster 2
	return cluster2.DeleteGatewayClient(ctx, gatewayclient.DefaultGatewayClientName(cluster1.clusterIdentity))
}

func (o *Options) newGatewayServerForgeOptions(kubeClient kubernetes.Interface, remoteClusterID string) *gatewayserver.ForgeOptions {
	if o.ServerTemplateNamespace == "" {
		o.ServerTemplateNamespace = o.LocalFactory.LiqoNamespace
	}

	return &gatewayserver.ForgeOptions{
		KubeClient:        kubeClient,
		RemoteClusterID:   remoteClusterID,
		GatewayType:       o.ServerGatewayType,
		TemplateName:      o.ServerTemplateName,
		TemplateNamespace: o.ServerTemplateNamespace,
		ServiceType:       v1.ServiceType(o.ServerServiceType.Value),
		MTU:               o.MTU,
		Port:              o.ServerPort,
		NodePort:          ptr.To(o.ServerNodePort),
		LoadBalancerIP:    ptr.To(o.ServerLoadBalancerIP),
		Proxy:             o.Proxy,
	}
}

func (o *Options) newGatewayClientForgeOptions(kubeClient kubernetes.Interface, remoteClusterID string,
	serverEndpoint *networkingv1alpha1.EndpointStatus) *gatewayclient.ForgeOptions {
	if o.ClientTemplateNamespace == "" {
		o.ClientTemplateNamespace = o.RemoteFactory.LiqoNamespace
	}

	return &gatewayclient.ForgeOptions{
		KubeClient:        kubeClient,
		RemoteClusterID:   remoteClusterID,
		GatewayType:       o.ClientGatewayType,
		TemplateName:      o.ClientTemplateName,
		TemplateNamespace: o.ClientTemplateNamespace,
		MTU:               o.MTU,
		Addresses:         serverEndpoint.Addresses,
		Port:              serverEndpoint.Port,
		Protocol:          string(*serverEndpoint.Protocol),
	}
}

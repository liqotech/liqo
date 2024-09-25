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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/getters"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
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
}

// NewOptions returns a new Options struct.
func NewOptions(localFactory *factory.Factory) *Options {
	return &Options{
		LocalFactory: localFactory,
		ServerServiceType: argsutils.NewEnum(
			[]string{string(corev1.ServiceTypeLoadBalancer), string(corev1.ServiceTypeNodePort), string(corev1.ServiceTypeClusterIP)},
			string(forge.DefaultGwServerServiceType)),
	}
}

// RunInit initializes the liqo networking between two clusters.
func (o *Options) RunInit(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster 1.
	cluster1, err := NewCluster(ctx, o.LocalFactory, o.RemoteFactory, true)
	if err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2, err := NewCluster(ctx, o.RemoteFactory, o.LocalFactory, true)
	if err != nil {
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
		if err := cluster1.waiter.ForConfiguration(ctx, cluster2.networkConfiguration); err != nil {
			return err
		}

		// Wait for cluster 2 to be ready.
		if err := cluster2.waiter.ForConfiguration(ctx, cluster1.networkConfiguration); err != nil {
			return err
		}
	}

	return nil
}

// RunReset reset the liqo networking between two clusters.
// If the clusters are still connected through the gateways, it deletes them before removing network Configurations.
func (o *Options) RunReset(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster 1.
	cluster1, err := NewCluster(ctx, o.LocalFactory, o.RemoteFactory, false)
	if err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2, err := NewCluster(ctx, o.RemoteFactory, o.LocalFactory, false)
	if err != nil {
		return err
	}

	// Delete gateway client on cluster 1
	if err := cluster1.DeleteGatewayClient(ctx, forge.DefaultGatewayClientName(cluster2.localClusterID)); err != nil {
		return err
	}

	// Delete gateway client on cluster 2
	if err := cluster2.DeleteGatewayClient(ctx, forge.DefaultGatewayClientName(cluster1.localClusterID)); err != nil {
		return err
	}

	// Delete gateway server on cluster 1
	if err := cluster1.DeleteGatewayServer(ctx, forge.DefaultGatewayServerName(cluster2.localClusterID)); err != nil {
		return err
	}

	// Delete gateway server on cluster 2
	if err := cluster2.DeleteGatewayServer(ctx, forge.DefaultGatewayServerName(cluster1.localClusterID)); err != nil {
		return err
	}

	// Delete Configuration on cluster 1
	if err := cluster1.DeleteConfiguration(ctx, forge.DefaultConfigurationName(cluster2.localClusterID)); err != nil {
		return err
	}

	// Delete Configuration on cluster 2
	return cluster2.DeleteConfiguration(ctx, forge.DefaultConfigurationName(cluster1.localClusterID))
}

// RunConnect connect two clusters using liqo networking.
func (o *Options) RunConnect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Create and initialize cluster 1.
	cluster1, err := NewCluster(ctx, o.LocalFactory, o.RemoteFactory, true)
	if err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2, err := NewCluster(ctx, o.RemoteFactory, o.LocalFactory, true)
	if err != nil {
		return err
	}

	// Check if the Networking is initialized on cluster 1
	if err := cluster1.CheckNetworkInitialized(ctx, cluster2.localClusterID); err != nil {
		return err
	}

	// Check if the Networking is initialized on cluster 2
	if err := cluster2.CheckNetworkInitialized(ctx, cluster1.localClusterID); err != nil {
		return err
	}

	// Check if the reverse Networking is already established on cluster 1
	if established, err := cluster1.CheckAlreadyEstablishedForGwServer(ctx); err != nil {
		return err
	} else if established {
		return nil
	}

	// Check if the reverse Networking is already established on cluster 2
	if established, err := cluster2.CheckAlreadyEstablishedForGwClient(ctx); err != nil {
		return err
	} else if established {
		return nil
	}

	// Create gateway server on cluster 2
	gwServer, err := cluster2.EnsureGatewayServer(ctx,
		forge.DefaultGatewayServerName(cluster1.localClusterID),
		o.newGatewayServerForgeOptions(o.RemoteFactory.KubeClient, cluster1.localClusterID))
	if err != nil {
		return err
	}

	// Wait for the gateway pod to be ready
	if err := cluster2.waiter.ForGatewayPodReady(ctx, gwServer); err != nil {
		return err
	}

	// Wait for the endpoint status of the gateway server to be set
	if err := cluster2.waiter.ForGatewayServerStatusEndpoint(ctx, gwServer); err != nil {
		return err
	}

	// Create gateway client on cluster 1
	gwClient, err := cluster1.EnsureGatewayClient(ctx,
		forge.DefaultGatewayClientName(cluster2.localClusterID),
		o.newGatewayClientForgeOptions(o.LocalFactory.KubeClient, cluster2.localClusterID, gwServer.Status.Endpoint))
	if err != nil {
		return err
	}

	// Wait for the gateway pod to be ready
	if err := cluster1.waiter.ForGatewayPodReady(ctx, gwClient); err != nil {
		return err
	}

	// If sharing keys is disabled, return immediately
	if o.DisableSharingKeys {
		return nil
	}

	// Wait for gateway server to set secret reference (containing the server public key) in the status
	err = cluster2.waiter.ForGatewayServerSecretRef(ctx, gwServer)
	if err != nil {
		return err
	}
	keyServer, err := getters.ExtractKeyFromSecretRef(ctx, cluster2.local.CRClient, gwServer.Status.SecretRef)
	if err != nil {
		return err
	}

	// Create PublicKey of gateway server on cluster 1
	if err := cluster1.EnsurePublicKey(ctx, cluster2.localClusterID, keyServer, gwClient); err != nil {
		return err
	}

	// Wait for gateway client to set secret reference (containing the client public key) in the status
	err = cluster1.waiter.ForGatewayClientSecretRef(ctx, gwClient)
	if err != nil {
		return err
	}
	keyClient, err := getters.ExtractKeyFromSecretRef(ctx, cluster1.local.CRClient, gwClient.Status.SecretRef)
	if err != nil {
		return err
	}

	// Create PublicKey of gateway client on cluster 2
	if err := cluster2.EnsurePublicKey(ctx, cluster1.localClusterID, keyClient, gwServer); err != nil {
		return err
	}

	if o.Wait {
		// Wait for Connections on both cluster to be created.
		conn2, err := cluster2.waiter.ForConnection(ctx, gwServer.Namespace, cluster1.localClusterID)
		if err != nil {
			return err
		}
		conn1, err := cluster1.waiter.ForConnection(ctx, gwClient.Namespace, cluster2.localClusterID)
		if err != nil {
			return err
		}

		// Wait for Connections on both cluster cluster to be established
		if err := cluster1.waiter.ForConnectionEstablished(ctx, conn1); err != nil {
			return err
		}
		if err := cluster2.waiter.ForConnectionEstablished(ctx, conn2); err != nil {
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
	cluster1, err := NewCluster(ctx, o.LocalFactory, o.RemoteFactory, false)
	if err != nil {
		return err
	}

	// Create and initialize cluster 2.
	cluster2, err := NewCluster(ctx, o.RemoteFactory, o.LocalFactory, false)
	if err != nil {
		return err
	}

	// Delete gateway client on cluster 1
	if err := cluster1.DeleteGatewayClient(ctx, forge.DefaultGatewayClientName(cluster2.localClusterID)); err != nil {
		return err
	}

	// Delete gateway server on cluster 2
	return cluster2.DeleteGatewayServer(ctx, forge.DefaultGatewayServerName(cluster1.localClusterID))
}

func (o *Options) newGatewayServerForgeOptions(kubeClient kubernetes.Interface, remoteClusterID liqov1beta1.ClusterID) *forge.GwServerOptions {
	if o.ServerTemplateNamespace == "" {
		o.ServerTemplateNamespace = o.RemoteFactory.LiqoNamespace
	}

	return &forge.GwServerOptions{
		KubeClient:        kubeClient,
		RemoteClusterID:   remoteClusterID,
		GatewayType:       o.ServerGatewayType,
		TemplateName:      o.ServerTemplateName,
		TemplateNamespace: o.ServerTemplateNamespace,
		ServiceType:       corev1.ServiceType(o.ServerServiceType.Value),
		MTU:               o.MTU,
		Port:              o.ServerPort,
		NodePort:          ptr.To(o.ServerNodePort),
		LoadBalancerIP:    ptr.To(o.ServerLoadBalancerIP),
	}
}

func (o *Options) newGatewayClientForgeOptions(kubeClient kubernetes.Interface, remoteClusterID liqov1beta1.ClusterID,
	serverEndpoint *networkingv1beta1.EndpointStatus) *forge.GwClientOptions {
	if o.ClientTemplateNamespace == "" {
		o.ClientTemplateNamespace = o.LocalFactory.LiqoNamespace
	}

	return &forge.GwClientOptions{
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

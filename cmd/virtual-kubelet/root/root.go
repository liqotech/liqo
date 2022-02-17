// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package root provides methods to build and start the virtual-kubelet.
package root

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/internal/utils/errdefs"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	nodeprovider "github.com/liqotech/liqo/pkg/virtualKubelet/liqoNodeProvider"
	podprovider "github.com/liqotech/liqo/pkg/virtualKubelet/provider"
)

const defaultVersion = "v1.22.1" // This should follow the version of k8s.io/kubernetes we are importing

// NewCommand creates a new top-level command.
// This command is used to start the virtual-kubelet daemon.
func NewCommand(ctx context.Context, name string, c *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: name + " implements the Liqo Virtual Kubelet logic.",
		Long:  name + " implements the Liqo Virtual Kubelet logic.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootCommand(ctx, c)
		},
	}
	return cmd
}

func runRootCommand(ctx context.Context, c *Opts) error {
	if c.ForeignCluster.ClusterID == "" {
		return errors.New("cluster id is mandatory")
	}
	if c.ForeignCluster.ClusterName == "" {
		return errors.New("cluster name is mandatory")
	}

	if c.PodWorkers == 0 || c.ServiceWorkers == 0 || c.EndpointSliceWorkers == 0 || c.ConfigMapWorkers == 0 || c.SecretWorkers == 0 {
		return errdefs.InvalidInput("reflection workers must be greater than 0")
	}

	localConfig, err := utils.GetRestConfig(c.HomeKubeconfig)
	if err != nil {
		return err
	}

	restcfg.SetRateLimiter(localConfig)
	localClient := kubernetes.NewForConfigOrDie(localConfig)

	// Retrieve the remote restcfg
	tenantNamespaceManager := tenantnamespace.NewTenantNamespaceManager(localClient)
	identityManager := identitymanager.NewCertificateIdentityReader(localClient, c.HomeCluster, tenantNamespaceManager)

	remoteConfig, err := identityManager.GetConfig(c.ForeignCluster, c.TenantNamespace)
	if err != nil {
		return err
	}

	restcfg.SetRateLimiter(remoteConfig)

	// Initialize the pod provider
	podcfg := podprovider.InitConfig{
		HomeConfig:    localConfig,
		RemoteConfig:  remoteConfig,
		HomeCluster:   c.HomeCluster,
		RemoteCluster: c.ForeignCluster,

		Namespace: c.TenantNamespace,
		NodeName:  c.NodeName,
		NodeIP:    os.Getenv("VKUBELET_POD_IP"),

		LiqoIpamServer:       c.LiqoIpamServer,
		InformerResyncPeriod: c.InformerResyncPeriod,

		PodWorkers:                  c.PodWorkers,
		ServiceWorkers:              c.ServiceWorkers,
		EndpointSliceWorkers:        c.EndpointSliceWorkers,
		ConfigMapWorkers:            c.ConfigMapWorkers,
		SecretWorkers:               c.SecretWorkers,
		PersistenVolumeClaimWorkers: c.PersistenVolumeClaimWorkers,

		EnableStorage:              c.EnableStorage,
		VirtualStorageClassName:    c.VirtualStorageClassName,
		RemoteRealStorageClassName: c.RemoteRealStorageClassName,
	}

	eb := record.NewBroadcaster()
	podProvider, err := podprovider.NewLiqoProvider(ctx, &podcfg, eb)
	if err != nil {
		return err
	}

	// Initialize the node provider
	nodecfg := nodeprovider.InitConfig{
		HomeConfig:      localConfig,
		RemoteConfig:    remoteConfig,
		HomeClusterID:   c.HomeCluster.ClusterID,
		RemoteClusterID: c.ForeignCluster.ClusterID,
		Namespace:       c.TenantNamespace,

		NodeName:         c.NodeName,
		InternalIP:       os.Getenv("VKUBELET_POD_IP"),
		DaemonPort:       c.ListenPort,
		Version:          getVersion(localConfig),
		ExtraLabels:      c.NodeExtraLabels.StringMap,
		ExtraAnnotations: c.NodeExtraAnnotations.StringMap,

		InformerResyncPeriod: c.InformerResyncPeriod,
		PingDisabled:         c.NodePingInterval == 0,
	}

	nodeProvider := nodeprovider.NewLiqoNodeProvider(&nodecfg)
	nodeReady := nodeProvider.StartProvider(ctx)

	nodeRunner, err := node.NewNodeController(
		nodeProvider, nodeProvider.GetNode(),
		localClient.CoreV1().Nodes(),
		node.WithNodeEnableLeaseV1(localClient.CoordinationV1().Leases(corev1.NamespaceNodeLease), int32(c.NodeLeaseDuration.Seconds())),
		node.WithNodePingInterval(c.NodePingInterval), node.WithNodePingTimeout(c.NodePingTimeout),
		node.WithNodeStatusUpdateErrorHandler(
			func(ctx context.Context, err error) error {
				klog.Info("node setting up")
				newNode := nodeProvider.GetNode().DeepCopy()
				newNode.ResourceVersion = ""

				if nodeProvider.IsTerminating() {
					// this avoids the re-creation of terminated nodes
					klog.V(4).Info("skipping: node is in terminating phase")
					return nil
				}

				oldNode, newErr := localClient.CoreV1().Nodes().Get(ctx, newNode.Name, metav1.GetOptions{})
				if newErr != nil {
					if !k8serrors.IsNotFound(newErr) {
						klog.Error(newErr, "node error")
						return newErr
					}
					_, newErr = localClient.CoreV1().Nodes().Create(ctx, newNode, metav1.CreateOptions{})
					klog.Info("new node created")
				} else {
					oldNode.Status = newNode.Status
					_, newErr = localClient.CoreV1().Nodes().UpdateStatus(ctx, oldNode, metav1.UpdateOptions{})
					if newErr != nil {
						klog.Info("node updated")
					}
				}

				if newErr != nil {
					return newErr
				}
				return nil
			}),
	)
	if err != nil {
		return err
	}

	cancelHTTP, err := setupHTTPServer(ctx,
		podProvider.PodHandler(), getAPIConfig(c), c.HomeCluster.ClusterID, remoteConfig)
	if err != nil {
		return errors.Wrap(err, "error while setting up HTTP server")
	}
	defer cancelHTTP()

	go func() {
		if err := nodeRunner.Run(ctx); err != nil {
			klog.Error(err, "error in pod controller running")
			panic(nil)
		}
	}()

	<-nodeRunner.Ready()

	klog.Info("Setup ended")
	close(nodeReady)
	<-ctx.Done()
	return nil
}

func getVersion(config *rest.Config) string {
	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		klog.Warningf("Cannot read k8s version: using default version %v; error: %v", defaultVersion, err)
		return defaultVersion
	}

	version, err := client.ServerVersion()
	if err != nil {
		klog.Warningf("Cannot read k8s version: using default version %v; error: %v", defaultVersion, err)
		return defaultVersion
	}

	return version.GitVersion
}

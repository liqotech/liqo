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
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if c.ForeignClusterID == "" {
		return errors.New("cluster id is mandatory")
	}

	if c.PodWorkers == 0 || c.ServiceWorkers == 0 || c.EndpointSliceWorkers == 0 || c.ConfigMapWorkers == 0 || c.SecretWorkers == 0 {
		return errdefs.InvalidInput("reflection workers must be greater than 0")
	}

	config, err := utils.GetRestConfig(c.HomeKubeconfig)
	if err != nil {
		return err
	}

	restcfg.SetRateLimiterWithCustomParamenters(config, virtualKubelet.HOME_CLIENT_QPS, virtualKubelet.HOME_CLIENTS_BURST)
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Initialize the pod provider
	podcfg := podprovider.InitConfig{
		HomeConfig:      config,
		HomeClusterID:   c.HomeClusterID,
		RemoteClusterID: c.ForeignClusterID,

		Namespace: c.KubeletNamespace,
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

	podProviderStopper := make(chan struct{})
	podProvider.SetProviderStopper(podProviderStopper)

	// Initialize the node provider
	nodecfg := nodeprovider.InitConfig{
		HomeConfig:      config,
		HomeClusterID:   c.HomeClusterID,
		RemoteClusterID: c.ForeignClusterID,
		Namespace:       c.KubeletNamespace,

		NodeName:         c.NodeName,
		InternalIP:       os.Getenv("VKUBELET_POD_IP"),
		DaemonPort:       c.ListenPort,
		Version:          getVersion(config),
		ExtraLabels:      c.NodeExtraLabels.StringMap,
		ExtraAnnotations: c.NodeExtraAnnotations.StringMap,

		PodProviderStopper:   podProviderStopper,
		InformerResyncPeriod: c.LiqoInformerResyncPeriod,
	}

	nodeProvider := nodeprovider.NewLiqoNodeProvider(&nodecfg)
	nodeReady := nodeProvider.StartProvider(ctx)

	nodeRunner, err := node.NewNodeController(
		nodeProvider, nodeProvider.GetNode(),
		client.CoreV1().Nodes(),
		node.WithNodeEnableLeaseV1(client.CoordinationV1().Leases(corev1.NamespaceNodeLease), node.DefaultLeaseDuration),
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

				oldNode, newErr := client.CoreV1().Nodes().Get(ctx, newNode.Name, metav1.GetOptions{})
				if newErr != nil {
					if !k8serrors.IsNotFound(newErr) {
						klog.Error(newErr, "node error")
						return newErr
					}
					_, newErr = client.CoreV1().Nodes().Create(ctx, newNode, metav1.CreateOptions{})
					klog.Info("new node created")
				} else {
					oldNode.Status = newNode.Status
					_, newErr = client.CoreV1().Nodes().UpdateStatus(ctx, oldNode, metav1.UpdateOptions{})
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

	cancelHTTP, err := setupHTTPServer(ctx, podProvider.PodHandler(), getAPIConfig(c))
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

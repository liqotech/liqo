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
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	"github.com/liqotech/liqo/pkg/virtualKubelet/leaderelection"
	nodeprovider "github.com/liqotech/liqo/pkg/virtualKubelet/liqoNodeProvider"
	metrics "github.com/liqotech/liqo/pkg/virtualKubelet/metrics"
	podprovider "github.com/liqotech/liqo/pkg/virtualKubelet/provider"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
)

const defaultVersion = "v1.25.0" // This should follow the version of k8s.io/kubernetes we are importing

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

	localConfig, err := utils.GetRestConfig(c.HomeKubeconfig)
	if err != nil {
		return err
	}

	restcfg.SetRateLimiter(localConfig)
	localClient := kubernetes.NewForConfigOrDie(localConfig)

	// Retrieve the remote restcfg
	tenantNamespaceManager := tenantnamespace.NewManager(localClient) // Do not use the cached version, as leveraged only once.
	identityManager := identitymanager.NewCertificateIdentityReader(localClient, c.HomeCluster, tenantNamespaceManager)

	if c.RemoteKubeconfigSecretName == "" {
		return fmt.Errorf("remote kubeconfig secret name is mandatory")
	}
	secret, err := localClient.CoreV1().Secrets(c.TenantNamespace).Get(ctx, c.RemoteKubeconfigSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("remote kubeconfig secret not found: %w", err)
		}
		return err
	}
	remoteConfig, err := identityManager.GetConfigFromSecret(secret)
	if err != nil {
		return err
	}

	reflectorsConfigs, err := getReflectorsConfigs(c)
	if err != nil {
		return err
	}

	restcfg.SetRateLimiter(remoteConfig)

	// Initialize the pod provider
	podcfg := podprovider.InitConfig{
		LocalConfig:   localConfig,
		RemoteConfig:  remoteConfig,
		LocalCluster:  c.HomeCluster,
		RemoteCluster: c.ForeignCluster,

		Namespace: c.TenantNamespace,
		NodeName:  c.NodeName,
		NodeIP:    c.NodeIP,

		LiqoIpamServer:       c.LiqoIpamServer,
		DisableIPReflection:  c.DisableIPReflection,
		InformerResyncPeriod: c.InformerResyncPeriod,

		ReflectorsConfigs: reflectorsConfigs,

		EnableAPIServerSupport:     c.EnableAPIServerSupport,
		EnableStorage:              c.EnableStorage,
		VirtualStorageClassName:    c.VirtualStorageClassName,
		RemoteRealStorageClassName: c.RemoteRealStorageClassName,
		EnableMetrics:              c.EnableMetrics,

		HomeAPIServerHost: c.HomeAPIServerHost,
		HomeAPIServerPort: c.HomeAPIServerPort,

		LabelsNotReflected:      c.LabelsNotReflected.StringList,
		AnnotationsNotReflected: c.AnnotationsNotReflected.StringList,
	}

	eb := record.NewBroadcaster()
	eb.StartRecordingToSink(&corev1clients.EventSinkImpl{Interface: localClient.CoreV1().Events(corev1.NamespaceAll)})

	podProvider, err := podprovider.NewLiqoProvider(ctx, &podcfg, eb)
	if err != nil {
		return err
	}

	leaderelectionOpts := leaderelection.Opts{
		Enabled:         c.VirtualKubeletLeaseEnabled,
		PodName:         c.PodName,
		TenantNamespace: c.TenantNamespace,
		LeaseDuration:   c.VirtualKubeletLeaseLeaseDuration,
		RenewDeadline:   c.VirtualKubeletLeaseRenewDeadline,
		RetryPeriod:     c.VirtualKubeletLeaseRetryPeriod,
	}
	if err := leaderelection.InitAndRun(ctx, leaderelectionOpts, localConfig, eb, func() {
		klog.Infof("Starting informer resync")
		if err := podProvider.Resync(); err != nil {
			klog.Errorf("Error during resync for pod provider: %s", err)
			return
		}
		klog.Infof("Resync informer completed")
	}); err != nil {
		return err
	}

	err = setupHTTPServer(ctx, podProvider.PodHandler(), localClient, remoteConfig, c)
	if err != nil {
		return fmt.Errorf("error while setting up HTTPS server: %w", err)
	}

	if c.EnableMetrics {
		metrics.SetupMetricHandler(c.MetricsAddress)
	}

	// Initialize the node provider
	nodecfg := nodeprovider.InitConfig{
		HomeConfig:      localConfig,
		RemoteConfig:    remoteConfig,
		HomeClusterID:   c.HomeCluster.ClusterID,
		RemoteClusterID: c.ForeignCluster.ClusterID,
		Namespace:       c.TenantNamespace,

		NodeName:         c.NodeName,
		InternalIP:       c.NodeIP,
		DaemonPort:       c.ListenPort,
		Version:          getVersion(localConfig),
		ExtraLabels:      c.NodeExtraLabels.StringMap,
		ExtraAnnotations: c.NodeExtraAnnotations.StringMap,

		InformerResyncPeriod: c.InformerResyncPeriod,
		PingDisabled:         c.NodePingInterval == 0,
		CheckNetworkStatus:   c.NodeCheckNetwork,
	}

	var nodeReady chan struct{}
	var nodeRunner *node.NodeController

	if c.CreateNode {
		nodeProvider := nodeprovider.NewLiqoNodeProvider(&nodecfg)
		nodeReady = nodeProvider.StartProvider(ctx)

		nodeRunner, err = node.NewNodeController(
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

		go func() {
			if err := nodeRunner.Run(ctx); err != nil {
				klog.Error(err, "error in pod controller running")
				panic(nil)
			}
		}()

		<-nodeRunner.Ready()
		close(nodeReady)
	}

	klog.Info("Setup ended")
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

func isReflectionTypeNotCustomizable(resource generic.ResourceReflected) bool {
	return resource == generic.Pod || resource == generic.ServiceAccount || resource == generic.PersistentVolumeClaim
}

func getReflectorsConfigs(c *Opts) (map[generic.ResourceReflected]*generic.ReflectorConfig, error) {
	reflectorsConfigs := make(map[generic.ResourceReflected]*generic.ReflectorConfig)
	for i := range generic.Reflectors {
		resource := &generic.Reflectors[i]
		numWorkers := *c.ReflectorsWorkers[string(*resource)]
		var reflectionType consts.ReflectionType
		if isReflectionTypeNotCustomizable(*resource) {
			reflectionType = DefaultReflectorsTypes[*resource]
		} else {
			if *resource == generic.EndpointSlice {
				// the endpointslice reflector inherits the reflection type from the service reflector.
				reflectionType = consts.ReflectionType(*c.ReflectorsType[string(generic.Service)])
			} else {
				reflectionType = consts.ReflectionType(*c.ReflectorsType[string(*resource)])
			}
			if reflectionType != consts.DenyList && reflectionType != consts.AllowList {
				return nil, fmt.Errorf("reflection type %q is not valid for resource %s. Ammitted values: %q, %q",
					reflectionType, *resource, consts.DenyList, consts.AllowList)
			}
		}
		reflectorsConfigs[*resource] = &generic.ReflectorConfig{NumWorkers: numWorkers, Type: reflectionType}
	}
	return reflectorsConfigs, nil
}

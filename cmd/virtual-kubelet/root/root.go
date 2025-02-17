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
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/leaderelection"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	nodeprovider "github.com/liqotech/liqo/pkg/virtualKubelet/liqoNodeProvider"
	metrics "github.com/liqotech/liqo/pkg/virtualKubelet/metrics"
	podprovider "github.com/liqotech/liqo/pkg/virtualKubelet/provider"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = offloadingv1beta1.AddToScheme(scheme)
	_ = liqov1beta1.AddToScheme(scheme)
	_ = networkingv1beta1.AddToScheme(scheme)
}

const defaultVersion = "v1.25.0" // This should follow the version of k8s.io/kubernetes we are importing
const leaderElectorName = "virtual-kubelet-leader-election"

// NewCommand creates a new top-level command.
// This command is used to start the virtual-kubelet daemon.
func NewCommand(ctx context.Context, name string, c *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: name + " implements the Liqo Virtual Kubelet logic.",
		Long:  name + " implements the Liqo Virtual Kubelet logic.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runRootCommand(ctx, c)
		},
	}
	return cmd
}

func runRootCommand(ctx context.Context, c *Opts) error {
	if c.ForeignCluster.GetClusterID() == "" {
		return errors.New("cluster id is mandatory")
	}
	if c.ForeignCluster.GetClusterID() == "" {
		return errors.New("cluster name is mandatory")
	}

	localConfig, err := utils.GetRestConfig(c.HomeKubeconfig)
	if err != nil {
		return err
	}

	restcfg.SetRateLimiter(localConfig)
	localClient := kubernetes.NewForConfigOrDie(localConfig)
	cl, err := client.New(localConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return err
	}

	eb := record.NewBroadcaster()
	eb.StartRecordingToSink(&corev1clients.EventSinkImpl{Interface: localClient.CoreV1().Events(corev1.NamespaceAll)})

	// active-passive leader election; blocking leader election.
	// here we have multiple virtual kubelet pods running for the same virtual node.
	// We want to avoid that multiple virtual kubelet pods reflect the same resources.
	if leader, err := leaderelection.Blocking(ctx, localConfig, eb, &leaderelection.Opts{
		PodInfo: leaderelection.PodInfo{
			PodName:   c.PodName,
			Namespace: c.TenantNamespace,
		},
		LeaderElectorName: c.NodeName,
		LeaseDuration:     c.VirtualKubeletLeaseLeaseDuration,
		RenewDeadline:     c.VirtualKubeletLeaseRenewDeadline,
		RetryPeriod:       c.VirtualKubeletLeaseRetryPeriod,
	}); err != nil {
		return err
	} else if !leader {
		klog.Error("This virtual-kubelet is not the leader")
		os.Exit(1)
	}

	// Retrieve the remote restcfg
	tenantNamespaceManager := tenantnamespace.NewManager(localClient, cl.Scheme()) // Do not use the cached version, as leveraged only once.
	identityManager := identitymanager.NewCertificateIdentityReader(ctx, cl, localClient, localConfig,
		c.HomeCluster.GetClusterID(), tenantNamespaceManager)

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
	remoteConfig, err := identityManager.GetConfigFromSecret(c.ForeignCluster.GetClusterID(), secret)
	if err != nil {
		return err
	}

	restcfg.SetRateLimiter(remoteConfig)

	// Get reflectors configurations
	reflectorsConfigs, err := getReflectorsConfigs(c)
	if err != nil {
		return err
	}

	// Get virtual node
	vnName := os.Getenv("VIRTUALNODE_NAME")
	ns := os.Getenv("POD_NAMESPACE")
	var vn offloadingv1beta1.VirtualNode
	if err := cl.Get(ctx, client.ObjectKey{Name: vnName, Namespace: ns}, &vn); err != nil {
		klog.Errorf("Unable to get virtual node: %v", err)
		return err
	}

	foreignCluster, err := fcutils.GetForeignClusterByID(ctx, cl, c.ForeignCluster.GetClusterID())
	if err != nil {
		klog.Errorf("Unable to get foreign cluster: %v", err)
		return err
	}

	var netConfiguration *networkingv1beta1.Configuration
	if fcutils.IsNetworkingModuleEnabled(foreignCluster) {
		netConfiguration, err = getters.GetConfigurationByClusterID(ctx, cl, c.ForeignCluster.GetClusterID(), corev1.NamespaceAll)
		if err != nil {
			klog.Errorf("Unable to get network configuration: %v", err)
			return err
		}
	}

	// Initialize the pod provider
	podcfg := podprovider.InitConfig{
		LocalConfig:   localConfig,
		RemoteConfig:  remoteConfig,
		LocalCluster:  c.HomeCluster.GetClusterID(),
		RemoteCluster: c.ForeignCluster.GetClusterID(),

		Namespace:     c.TenantNamespace,
		LiqoNamespace: c.LiqoNamespace,
		NodeName:      c.NodeName,
		NodeIP:        c.NodeIP,

		DisableIPReflection:  c.DisableIPReflection,
		LocalPodCIDR:         c.LocalPodCIDR,
		InformerResyncPeriod: c.InformerResyncPeriod,

		ReflectorsConfigs: reflectorsConfigs,

		EnableAPIServerSupport:          c.EnableAPIServerSupport,
		EnableStorage:                   c.EnableStorage,
		VirtualStorageClassName:         c.VirtualStorageClassName,
		RemoteRealStorageClassName:      c.RemoteRealStorageClassName,
		EnableIngress:                   c.EnableIngress,
		RemoteRealIngressClassName:      c.RemoteRealIngressClassName,
		EnableLoadBalancer:              c.EnableLoadBalancer,
		RemoteRealLoadBalancerClassName: c.RemoteRealLoadBalancerClassName,
		EnableMetrics:                   c.EnableMetrics,

		HomeAPIServerHost: c.HomeAPIServerHost,
		HomeAPIServerPort: c.HomeAPIServerPort,

		OffloadingPatch: vn.Spec.OffloadingPatch,

		NetConfiguration: netConfiguration,
	}

	podProvider, err := podprovider.NewLiqoProvider(ctx, &podcfg, eb)
	if err != nil {
		return err
	}

	initCallback := func() {
		klog.Infof("Starting informer resync")
		if err := podProvider.Resync(); err != nil {
			klog.Errorf("Error during resync for pod provider: %s", err)
			return
		}
		klog.Infof("Resync informer completed")
	}

	// The leader election avoids that multiple virtual node targeting the same cluster reflect some resources.
	// This is important when we have multiple virtual nodes targeting the same cluster.
	if c.VirtualKubeletLeaseEnabled {
		leaderelectionOpts := &leaderelection.Opts{
			PodInfo: leaderelection.PodInfo{
				PodName:   c.PodName,
				Namespace: c.TenantNamespace,
			},
			LeaderElectorName: leaderElectorName,
			LeaseDuration:     c.VirtualKubeletLeaseLeaseDuration,
			RenewDeadline:     c.VirtualKubeletLeaseRenewDeadline,
			RetryPeriod:       c.VirtualKubeletLeaseRetryPeriod,
			InitCallback:      initCallback,
			StopCallback:      nil,
		}
		leaderElector, err := leaderelection.Init(leaderelectionOpts, localConfig, eb)
		if err != nil {
			return err
		}
		go leaderelection.Run(ctx, leaderElector)
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
		HomeClusterID:   c.HomeCluster.GetClusterID(),
		RemoteClusterID: c.ForeignCluster.GetClusterID(),
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
				func(ctx context.Context, _ error) error {
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
				panic(err)
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
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		klog.Warningf("Cannot read k8s version: using default version %v; error: %v", defaultVersion, err)
		return defaultVersion
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		klog.Warningf("Cannot read k8s version: using default version %v; error: %v", defaultVersion, err)
		return defaultVersion
	}

	return version.GitVersion
}

func isReflectionTypeNotCustomizable(resource resources.ResourceReflected) bool {
	return resource == resources.Pod || resource == resources.ServiceAccount || resource == resources.PersistentVolumeClaim
}

func getReflectorsConfigs(c *Opts) (map[resources.ResourceReflected]offloadingv1beta1.ReflectorConfig, error) {
	reflectorsConfigs := make(map[resources.ResourceReflected]offloadingv1beta1.ReflectorConfig)
	for i := range resources.Reflectors {
		resource := &resources.Reflectors[i]
		numWorkers := *c.ReflectorsWorkers[string(*resource)]
		var reflectionType offloadingv1beta1.ReflectionType
		if isReflectionTypeNotCustomizable(*resource) {
			reflectionType = DefaultReflectorsTypes[*resource]
		} else {
			if *resource == resources.EndpointSlice {
				// the endpointslice reflector inherits the reflection type from the service reflector.
				reflectionType = offloadingv1beta1.ReflectionType(*c.ReflectorsType[string(resources.Service)])
			} else {
				reflectionType = offloadingv1beta1.ReflectionType(*c.ReflectorsType[string(*resource)])
			}
			if reflectionType != offloadingv1beta1.DenyList && reflectionType != offloadingv1beta1.AllowList {
				return nil, fmt.Errorf("reflection type %q is not valid for resource %s. Ammitted values: %q, %q",
					reflectionType, *resource, offloadingv1beta1.DenyList, offloadingv1beta1.AllowList)
			}
		}
		reflectorsConfigs[*resource] = offloadingv1beta1.ReflectorConfig{NumWorkers: numWorkers, Type: reflectionType}
	}
	return reflectorsConfigs, nil
}

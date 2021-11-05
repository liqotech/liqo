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
	"path"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/discovery"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/internal/utils/errdefs"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	nodeprovider "github.com/liqotech/liqo/pkg/virtualKubelet/liqoNodeProvider"
	"github.com/liqotech/liqo/pkg/virtualKubelet/manager"
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

	if c.PodSyncWorkers == 0 {
		return errdefs.InvalidInput("pod sync workers must be greater than 0")
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

	// Create a shared informer factory for Kubernetes pods in the current namespace (if specified) and scheduled to the current node.
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		client,
		c.InformerResyncPeriod,
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", c.NodeName).String()
		}))
	podInformer := podInformerFactory.Core().V1().Pods()

	// Create another shared informer factory for Kubernetes secrets and configmaps (not subject to any selectors).
	scmInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(client, c.InformerResyncPeriod)
	// Create a secret informer and a config map informer so we can pass their listers to the resource manager.
	secretInformer := scmInformerFactory.Core().V1().Secrets()
	configMapInformer := scmInformerFactory.Core().V1().ConfigMaps()
	serviceInformer := scmInformerFactory.Core().V1().Services()

	rm, err := manager.NewResourceManager(podInformer.Lister(), secretInformer.Lister(), configMapInformer.Lister(), serviceInformer.Lister())
	if err != nil {
		return errors.Wrap(err, "could not create resource manager")
	}

	// Initialize the pod provider
	podcfg := podprovider.InitConfig{
		HomeConfig:      config,
		HomeClusterID:   c.HomeClusterID,
		RemoteClusterID: c.ForeignClusterID,

		Namespace: c.KubeletNamespace,
		NodeName:  c.NodeName,

		LiqoIpamServer:       c.LiqoIpamServer,
		InformerResyncPeriod: c.InformerResyncPeriod,

		ServiceWorkers:       c.ServiceWorkers,
		EndpointSliceWorkers: c.EndpointSliceWorkers,
		ConfigMapWorkers:     c.ConfigMapWorkers,
		SecretWorkers:        c.SecretWorkers,
	}

	podProvider, err := podprovider.NewLiqoProvider(ctx, &podcfg)
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

	eb := record.NewBroadcaster()
	pc, err := node.NewPodController(node.PodControllerConfig{
		PodClient:                            client.CoreV1(),
		PodInformer:                          podInformer,
		EventRecorder:                        eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: path.Join(c.NodeName, "pod-controller")}),
		Provider:                             podProvider,
		SecretInformer:                       secretInformer,
		ConfigMapInformer:                    configMapInformer,
		ServiceInformer:                      serviceInformer,
		SyncPodsFromKubernetesRateLimiter:    newPodControllerWorkqueueRateLimiter(),
		SyncPodStatusFromProviderRateLimiter: newPodControllerWorkqueueRateLimiter(),
		DeletePodsFromKubernetesRateLimiter:  newPodControllerWorkqueueRateLimiter(),
	})
	if err != nil {
		return errors.Wrap(err, "error setting up pod controller")
	}

	go podInformerFactory.Start(ctx.Done())
	go scmInformerFactory.Start(ctx.Done())

	cancelHTTP, err := setupHTTPServer(ctx, podProvider, getAPIConfig(c), func(context.Context) ([]*corev1.Pod, error) {
		return rm.GetPods(), nil
	})
	if err != nil {
		return errors.Wrap(err, "error while setting up HTTP server")
	}
	defer cancelHTTP()

	go func() {
		if err := pc.Run(ctx, int(c.PodSyncWorkers)); err != nil && errors.Is(err, context.Canceled) {
			klog.Fatal(errors.Wrap(err, "error in pod controller running"))
		}
	}()

	if c.StartupTimeout > 0 {
		ctx, cancel := context.WithTimeout(ctx, c.StartupTimeout)
		klog.Info("Waiting for pod controller / VK to be ready")
		select {
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case <-pc.Ready():
		}
		cancel()
		if err := pc.Err(); err != nil {
			return err
		}
	}

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

// newPodControllerWorkqueueRateLimiter returns a new custom rate limiter to be assigned to the pod controller workqueues.
// Differently from the standard workqueue.DefaultControllerRateLimiter(), composed of an overall bucket rate limiter
// and a per-item exponential rate limiter to address failures, this includes only the latter component. Hance avoiding
// performance limitations when processing a high number of pods in parallel.
func newPodControllerWorkqueueRateLimiter() workqueue.RateLimiter {
	return workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second)
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

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
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	coordv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/liqotech/liqo/cmd/virtual-kubelet/provider"
	"github.com/liqotech/liqo/internal/utils/errdefs"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	liqonodeprovider "github.com/liqotech/liqo/pkg/virtualKubelet/liqoNodeProvider"
	"github.com/liqotech/liqo/pkg/virtualKubelet/manager"
	liqoprovider "github.com/liqotech/liqo/pkg/virtualKubelet/provider"
)

// NewCommand creates a new top-level command.
// This command is used to start the virtual-kubelet daemon.
func NewCommand(ctx context.Context, name string, s *provider.Store, c *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: name + " provides a virtual kubelet interface for your kubernetes cluster.",
		Long: name + ` implements the Kubelet interface with a pluggable
backend implementation allowing users to create kubernetes nodes without running the kubelet.
This allows users to schedule kubernetes workloads on nodes that aren't running Kubernetes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootCommand(ctx, s, c)
		},
	}
	return cmd
}

func runRootCommand(ctx context.Context, s *provider.Store, c *Opts) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if c.ForeignClusterID == "" {
		return errors.New("cluster id is mandatory")
	}

	if c.PodSyncWorkers == 0 {
		return errdefs.InvalidInput("pod sync workers must be greater than 0")
	}

	config, err := crdclient.NewKubeconfig(c.HomeKubeconfig, nil, func(config *rest.Config) {
		config.QPS = virtualKubelet.HOME_CLIENT_QPS
		config.Burst = virtualKubelet.HOME_CLIENTS_BURST
	})
	if err != nil {
		return err
	}

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

	apiConfig, err := getAPIConfig(*c)
	if err != nil {
		return err
	}

	initConfig := provider.InitConfig{
		HomeKubeConfig:       c.HomeKubeconfig,
		NodeName:             c.NodeName,
		ResourceManager:      rm,
		DaemonPort:           c.ListenPort,
		InternalIP:           os.Getenv("VKUBELET_POD_IP"),
		KubeClusterDomain:    c.KubeClusterDomain,
		RemoteClusterID:      c.ForeignClusterID,
		HomeClusterID:        c.HomeClusterID,
		InformerResyncPeriod: c.InformerResyncPeriod,
		LiqoIpamServer:       c.LiqoIpamServer,
	}

	pInit := s.Get(c.Provider)
	if pInit == nil {
		return errors.Errorf("provider %q not found", c.Provider)
	}

	p, err := pInit(initConfig)
	if err != nil {
		return errors.Wrapf(err, "error initializing provider %s", c.Provider)
	}

	var leaseClient coordv1.LeaseInterface
	if c.EnableNodeLease {
		leaseClient = client.CoordinationV1().Leases(corev1.NamespaceNodeLease)
	}

	var nodeRunner *node.NodeController

	pNode, err := NodeFromProvider(ctx, c.NodeName, p, c.Version, []metav1.OwnerReference{})
	if err != nil {
		klog.Fatal(err)
	}

	var nodeReady chan struct{}
	var nodeProvider node.NodeProvider
	if liqoProvider, ok := p.(*liqoprovider.LiqoProvider); ok {
		podProviderStopper := make(chan struct{}, 1)
		liqoProvider.SetProviderStopper(podProviderStopper)
		networkReadyChan := liqoProvider.GetNetworkReadyChan()

		liqoNodeProvider, err := liqonodeprovider.NewLiqoNodeProvider(c.NodeName, c.ForeignClusterID,
			c.KubeletNamespace, pNode, podProviderStopper, networkReadyChan, nil, c.LiqoInformerResyncPeriod)
		if err != nil {
			klog.Fatal(err)
		}

		nodeReady, _ = liqoNodeProvider.StartProvider()
		nodeProvider = liqoNodeProvider
	} else {
		nodeProvider = node.NaiveNodeProvider{}
	}

	nodeRunner, err = node.NewNodeController(
		nodeProvider,
		pNode,
		client.CoreV1().Nodes(),
		node.WithNodeEnableLeaseV1(leaseClient, node.DefaultLeaseDuration),
		node.WithNodeStatusUpdateErrorHandler(
			func(ctx context.Context, err error) error {
				klog.Info("node setting up")
				newNode := pNode.DeepCopy()
				newNode.ResourceVersion = ""

				if liqoNodeProvider, ok := nodeProvider.(*liqonodeprovider.LiqoNodeProvider); ok {
					if liqoNodeProvider.IsTerminating() {
						// this avoids the re-creation of terminated nodes
						klog.V(4).Info("skipping: node is in terminating phase")
						return nil
					}
				}

				oldNode, newErr := client.CoreV1().Nodes().Get(context.TODO(), newNode.Name, metav1.GetOptions{})
				if newErr != nil {
					if !k8serrors.IsNotFound(newErr) {
						klog.Error(newErr, "node error")
						return newErr
					}
					_, newErr = client.CoreV1().Nodes().Create(context.TODO(), newNode, metav1.CreateOptions{})
					klog.Info("new node created")
				} else {
					oldNode.Status = newNode.Status
					_, newErr = client.CoreV1().Nodes().UpdateStatus(context.TODO(), oldNode, metav1.UpdateOptions{})
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
		klog.Fatal("cannot create the node controller")
	}

	eb := record.NewBroadcaster()

	pc, err := node.NewPodController(node.PodControllerConfig{
		PodClient:                            client.CoreV1(),
		PodInformer:                          podInformer,
		EventRecorder:                        eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: path.Join(pNode.Name, "pod-controller")}),
		Provider:                             p,
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

	cancelHTTP, err := setupHTTPServer(ctx, p, apiConfig, func(context.Context) ([]*corev1.Pod, error) {
		return rm.GetPods(), nil
	})
	if err != nil {
		klog.Fatal(errors.Wrap(err, "error while setting up HTTP server"))
	}
	defer cancelHTTP()

	go func() {
		if err := pc.Run(ctx, c.PodSyncWorkers); err != nil && errors.Is(err, context.Canceled) {
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

	klog.Info("setup ended")
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

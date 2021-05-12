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

package root

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/provider"
	"github.com/liqotech/liqo/internal/utils/errdefs"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/node/module"
	nodeProvider "github.com/liqotech/liqo/pkg/virtualKubelet/node/provider"
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

	if c.ForeignKubeconfig == "" {
		return errors.New("provider kubeconfig is mandatory")
	}

	if c.ForeignClusterId == "" {
		return errors.New("cluster id is mandatory")
	}

	if c.PodSyncWorkers == 0 {
		return errdefs.InvalidInput("pod sync workers must be greater than 0")
	}

	client, err := v1alpha1.CreateAdvertisementClient(c.HomeKubeconfig, nil, false, func(config *rest.Config) {
		config.QPS = virtualKubelet.HOME_CLIENT_QPS
		config.Burst = virtualKubelet.HOME_CLIENTS_BURST
	})
	if err != nil {
		return err
	}

	// Create a shared informer factory for Kubernetes pods in the current namespace (if specified) and scheduled to the current node.
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		client.Client(),
		c.InformerResyncPeriod,
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", c.NodeName).String()
		}))
	podInformer := podInformerFactory.Core().V1().Pods()

	// Create another shared informer factory for Kubernetes secrets and configmaps (not subject to any selectors).
	scmInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(client.Client(), c.InformerResyncPeriod)
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
		RemoteClusterID:      c.ForeignClusterId,
		HomeClusterID:        c.HomeClusterId,
		RemoteKubeConfig:     c.ForeignKubeconfig,
		InformerResyncPeriod: c.InformerResyncPeriod,
	}

	pInit := s.Get(c.Provider)
	if pInit == nil {
		return errors.Errorf("provider %q not found", c.Provider)
	}

	p, err := pInit(initConfig)
	if err != nil {
		return errors.Wrapf(err, "error initializing provider %s", c.Provider)
	}

	var leaseClient v1beta1.LeaseInterface
	if c.EnableNodeLease {
		leaseClient = client.Client().CoordinationV1beta1().Leases(corev1.NamespaceNodeLease)
	}

	advName := strings.Join([]string{virtualKubelet.AdvertisementPrefix, c.ForeignClusterId}, "")
	refs := createOwnerReference(client, advName, "")

	var nodeRunner *module.NodeController

	pNode, err := nodeProvider.NodeFromProvider(ctx, c.NodeName, p, c.Version, refs)
	if err != nil {
		klog.Fatal(err)
	}

	nodeRunner, err = module.NewNodeController(
		module.NaiveNodeProvider{},
		pNode,
		client.Client().CoreV1().Nodes(),
		module.WithNodeEnableLeaseV1Beta1(leaseClient, nil),
		module.WithNodeStatusUpdateErrorHandler(
			func(ctx context.Context, err error) error {
				klog.Info("node setting up")
				newNode := pNode.DeepCopy()
				newNode.ResourceVersion = ""

				if len(refs) > 0 {
					newNode.SetOwnerReferences(refs)
				}

				oldNode, newErr := client.Client().CoreV1().Nodes().Get(context.TODO(), newNode.Name, metav1.GetOptions{})
				if newErr != nil {
					if !k8serrors.IsNotFound(newErr) {
						klog.Error(newErr, "node error")
						return newErr
					}
					_, newErr = client.Client().CoreV1().Nodes().Create(context.TODO(), newNode, metav1.CreateOptions{})
					klog.Info("new node created")
				} else {
					oldNode.Status = newNode.Status
					_, newErr = client.Client().CoreV1().Nodes().UpdateStatus(context.TODO(), oldNode, metav1.UpdateOptions{})
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

	nodeReady, _, err := p.StartNodeUpdater(nodeRunner)
	if err != nil {
		klog.Fatal(err)
	}

	eb := record.NewBroadcaster()

	pc, err := module.NewPodController(module.PodControllerConfig{
		PodClient:                            client.Client().CoreV1(),
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

	nodeRunner.Ready()

	klog.Info("setup ended")
	close(nodeReady)
	<-ctx.Done()
	return nil
}

func createOwnerReference(c *crdclient.CRDClient, advName, namespace string) []metav1.OwnerReference {
	d, err := c.Resource("advertisements").Namespace(namespace).Get(advName, &metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Info("advertisement not found, setting empty owner reference")
		}
		return []metav1.OwnerReference{}
	}

	return []metav1.OwnerReference{
		{
			APIVersion: fmt.Sprintf("%s/%s", v1alpha1.GroupVersion.Group, v1alpha1.GroupVersion.Version),
			Kind:       "Advertisement",
			Name:       advName,
			UID:        d.(metav1.Object).GetUID(),
		},
	}
}

// newPodControllerWorkqueueRateLimiter returns a new custom rate limiter to be assigned to the pod controller workqueues.
// Differently from the standard workqueue.DefaultControllerRateLimiter(), composed of an overall bucket rate limiter
// and a per-item exponential rate limiter to address failures, this includes only the latter component. Hance avoiding
// performance limitations when processing a high number of pods in parallel.
func newPodControllerWorkqueueRateLimiter() workqueue.RateLimiter {
	return workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second)
}

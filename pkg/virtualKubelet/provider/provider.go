// Copyright 2019-2022 The Liqo Authors
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

package provider

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	vkalpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	"github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/configuration"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/exposition"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/namespacemap"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/storage"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/workload"
)

func init() {
	utilruntime.Must(vkalpha1.AddToScheme(scheme.Scheme))
}

// InitConfig is the config passed to initialize the LiqoPodProvider.
type InitConfig struct {
	HomeConfig    *rest.Config
	RemoteConfig  *rest.Config
	HomeCluster   discoveryv1alpha1.ClusterIdentity
	RemoteCluster discoveryv1alpha1.ClusterIdentity
	Namespace     string

	NodeName             string
	NodeIP               string
	LiqoIpamServer       string
	InformerResyncPeriod time.Duration

	PodWorkers                  uint
	ServiceWorkers              uint
	EndpointSliceWorkers        uint
	PersistenVolumeClaimWorkers uint
	ConfigMapWorkers            uint
	SecretWorkers               uint

	EnableStorage              bool
	VirtualStorageClassName    string
	RemoteRealStorageClassName string
}

// LiqoProvider implements the virtual-kubelet provider interface and stores pods in memory.
type LiqoProvider struct {
	reflectionManager manager.Manager
	podHandler        workload.PodHandler
}

// NewLiqoProvider creates a new NewLiqoProvider instance.
func NewLiqoProvider(ctx context.Context, cfg *InitConfig, eb record.EventBroadcaster) (*LiqoProvider, error) {
	forge.Init(cfg.HomeCluster.ClusterID, cfg.RemoteCluster.ClusterID, cfg.NodeName, cfg.NodeIP)
	homeClient := kubernetes.NewForConfigOrDie(cfg.HomeConfig)
	homeLiqoClient := liqoclient.NewForConfigOrDie(cfg.HomeConfig)

	foreignClient := kubernetes.NewForConfigOrDie(cfg.RemoteConfig)
	foreignLiqoClient := liqoclient.NewForConfigOrDie(cfg.RemoteConfig)
	foreignMetricsClient := metrics.NewForConfigOrDie(cfg.RemoteConfig)

	dialctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	connection, err := grpc.DialContext(dialctx, cfg.LiqoIpamServer, grpc.WithInsecure(), grpc.WithBlock())
	cancel()
	if err != nil {
		return nil, errors.Wrap(err, "failed to establish a connection to the IPAM")
	}
	ipamClient := ipam.NewIpamClient(connection)

	reflectionManager := manager.New(homeClient, foreignClient, homeLiqoClient, foreignLiqoClient, cfg.InformerResyncPeriod, eb)
	podreflector := workload.NewPodReflector(cfg.RemoteConfig, foreignMetricsClient.MetricsV1beta1().PodMetricses, ipamClient, cfg.PodWorkers)
	namespaceMapHandler := namespacemap.NewHandler(homeLiqoClient, cfg.Namespace, cfg.InformerResyncPeriod)
	reflectionManager.
		With(exposition.NewServiceReflector(cfg.ServiceWorkers)).
		With(exposition.NewEndpointSliceReflector(ipamClient, cfg.EndpointSliceWorkers)).
		With(configuration.NewConfigMapReflector(cfg.ConfigMapWorkers)).
		With(configuration.NewSecretReflector(cfg.SecretWorkers)).
		With(podreflector).
		With(storage.NewPersistentVolumeClaimReflector(cfg.PersistenVolumeClaimWorkers,
			cfg.VirtualStorageClassName, cfg.RemoteRealStorageClassName, cfg.EnableStorage)).
		WithNamespaceHandler(namespaceMapHandler)

	reflectionManager.Start(ctx)

	return &LiqoProvider{
		reflectionManager: reflectionManager,
		podHandler:        podreflector,
	}, nil
}

// PodHandler returns an handler to interact with the pods offloaded to the remote cluster.
func (p *LiqoProvider) PodHandler() workload.PodHandler {
	return p.podHandler
}

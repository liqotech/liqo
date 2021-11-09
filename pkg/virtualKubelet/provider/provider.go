// Copyright 2019-2021 The Liqo Authors
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
	"k8s.io/klog/v2"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	vkalpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/liqonet/ipam"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/controller"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/configuration"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/exposition"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/storage"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/workload"
)

func init() {
	utilruntime.Must(vkalpha1.AddToScheme(scheme.Scheme))
}

// InitConfig is the config passed to initialize the LiqoPodProvider.
type InitConfig struct {
	HomeConfig      *rest.Config
	HomeClusterID   string
	RemoteClusterID string
	Namespace       string

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
	namespaceMapper   namespacesmapping.MapperController
	reflectionManager manager.Manager
	podHandler        workload.PodHandler
	apiController     controller.APIController
}

// NewLiqoProvider creates a new NewLiqoProvider instance.
func NewLiqoProvider(ctx context.Context, cfg *InitConfig, eb record.EventBroadcaster) (*LiqoProvider, error) {
	forge.Init(cfg.HomeClusterID, cfg.RemoteClusterID, cfg.NodeName, cfg.NodeIP)
	homeClient := kubernetes.NewForConfigOrDie(cfg.HomeConfig)
	homeLiqoClient := liqoclient.NewForConfigOrDie(cfg.HomeConfig)

	tenantNamespaceManager := tenantnamespace.NewTenantNamespaceManager(homeClient)
	identityManager := identitymanager.NewCertificateIdentityReader(homeClient, cfg.HomeClusterID, tenantNamespaceManager)

	remoteRestConfig, err := identityManager.GetConfig(cfg.RemoteClusterID, "")
	if err != nil {
		return nil, err
	}

	restcfg.SetRateLimiterWithCustomParamenters(remoteRestConfig, virtualKubelet.FOREIGN_CLIENT_QPS, virtualKubelet.FOREIGN_CLIENT_BURST)
	foreignClient, err := kubernetes.NewForConfig(remoteRestConfig)
	if err != nil {
		return nil, err
	}

	foreignLiqoClient, err := liqoclient.NewForConfig(remoteRestConfig)
	if err != nil {
		return nil, err
	}

	foreignMetricsClient, err := metrics.NewForConfig(remoteRestConfig)
	if err != nil {
		return nil, err
	}

	dialctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	connection, err := grpc.DialContext(dialctx, cfg.LiqoIpamServer, grpc.WithInsecure(), grpc.WithBlock())
	cancel()
	if err != nil {
		return nil, errors.Wrap(err, "failed to establish a connection to the IPAM")
	}
	ipamClient := ipam.NewIpamClient(connection)

	// TODO: make the resync period configurable. This is currently hardcoded since the one specified as part of
	// the configuration needs to be very low to avoid issues with the legacy reflection.
	reflectionManager := manager.New(homeClient, foreignClient, homeLiqoClient, foreignLiqoClient, 10*time.Hour, eb)
	podreflector := workload.NewPodReflector(remoteRestConfig, foreignMetricsClient.MetricsV1beta1().PodMetricses, ipamClient, cfg.PodWorkers)
	reflectionManager.
		With(exposition.NewServiceReflector(cfg.ServiceWorkers)).
		With(exposition.NewEndpointSliceReflector(ipamClient, cfg.EndpointSliceWorkers)).
		With(configuration.NewConfigMapReflector(cfg.ConfigMapWorkers)).
		With(configuration.NewSecretReflector(cfg.SecretWorkers)).
		With(podreflector).
		With(storage.NewPersistentVolumeClaimReflector(cfg.PersistenVolumeClaimWorkers,
			cfg.VirtualStorageClassName, cfg.RemoteRealStorageClassName, cfg.EnableStorage))
	reflectionManager.Start(ctx)

	mapper, err := namespacesmapping.NewNamespaceMapperController(ctx, cfg.HomeConfig, cfg.HomeClusterID,
		cfg.RemoteClusterID, cfg.Namespace, reflectionManager)
	if err != nil {
		return nil, err
	}
	mapper.WaitForSync()

	// All namespaces with active reflection have been detected, and we can start the podreflector default management for all namespaces.
	podreflector.StartAllNamespaces()

	return &LiqoProvider{
		namespaceMapper:   mapper,
		reflectionManager: reflectionManager,
		podHandler:        podreflector,
		apiController:     controller.NewAPIController(homeClient, foreignClient, cfg.InformerResyncPeriod, mapper, nil),
	}, nil
}

// PodHandler returns an handler to interact with the pods offloaded to the remote cluster.
func (p *LiqoProvider) PodHandler() workload.PodHandler {
	return p.podHandler
}

// SetProviderStopper sets the provided chan as the stopper for the API reflector.
func (p *LiqoProvider) SetProviderStopper(stopper chan struct{}) {
	go func() {
		<-stopper
		if err := p.apiController.StopController(); err != nil {
			klog.Error(err)
		}
	}()
}

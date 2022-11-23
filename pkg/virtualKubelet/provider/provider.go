// Copyright 2019-2023 The Liqo Authors
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
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
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
	LocalConfig   *rest.Config
	RemoteConfig  *rest.Config
	LocalCluster  discoveryv1alpha1.ClusterIdentity
	RemoteCluster discoveryv1alpha1.ClusterIdentity
	Namespace     string

	NodeName             string
	NodeIP               string
	LiqoIpamServer       string
	InformerResyncPeriod time.Duration

	PodWorkers                  uint
	ServiceWorkers              uint
	EndpointSliceWorkers        uint
	IngressWorkers              uint
	PersistenVolumeClaimWorkers uint
	ConfigMapWorkers            uint
	SecretWorkers               uint
	ServiceAccountWorkers       uint

	EnableAPIServerSupport     bool
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
	forge.Init(cfg.LocalCluster, cfg.RemoteCluster, cfg.NodeName, cfg.NodeIP)
	localClient := kubernetes.NewForConfigOrDie(cfg.LocalConfig)
	localLiqoClient := liqoclient.NewForConfigOrDie(cfg.LocalConfig)

	remoteClient := kubernetes.NewForConfigOrDie(cfg.RemoteConfig)
	remoteLiqoClient := liqoclient.NewForConfigOrDie(cfg.RemoteConfig)
	remoteMetricsClient := metrics.NewForConfigOrDie(cfg.RemoteConfig).MetricsV1beta1().PodMetricses

	dialctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	connection, err := grpc.DialContext(dialctx, cfg.LiqoIpamServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	cancel()
	if err != nil {
		return nil, errors.Wrap(err, "failed to establish a connection to the IPAM")
	}
	ipamClient := ipam.NewIpamClient(connection)

	apiServerSupport := forge.APIServerSupportDisabled
	if cfg.EnableAPIServerSupport {
		tokenAPISupported, err := isSATokenAPISupport(localClient)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check whether service account token API is supported")
		}

		apiServerSupport = forge.APIServerSupportLegacy
		if tokenAPISupported {
			apiServerSupport = forge.APIServerSupportTokenAPI
		}
		klog.V(4).Infof("Enabled support for local API server interactions (%v mode)", apiServerSupport)
	}

	reflectionManager := manager.New(localClient, remoteClient, localLiqoClient, remoteLiqoClient, cfg.InformerResyncPeriod, eb)
	podreflector := workload.NewPodReflector(cfg.RemoteConfig, remoteMetricsClient, ipamClient, apiServerSupport, cfg.PodWorkers)
	namespaceMapHandler := namespacemap.NewHandler(localLiqoClient, cfg.Namespace, cfg.InformerResyncPeriod)
	reflectionManager.
		With(exposition.NewServiceReflector(cfg.ServiceWorkers)).
		With(exposition.NewEndpointSliceReflector(ipamClient, cfg.EndpointSliceWorkers)).
		With(exposition.NewIngressReflector(cfg.IngressWorkers)).
		With(configuration.NewConfigMapReflector(cfg.ConfigMapWorkers)).
		With(configuration.NewSecretReflector(apiServerSupport == forge.APIServerSupportLegacy, cfg.SecretWorkers)).
		With(configuration.NewServiceAccountReflector(apiServerSupport == forge.APIServerSupportTokenAPI, cfg.ServiceAccountWorkers)).
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

func isSATokenAPISupport(localClient kubernetes.Interface) (bool, error) {
	resources, err := localClient.Discovery().ServerResourcesForGroupVersion(corev1.SchemeGroupVersion.String())
	if err != nil {
		return false, err
	}

	for i := range resources.APIResources {
		if resources.APIResources[i].Name == "serviceaccounts/token" {
			return true, nil
		}
	}

	return false, nil
}

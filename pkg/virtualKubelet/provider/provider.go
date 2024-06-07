// Copyright 2019-2024 The Liqo Authors
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	vkalpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/configuration"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/event"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/exposition"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
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
	LocalCluster  discoveryv1alpha1.ClusterID
	RemoteCluster discoveryv1alpha1.ClusterID
	Namespace     string
	LiqoNamespace string

	NodeName             string
	NodeIP               string
	DisableIPReflection  bool
	LocalPodCIDR         string
	InformerResyncPeriod time.Duration

	ReflectorsConfigs map[generic.ResourceReflected]*generic.ReflectorConfig

	EnableAPIServerSupport          bool
	EnableStorage                   bool
	VirtualStorageClassName         string
	RemoteRealStorageClassName      string
	EnableIngress                   bool
	RemoteRealIngressClassName      string
	EnableLoadBalancer              bool
	RemoteRealLoadBalancerClassName string
	EnableMetrics                   bool

	HomeAPIServerHost string
	HomeAPIServerPort string

	OffloadingPatch *vkalpha1.OffloadingPatch

	NetConfiguration *networkingv1alpha1.Configuration // only available if network module is enabled
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

	podReflectorConfig := workload.PodReflectorConfig{
		APIServerSupport:    apiServerSupport,
		DisableIPReflection: cfg.DisableIPReflection,
		HomeAPIServerHost:   cfg.HomeAPIServerHost,
		HomeAPIServerPort:   cfg.HomeAPIServerPort,
		KubernetesServiceIPMapper: func(ctx context.Context) (string, error) {
			ip, err := localLiqoClient.IpamV1alpha1().IPs(cfg.LiqoNamespace).Get(ctx, "api-server", metav1.GetOptions{})
			if err != nil {
				return "", err
			}

			if ip.Status.IPMappings == nil {
				return "", errors.New("no IP mappings found for the API server")
			}

			v, ok := ip.Status.IPMappings[string(cfg.RemoteCluster)]
			if !ok {
				return "", errors.New("no IP mapping found for the remote cluster API server")
			}

			return string(v), nil
		},
		NetConfiguration: cfg.NetConfiguration,
	}

	podreflector := workload.NewPodReflector(cfg.RemoteConfig, remoteMetricsClient, &podReflectorConfig, cfg.ReflectorsConfigs[generic.Pod])

	forgingOpts := forge.NewForgingOpts(cfg.OffloadingPatch)

	reflectionManager := manager.New(localClient, remoteClient, localLiqoClient, remoteLiqoClient, cfg.InformerResyncPeriod, eb, &forgingOpts).
		With(podreflector).
		With(exposition.NewServiceReflector(cfg.ReflectorsConfigs[generic.Service], cfg.EnableLoadBalancer, cfg.RemoteRealLoadBalancerClassName)).
		With(exposition.NewIngressReflector(cfg.ReflectorsConfigs[generic.Ingress], cfg.EnableIngress, cfg.RemoteRealIngressClassName)).
		With(configuration.NewConfigMapReflector(cfg.ReflectorsConfigs[generic.ConfigMap])).
		With(configuration.NewSecretReflector(apiServerSupport == forge.APIServerSupportLegacy, cfg.ReflectorsConfigs[generic.Secret])).
		With(configuration.NewServiceAccountReflector(apiServerSupport == forge.APIServerSupportTokenAPI, cfg.ReflectorsConfigs[generic.ServiceAccount])).
		With(storage.NewPersistentVolumeClaimReflector(cfg.VirtualStorageClassName, cfg.RemoteRealStorageClassName,
			cfg.EnableStorage, cfg.ReflectorsConfigs[generic.PersistentVolumeClaim])).
		With(event.NewEventReflector(cfg.ReflectorsConfigs[generic.Event])).
		WithNamespaceHandler(namespacemap.NewHandler(localLiqoClient, cfg.Namespace, cfg.InformerResyncPeriod))

	if !cfg.DisableIPReflection {
		reflectionManager.With(exposition.NewEndpointSliceReflector(cfg.LocalPodCIDR, cfg.ReflectorsConfigs[generic.EndpointSlice]))
	}

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

// Resync force the resync of all informers contained in the reflection manager.
func (p *LiqoProvider) Resync() error {
	return p.reflectionManager.Resync()
}

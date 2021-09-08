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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	vkalpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterid"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/controller"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	optTypes "github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
)

// LiqoProvider implements the virtual-kubelet provider interface and stores pods in memory.
type LiqoProvider struct {
	namespaceMapper namespacesmapping.MapperController
	apiController   controller.APIController

	tepReady             chan struct{}
	homeClient           kubernetes.Interface
	foreignClient        kubernetes.Interface
	foreignMetricsClient metricsv.Interface

	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	startTime          time.Time
	homeClusterID      string
	foreignClusterID   string
	foreignRestConfig  *rest.Config

	nodeName options.Option

	foreignPodWatcherStop chan struct{}
}

// NewLiqoProvider creates a new NewLiqoProvider instance.
func NewLiqoProvider(ctx context.Context, nodeName, foreignClusterID, homeClusterID, internalIP string, daemonEndpointPort int32,
	kubeconfig string, informerResyncPeriod time.Duration, ipamGRPCServer string) (*LiqoProvider, error) {
	var err error

	if err = vkalpha1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	homeRestConfig, err := utils.UserConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	homeRestConfig.QPS = virtualKubelet.HOME_CLIENT_QPS
	homeRestConfig.Burst = virtualKubelet.HOME_CLIENTS_BURST

	homeClient, err := kubernetes.NewForConfig(homeRestConfig)
	if err != nil {
		return nil, err
	}

	clusterID := clusterid.NewStaticClusterID(homeClusterID)
	tenantNamespaceManager := tenantnamespace.NewTenantNamespaceManager(homeClient)
	identityManager := identitymanager.NewCertificateIdentityReader(homeClient, clusterID, tenantNamespaceManager)
	namespace, err := utils.RetrieveNamespace()
	if err != nil {
		return nil, err
	}

	remoteRestConfig, err := identityManager.GetConfig(foreignClusterID, "")
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	remoteRestConfig.QPS = virtualKubelet.FOREIGN_CLIENT_QPS
	remoteRestConfig.Burst = virtualKubelet.FOREIGN_CLIENT_BURST

	foreignClient, err := kubernetes.NewForConfig(remoteRestConfig)
	if err != nil {
		return nil, err
	}

	foreignMetricsClient, err := metricsv.NewForConfig(remoteRestConfig)
	if err != nil {
		return nil, err
	}

	mapper, err := namespacesmapping.NewNamespaceMapperController(ctx, homeRestConfig, homeClusterID, foreignClusterID, namespace)
	if err != nil {
		klog.Fatal(err)
	}
	mapper.WaitForSync()

	virtualNodeNameOpt := optTypes.NewNetworkingOption(optTypes.VirtualNodeName, optTypes.NetworkingValue(nodeName))
	grpcServerNameOpt := optTypes.NewNetworkingOption(optTypes.LiqoIpamServer, optTypes.NetworkingValue(ipamGRPCServer))
	remoteClusterIDOpt := optTypes.NewNetworkingOption(optTypes.RemoteClusterID, optTypes.NetworkingValue(foreignClusterID))

	forge.InitForger(mapper, virtualNodeNameOpt, grpcServerNameOpt, remoteClusterIDOpt)

	opts := forgeOptionsMap(
		virtualNodeNameOpt,
		grpcServerNameOpt)

	tepReady := make(chan struct{})

	provider := LiqoProvider{
		apiController:         controller.NewAPIController(homeClient, foreignClient, informerResyncPeriod, mapper, opts, tepReady),
		namespaceMapper:       mapper,
		nodeName:              virtualNodeNameOpt,
		internalIP:            internalIP,
		daemonEndpointPort:    daemonEndpointPort,
		startTime:             time.Now(),
		foreignClusterID:      foreignClusterID,
		homeClusterID:         homeClusterID,
		foreignPodWatcherStop: make(chan struct{}, 1),
		foreignRestConfig:     remoteRestConfig,
		homeClient:            homeClient,
		foreignClient:         foreignClient,
		foreignMetricsClient:  foreignMetricsClient,
		tepReady:              tepReady,
	}

	return &provider, nil
}

func forgeOptionsMap(opts ...options.Option) map[options.OptionKey]options.Option {
	outOpts := make(map[options.OptionKey]options.Option)

	for _, o := range opts {
		outOpts[o.Key()] = o
	}

	return outOpts
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

// GetNetworkReadyChan reetrun the chan where to notify that the network connectivity has been established.
func (p *LiqoProvider) GetNetworkReadyChan() chan struct{} {
	return p.tepReady
}

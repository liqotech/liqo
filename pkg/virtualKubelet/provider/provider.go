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

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	vkalpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/controller"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	optTypes "github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
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
	LiqoIpamServer       string
	InformerResyncPeriod time.Duration
}

// LiqoProvider implements the virtual-kubelet provider interface and stores pods in memory.
type LiqoProvider struct {
	homeClient           kubernetes.Interface
	foreignClient        kubernetes.Interface
	foreignMetricsClient metrics.Interface
	foreignRestConfig    *rest.Config

	namespaceMapper namespacesmapping.MapperController
	apiController   controller.APIController

	startTime time.Time
	nodeName  string
}

// NewLiqoProvider creates a new NewLiqoProvider instance.
func NewLiqoProvider(ctx context.Context, cfg *InitConfig) (*LiqoProvider, error) {
	homeClient := kubernetes.NewForConfigOrDie(cfg.HomeConfig)

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

	foreignMetricsClient, err := metrics.NewForConfig(remoteRestConfig)
	if err != nil {
		return nil, err
	}

	mapper, err := namespacesmapping.NewNamespaceMapperController(
		ctx, cfg.HomeConfig, cfg.HomeClusterID, cfg.RemoteClusterID, cfg.Namespace)
	if err != nil {
		return nil, err
	}
	mapper.WaitForSync()

	virtualNodeNameOpt := optTypes.NewNetworkingOption(optTypes.VirtualNodeName, optTypes.NetworkingValue(cfg.NodeName))
	grpcServerNameOpt := optTypes.NewNetworkingOption(optTypes.LiqoIpamServer, optTypes.NetworkingValue(cfg.LiqoIpamServer))
	remoteClusterIDOpt := optTypes.NewNetworkingOption(optTypes.RemoteClusterID, optTypes.NetworkingValue(cfg.RemoteClusterID))

	forge.InitForger(mapper, virtualNodeNameOpt, grpcServerNameOpt, remoteClusterIDOpt)

	opts := forgeOptionsMap(
		virtualNodeNameOpt,
		grpcServerNameOpt)

	return &LiqoProvider{
		homeClient:           homeClient,
		foreignClient:        foreignClient,
		foreignMetricsClient: foreignMetricsClient,
		foreignRestConfig:    remoteRestConfig,

		namespaceMapper: mapper,
		apiController:   controller.NewAPIController(homeClient, foreignClient, cfg.InformerResyncPeriod, mapper, opts),

		nodeName:  cfg.NodeName,
		startTime: time.Now(),
	}, nil
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

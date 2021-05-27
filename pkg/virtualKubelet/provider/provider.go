package provider

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	nattingv1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterid"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantcontrolnamespace "github.com/liqotech/liqo/pkg/tenantControlNamespace"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/controller"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	optTypes "github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
)

// LiqoProvider implements the virtual-kubelet provider interface and stores pods in memory.
type LiqoProvider struct {
	namespaceMapper namespacesMapping.MapperController
	apiController   controller.APIController

	tepReady             chan struct{}
	nntClient            *crdclient.CRDClient
	foreignClient        kubernetes.Interface
	foreignMetricsClient metricsv.Interface

	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	startTime          time.Time
	homeClusterID      string
	foreignClusterID   string
	restConfig         *rest.Config

	nodeName options.Option

	useNewAuth bool

	foreignPodWatcherStop chan struct{}
}

// NewLiqoProvider creates a new NewLiqoProvider instance.
func NewLiqoProvider(nodeName, foreignClusterID, homeClusterID, internalIP string, daemonEndpointPort int32, kubeconfig,
	remoteKubeConfig string, informerResyncPeriod time.Duration, ipamGRPCServer string, useNewAuth bool) (*LiqoProvider, error) {
	var err error

	if err = nattingv1.AddToScheme(clientgoscheme.Scheme); err != nil {
		return nil, err
	}

	client, err := nattingv1.CreateClient(kubeconfig, func(config *rest.Config) {
		config.QPS = virtualKubelet.HOME_CLIENT_QPS
		config.Burst = virtualKubelet.HOME_CLIENTS_BURST
	})
	if err != nil {
		return nil, err
	}

	clusterID := clusterid.NewStaticClusterID(homeClusterID)
	tenantNamespaceManager := tenantcontrolnamespace.NewTenantControlNamespaceManager(client.Client())
	identityManager := identitymanager.NewCertificateIdentityManager(client.Client(), clusterID, tenantNamespaceManager)

	var restConfig *rest.Config
	if useNewAuth {
		restConfig, err = identityManager.GetConfig(foreignClusterID, "")
		if err != nil {
			klog.Error(err)
			return nil, err
		}

		restConfig.QPS = virtualKubelet.FOREIGN_CLIENT_QPS
		restConfig.Burst = virtualKubelet.FOREIGN_CLIENT_BURST
	} else {
		restConfig, err = crdclient.NewKubeconfig(remoteKubeConfig, &schema.GroupVersion{}, func(config *rest.Config) {
			config.QPS = virtualKubelet.FOREIGN_CLIENT_QPS
			config.Burst = virtualKubelet.FOREIGN_CLIENT_BURST
		})
		if err != nil {
			klog.Error(err)
			return nil, err
		}
	}

	foreignClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	foreignMetricsClient, err := metricsv.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	mapper, err := namespacesMapping.NewNamespaceMapperController(client, foreignClient, homeClusterID, foreignClusterID)
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
		apiController:         controller.NewAPIController(client.Client(), foreignClient, informerResyncPeriod, mapper, opts, tepReady),
		namespaceMapper:       mapper,
		nodeName:              virtualNodeNameOpt,
		internalIP:            internalIP,
		daemonEndpointPort:    daemonEndpointPort,
		startTime:             time.Now(),
		foreignClusterID:      foreignClusterID,
		homeClusterID:         homeClusterID,
		nntClient:             client,
		foreignPodWatcherStop: make(chan struct{}, 1),
		restConfig:            restConfig,
		foreignClient:         foreignClient,
		foreignMetricsClient:  foreignMetricsClient,
		tepReady:              tepReady,
		useNewAuth:            useNewAuth,
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

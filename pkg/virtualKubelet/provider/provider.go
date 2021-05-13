package provider

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	nattingv1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/controller"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/node/module"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	optTypes "github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
)

// LiqoProvider implements the virtual-kubelet provider interface and stores pods in memory.
type LiqoProvider struct { // nolint:golint]
	namespaceMapper namespacesMapping.MapperController
	apiController   controller.ApiController

	tepReady             chan struct{}
	advClient            *crdClient.CRDClient
	tunEndClient         *crdClient.CRDClient
	nntClient            *crdClient.CRDClient
	foreignClient        kubernetes.Interface
	foreignMetricsClient metricsv.Interface

	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	startTime          time.Time
	foreignClusterId   string
	homeClusterID      string
	nodeController     *module.NodeController
	providerKubeconfig string
	restConfig         *rest.Config

	nodeName              options.Option
	RemoteRemappedPodCidr options.Option
	LocalRemappedPodCidr  options.Option

	foreignPodWatcherStop chan struct{}
}

// NewKubernetesProviderKubernetes creates a new KubernetesV0Provider. Kubernetes legacy provider does not implement the new asynchronous podnotifier interface.
func NewLiqoProvider(nodeName, foreignClusterId, homeClusterId string, internalIP string, daemonEndpointPort int32, kubeconfig, remoteKubeConfig string, informerResyncPeriod time.Duration) (*LiqoProvider, error) {
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

	advClient, err := advtypes.CreateAdvertisementClient(kubeconfig, nil, true, nil)
	if err != nil {
		return nil, err
	}

	tepClient, err := nettypes.CreateTunnelEndpointClient(kubeconfig)
	if err != nil {
		return nil, err
	}

	restConfig, err := crdClient.NewKubeconfig(remoteKubeConfig, &schema.GroupVersion{}, func(config *rest.Config) {
		config.QPS = virtualKubelet.FOREIGN_CLIENT_QPS
		config.Burst = virtualKubelet.FOREIGN_CLIENT_BURST
	})
	if err != nil {
		return nil, err
	}

	foreignClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	foreignMetricsClient, err := metricsv.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	mapper, err := namespacesMapping.NewNamespaceMapperController(client, foreignClient, homeClusterId, foreignClusterId)
	if err != nil {
		klog.Fatal(err)
	}
	mapper.WaitForSync()

	remoteRemappedPodCIDROpt := optTypes.NewNetworkingOption(optTypes.RemoteRemappedPodCIDR, "")
	localRemappedPodCIDROpt := optTypes.NewNetworkingOption(optTypes.LocalRemappedPodCIDR, "")
	virtualNodeNameOpt := optTypes.NewNetworkingOption(optTypes.VirtualNodeName, optTypes.NetworkingValue(nodeName))

	forge.InitForger(mapper, remoteRemappedPodCIDROpt, localRemappedPodCIDROpt, virtualNodeNameOpt)

	opts := forgeOptionsMap(
		remoteRemappedPodCIDROpt,
		localRemappedPodCIDROpt,
		virtualNodeNameOpt)

	tepReady := make(chan struct{})

	provider := LiqoProvider{
		apiController:         controller.NewApiController(client.Client(), foreignClient, informerResyncPeriod, mapper, opts, tepReady),
		namespaceMapper:       mapper,
		nodeName:              virtualNodeNameOpt,
		internalIP:            internalIP,
		daemonEndpointPort:    daemonEndpointPort,
		startTime:             time.Now(),
		foreignClusterId:      foreignClusterId,
		homeClusterID:         homeClusterId,
		providerKubeconfig:    remoteKubeConfig,
		nntClient:             client,
		foreignPodWatcherStop: make(chan struct{}, 1),
		restConfig:            restConfig,
		foreignClient:         foreignClient,
		foreignMetricsClient:  foreignMetricsClient,
		advClient:             advClient,
		tunEndClient:          tepClient,
		RemoteRemappedPodCidr: remoteRemappedPodCIDROpt,
		LocalRemappedPodCidr:  localRemappedPodCIDROpt,
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

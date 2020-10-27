package provider

import (
	"errors"
	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	nattingv1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	"github.com/liqotech/liqo/internal/virtualKubelet/node"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/controller"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	optTypes "github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"time"
)

// KubernetesProvider implements the virtual-kubelet provider interface and stores pods in memory.
type KubernetesProvider struct { // nolint:golint]
	namespaceMapper *namespacesMapping.NamespaceMapperController
	apiController   *controller.Controller

	advClient     *crdClient.CRDClient
	tunEndClient  *crdClient.CRDClient
	foreignClient *crdClient.CRDClient
	homeClient    *crdClient.CRDClient

	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	startTime          time.Time
	notifier           func(interface{})
	foreignClusterId   string
	homeClusterID      string
	nodeController     *node.NodeController
	providerKubeconfig string
	restConfig         *rest.Config

	nodeName              options.Option
	RemoteRemappedPodCidr options.Option
	LocalRemappedPodCidr  options.Option

	foreignPodWatcherStop chan struct{}
	nodeUpdateStop        chan struct{}
	nodeReady             chan struct{}
}

// NewKubernetesProviderKubernetesConfig creates a new KubernetesV0Provider. Kubernetes legacy provider does not implement the new asynchronous podnotifier interface
func NewKubernetesProvider(nodeName, foreignClusterId, homeClusterId string, internalIP string, daemonEndpointPort int32, kubeconfig, remoteKubeConfig string) (*KubernetesProvider, error) {
	var err error

	if err = nattingv1.AddToScheme(clientgoscheme.Scheme); err != nil {
		return nil, err
	}

	client, err := nattingv1.CreateClient(kubeconfig)
	if err != nil {
		return nil, err
	}

	advClient, err := advtypes.CreateAdvertisementClient(kubeconfig, nil, true)
	if err != nil {
		return nil, err
	}

	tepClient, err := nettypes.CreateTunnelEndpointClient(kubeconfig)
	if err != nil {
		return nil, err
	}

	restConfig, err := crdClient.NewKubeconfig(remoteKubeConfig, &schema.GroupVersion{})
	if err != nil {
		return nil, err
	}

	foreignClient, err := crdClient.NewFromConfig(restConfig)
	if err != nil {
		return nil, err
	}

	mapper, err := namespacesMapping.NewNamespaceMapperController(client, foreignClient.Client(), homeClusterId, foreignClusterId)
	if err != nil {
		klog.Fatal(err)
	}
	mapper.WaitForSync()

	remoteRemappedPodCIDROpt := optTypes.NewNetworkingOption(optTypes.RemoteRemappedPodCIDR, "")
	localRemappedPodCIDROpt := optTypes.NewNetworkingOption(optTypes.LocalRemappedPodCIDR, "")
	nodeNameOpt := optTypes.NewNetworkingOption(optTypes.NodeName, optTypes.NetworkingValue(nodeName))

	opts := forgeOptionsMap(
		remoteRemappedPodCIDROpt,
		localRemappedPodCIDROpt,
		nodeNameOpt)

	provider := KubernetesProvider{
		apiController:         controller.NewApiController(client.Client(), foreignClient.Client(), mapper, opts),
		namespaceMapper:       mapper,
		nodeName:              nodeNameOpt,
		internalIP:            internalIP,
		daemonEndpointPort:    daemonEndpointPort,
		startTime:             time.Now(),
		foreignClusterId:      foreignClusterId,
		homeClusterID:         homeClusterId,
		providerKubeconfig:    remoteKubeConfig,
		homeClient:            client,
		foreignPodWatcherStop: make(chan struct{}, 1),
		restConfig:            restConfig,
		foreignClient:         foreignClient,
		advClient:             advClient,
		tunEndClient:          tepClient,

		RemoteRemappedPodCidr: remoteRemappedPodCIDROpt,
		LocalRemappedPodCidr:  localRemappedPodCIDROpt,
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

func (p *KubernetesProvider) GetNamespaceMapper() (*namespacesMapping.NamespaceMapperController, error) {
	if p.namespaceMapper == nil {
		return nil, errors.New("NamespaceMapper is nil")
	}
	return p.namespaceMapper, nil
}

func (p *KubernetesProvider) GetApiController() (*controller.Controller, error) {
	if p.apiController == nil {
		return nil, errors.New("ApiController is nil")
	}
	return p.apiController, nil
}

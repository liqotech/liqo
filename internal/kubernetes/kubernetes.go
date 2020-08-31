package kubernetes

import (
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	nattingv1 "github.com/liqoTech/liqo/api/virtualKubelet/v1alpha1"
	"github.com/liqoTech/liqo/internal/node"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"time"

	v1 "k8s.io/api/core/v1"
)

// KubernetesProvider implements the virtual-kubelet provider interface and stores pods in memory.
type KubernetesProvider struct { // nolint:golint]
	*Reflector

	ntCache            *namespaceNTCache
	foreignPodCaches   map[string]*podCache
	homeEpCaches       map[string]*epCache
	foreignEpCaches    map[string]*epCache
	nodeUpdateClient   *crdClient.CRDClient
	foreignClient      *crdClient.CRDClient
	homeClient         *crdClient.CRDClient
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	startTime          time.Time
	notifier           func(*v1.Pod)
	foreignClusterId   string
	homeClusterID      string
	initialized        bool
	nodeController     *node.NodeController
	providerKubeconfig string
	restConfig         *rest.Config
	RemappedPodCidr    string

	foreignPodWatcherStop chan struct{}
	nodeUpdateStop        chan struct{}
	nodeReady             chan struct{}
}

// NewKubernetesProviderKubernetesConfig creates a new KubernetesV0Provider. Kubernetes legacy provider does not implement the new asynchronous podnotifier interface
func NewKubernetesProvider(nodeName, clusterId, homeClusterId, operatingSystem string, internalIP string, daemonEndpointPort int32, kubeconfig, remoteKubeConfig string) (*KubernetesProvider, error) {
	var err error

	if err = nattingv1.AddToScheme(clientgoscheme.Scheme); err != nil {
		return nil, err
	}

	client, err := nattingv1.CreateClient(kubeconfig)
	if err != nil {
		return nil, err
	}

	advClient, err := advtypes.CreateAdvertisementClient(kubeconfig, nil)
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

	provider := KubernetesProvider{
		Reflector:             &Reflector{},
		ntCache:               &namespaceNTCache{nattingTableName: clusterId},
		foreignPodCaches:      make(map[string]*podCache),
		homeEpCaches:          make(map[string]*epCache),
		foreignEpCaches:       make(map[string]*epCache),
		nodeName:              nodeName,
		operatingSystem:       operatingSystem,
		internalIP:            internalIP,
		daemonEndpointPort:    daemonEndpointPort,
		startTime:             time.Now(),
		foreignClusterId:      clusterId,
		homeClusterID:         homeClusterId,
		providerKubeconfig:    remoteKubeConfig,
		homeClient:            client,
		foreignPodWatcherStop: make(chan struct{}, 1),
		restConfig:            restConfig,
		foreignClient:         foreignClient,
		nodeUpdateClient:      advClient,
	}

	return &provider, nil
}

func (p *KubernetesProvider) ConfigureReflection() error {
	if err := p.startNattingCache(p.homeClient); err != nil {
		return err
	}

	if err := p.createNattingTable(p.foreignClusterId); err != nil {
		return err
	}

	p.ntCache.WaitNamespaceNattingTableSync()

	return nil
}

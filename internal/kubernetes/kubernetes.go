package kubernetes

import (
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	nattingv1 "github.com/liqoTech/liqo/api/namespaceNattingTable/v1"
	"github.com/liqoTech/liqo/internal/node"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"time"

	v1 "k8s.io/api/core/v1"
)

// KubernetesProvider implements the virtual-kubelet provider interface and stores pods in memory.
type KubernetesProvider struct { // nolint:golint]
	*Reflector

	ntCache          *namespaceNTCache
	nodeUpdateClient *v1alpha1.CRDClient
	foreignClient    *v1alpha1.CRDClient
	homeClient       *v1alpha1.CRDClient

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

	advClient, err := protocolv1.CreateAdvertisementClient(kubeconfig, nil)
	if err != nil {
		return nil, err
	}

	restConfig, err := v1alpha1.NewKubeconfig(remoteKubeConfig, &schema.GroupVersion{})
	if err != nil {
		return nil, err
	}

	foreignClient, err := v1alpha1.NewFromConfig(restConfig)
	if err != nil {
		return nil, err
	}

	provider := KubernetesProvider{
		Reflector:             &Reflector{},
		ntCache:               &namespaceNTCache{nattingTableName: clusterId},
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

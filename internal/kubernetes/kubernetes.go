package kubernetes

import (
	"github.com/go-logr/logr"
	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	nattingv1 "github.com/netgroup-polito/dronev2/api/namespaceNattingTable/v1"
	"github.com/netgroup-polito/dronev2/internal/node"
	"github.com/netgroup-polito/dronev2/pkg/crdClient/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	manager2 "sigs.k8s.io/controller-runtime/pkg/manager"
	"time"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	namespaceKey       = "namespace"
	nameKey            = "name"
	defaultMetricsAddr = ":8080"
)

// KubernetesProvider implements the virtual-kubelet provider interface and stores pods in memory.
type KubernetesProvider struct { // nolint:golint]
	*Reflector

	ntCache       *namespaceNTCache
	manager       manager2.Manager
	foreignClient *v1alpha1.CRDClient
	homeClient    *v1alpha1.CRDClient

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

	foreignPodWatcherStop chan bool

	log logr.Logger
}

// NewKubernetesProviderKubernetesConfig creates a new KubernetesV0Provider. Kubernetes legacy provider does not implement the new asynchronous podnotifier interface
func NewKubernetesProvider(nodeName, clusterId, homeClusterId, operatingSystem string, internalIP string, daemonEndpointPort int32, kubeconfig, remoteKubeConfig string) (*KubernetesProvider, error) {
	var err error

	if err = nattingv1.AddToScheme(clientgoscheme.Scheme); err != nil {
		return nil, err
	}

	mgr, err := newControllerManager(kubeconfig)
	if err != nil {
		return nil, err
	}

	client, err := nattingv1.CreateClient(kubeconfig)
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
		manager:               mgr,
		foreignClusterId:      clusterId,
		homeClusterID:         homeClusterId,
		providerKubeconfig:    remoteKubeConfig,
		homeClient:            client,
		foreignPodWatcherStop: make(chan bool, 1),
		restConfig:            restConfig,
		foreignClient:         foreignClient,
	}

	return &provider, nil
}

func newControllerManager(configPath string) (manager2.Manager, error) {
	sc := runtime.NewScheme()
	_ = protocolv1.AddToScheme(sc)
	_ = clientgoscheme.AddToScheme(sc)

	kc, err := v1alpha1.NewKubeconfig(configPath, &protocolv1.GroupVersion)
	if err != nil {
		return nil, err
	}

	mgr, err := ctrl.NewManager(kc, ctrl.Options{
		Scheme:             sc,
		MetricsBindAddress: defaultMetricsAddr,
		LeaderElection:     false,
		Port:               9443,
	})
	if err != nil {
		return nil, err
	}

	return mgr, nil
}

func (p *KubernetesProvider) ConfigureReflection() error {
	p.startNattingCache(p.homeClient)

	if err := p.createNattingTable(p.foreignClusterId); err != nil {
		return err
	}

	p.ntCache.WaitNamespaceNattingTableSync()

	return nil
}

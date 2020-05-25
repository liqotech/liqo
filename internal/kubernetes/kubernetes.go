package kubernetes

import (
	"github.com/go-logr/logr"
	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	"github.com/netgroup-polito/dronev2/internal/node"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	manager2 "sigs.k8s.io/controller-runtime/pkg/manager"
	"time"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	namespaceKey     = "namespace"
	nameKey          = "name"
	defaultMetricsAddr = ":8080"
)

// KubernetesProvider implements the virtual-kubelet provider interface and stores pods in memory.
type KubernetesProvider struct { // nolint:golint]
	*Reflector

	manager            manager2.Manager
	foreignClient      *kubernetes.Clientset
	homeClient         *kubernetes.Clientset

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

	namespaceNatting map[string]string
	namespaceDeNatting map[string]string

	foreignPodWatcherStop chan bool

	log logr.Logger
}

// NewKubernetesProviderKubernetesConfig creates a new KubernetesV0Provider. Kubernetes legacy provider does not implement the new asynchronous podnotifier interface
func NewKubernetesProvider(nodeName, clusterId, homeClusterId, operatingSystem string, internalIP string, daemonEndpointPort int32, kubeconfig, remoteKubeConfig string) (*KubernetesProvider, error) {

	scheme := runtime.NewScheme()
	_ = protocolv1.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	kc, err := newKubeconfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	mgr, err := ctrl.NewManager(kc, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: defaultMetricsAddr,
		LeaderElection:     false,
		Port:               9443,
	})
	if err != nil {
		return nil, err
	}

	c, _, err := newClient(kubeconfig)
	if err != nil {
		return nil, err
	}

	provider := KubernetesProvider{
		Reflector:             &Reflector{},
		nodeName:              nodeName,
		operatingSystem:       operatingSystem,
		internalIP:            internalIP,
		daemonEndpointPort:    daemonEndpointPort,
		startTime:             time.Now(),
		manager:               mgr,
		foreignClusterId:      clusterId,
		homeClusterID:         homeClusterId,
		providerKubeconfig:    remoteKubeConfig,
		homeClient:            c,
		namespaceNatting:      map[string]string{},
		namespaceDeNatting:    map[string]string{},
		foreignPodWatcherStop: make(chan bool, 1),
	}

	return &provider, nil
}

func newKubeconfig(configPath string) (*rest.Config, error) {
	var config *rest.Config

	// Check if the kubeConfig file exists.
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, errors.Wrap(err, "error building client config")
		}
	} else {
		// Set to in-cluster config.
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, "error building in cluster config")
		}
	}

	return config, nil
}

func newClient(configPath string) (*kubernetes.Clientset, *rest.Config , error) {
	config, err := newKubeconfig(configPath)
	if err != nil {
		return nil, nil, err
	}

	if masterURI := os.Getenv("MASTER_URI"); masterURI != "" {
		config.Host = masterURI
	}

	client, err := kubernetes.NewForConfig(config)
	return  client, config, err
}



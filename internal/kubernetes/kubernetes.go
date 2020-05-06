package kubernetes

import (
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

var (
	defaultMetricsAddr = ":8080"
)

const (
	// Provider configuration defaults.
	defaultCPUCapacity    = "20"
	defaultMemoryCapacity = "100Gi"
	defaultPodCapacity    = "20"
	defaultNamespace      = "drone-v2"

	// Values used in tracing as attribute keys.
	namespaceKey     = "namespace"
	nameKey          = "name"
	containerNameKey = "containerName"
)

// See: https://github.com/virtual-kubelet/virtual-kubelet/issues/632
/*
var (
	_ providers.Provider           = (*KubernetesV0Provider)(nil)
	_ providers.PodMetricsProvider = (*KubernetesV0Provider)(nil)
	_ node.PodNotifier         = (*KubernetesProvider)(nil)
)
*/

// KubernetesProvider implements the virtual-kubelet provider interface and stores pods in memory.
type KubernetesProvider struct { // nolint:golint]
	manager manager2.Manager
	client             *kubernetes.Clientset
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	startTime          time.Time
	notifier           func(*v1.Pod)
	clusterId string
	initialized	bool
	nodeController *node.NodeController
	providerKubeconfig string
	restConfig		   *rest.Config
	providerNamespace string
	RemotePodCidr string
	config KubernetesConfig //TODO: To remove
}

// KubernetesConfig contains a kubernetes virtual-kubelet's configurable parameters.
type KubernetesConfig struct { //nolint:golint
	RemoteKubeConfigPath string `json:"remoteKubeconfig,omitempty"`
	CPU                  string `json:"cpu,omitempty"`
	Memory               string `json:"memory,omitempty"`
	Pods                 string `json:"pods,omitempty"`
	Namespace            string `json:"namespace,omitempty"`
	RemoteNewPodCidr     string `json:"remoteNewPodCidr,omitempty"`
}

// NewKubernetesProviderKubernetesConfig creates a new KubernetesV0Provider. Kubernetes legacy provider does not implement the new asynchronous podnotifier interface
func NewKubernetesProvider(nodeName, clusterId, operatingSystem string, internalIP string, daemonEndpointPort int32, kubeconfig, remoteKubeConfig string) (*KubernetesProvider, error) {

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

	provider := KubernetesProvider{
		nodeName:           nodeName,
		operatingSystem:    operatingSystem,
		internalIP:         internalIP,
		daemonEndpointPort: daemonEndpointPort,
		startTime:          time.Now(),
		manager:			mgr,
		clusterId: clusterId,
		providerKubeconfig: remoteKubeConfig,
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



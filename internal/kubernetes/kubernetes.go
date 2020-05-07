package kubernetes

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"regexp"
	"time"

	v1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	client             *kubernetes.Clientset
	restConfig		   *rest.Config
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	config             KubernetesConfig
	startTime          time.Time
	notifier           func(*v1.Pod)
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
func NewKubernetesProviderKubernetesConfig(config KubernetesConfig, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*KubernetesProvider, error) {
	//set defaults
	if config.CPU == "" {
		config.CPU = defaultCPUCapacity
	}
	if config.Memory == "" {
		config.Memory = defaultMemoryCapacity
	}
	if config.Pods == "" {
		config.Pods = defaultPodCapacity
	}
	if config.Namespace == "" {
		config.Namespace = defaultNamespace
	}
	if config.RemoteKubeConfigPath == "" {
		config.RemoteKubeConfigPath = os.Getenv("KUBECONFIG_REMOTE")
	}

	client, restConfig, err := newClient(config.RemoteKubeConfigPath)
	if err != nil {
		return nil, err
	}
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "drone-v2",
		},
	}
	// Create first namespace
	_, err = client.CoreV1().Namespaces().Create(ns)
	if err != nil && !kerror.IsAlreadyExists(err) {
		_ = fmt.Errorf("Failed tp create a namespace")
		return nil, err
	}

	provider := KubernetesProvider{
		client:             client,
		nodeName:           nodeName,
		operatingSystem:    operatingSystem,
		internalIP:         internalIP,
		daemonEndpointPort: daemonEndpointPort,
		config:             config,
		startTime:          time.Now(),
		restConfig:         restConfig,
	}
	provider.PodWatcher()

	return &provider, nil
}

func (p *KubernetesProvider) PodWatcher() error {
	watch, err := p.client.CoreV1().Pods(p.config.Namespace).Watch(metav1.ListOptions{})
	if err != nil {
		errors.Wrap(err, err.Error())
	}
	go func() {
		for event := range watch.ResultChan() {
			p2, ok := event.Object.(*v1.Pod)
			if !ok {
				_ = fmt.Errorf("unexpected type")
			}
			p.notifier(F2HTranslate(p2, p.config.RemoteNewPodCidr))
		}
	}()
	return nil
}

// NewKubernetesProvider creates a new KubernetesProvider, which implements the PodNotifier interface
func NewKubernetesProvider(providerConfig, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*KubernetesProvider, error) {
	config, err := loadConfig(providerConfig, nodeName)
	if err != nil {
		return nil, err
	}

	return NewKubernetesProviderKubernetesConfig(config, nodeName, operatingSystem, internalIP, daemonEndpointPort)
}

func newClient(configPath string) (*kubernetes.Clientset, *rest.Config , error) {
	var config *rest.Config

	// Check if the kubeConfig file exists.
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil,nil, errors.Wrap(err, "error building client config")
		}
	} else {
		// Set to in-cluster config.
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil,nil, errors.Wrap(err, "error building in cluster config")
		}
	}

	if masterURI := os.Getenv("MASTER_URI"); masterURI != "" {
		config.Host = masterURI
	}
	client, err := kubernetes.NewForConfig(config)
	return  client,config,err
}

// loadConfig loads the given json configuration files.
func loadConfig(providerConfig, nodeName string) (config KubernetesConfig, err error) {
	//regular expression which checks if the input CIDR has a correct format
	cidrRegEx := "^([0-9]{1,3}\\.){3}[0-9]{1,3}(\\/([0-9]|[1-2][0-9]|3[0-2]))$"
	data, err := ioutil.ReadFile(providerConfig)
	if err != nil {
		return config, err
	}
	configMap := map[string]KubernetesConfig{}
	err = json.Unmarshal(data, &configMap)
	if err != nil {
		return config, err
	}
	if _, exist := configMap[nodeName]; exist {
		config = configMap[nodeName]
		if config.CPU == "" {
			config.CPU = defaultCPUCapacity
		}
		if config.Memory == "" {
			config.Memory = defaultMemoryCapacity
		}
		if config.Pods == "" {
			config.Pods = defaultPodCapacity
		}
		if config.Namespace == "" {
			config.Namespace = defaultNamespace
		}

		if config.RemoteKubeConfigPath == "" {
			config.RemoteKubeConfigPath = os.Getenv("KUBECONFIG_REMOTE")
		}
		if config.RemoteNewPodCidr == "" {
			return config, fmt.Errorf("RemoteNewPodCidr is not defined #{config.RemoteNewPodCidr")
		}
	}

	if _, err = resource.ParseQuantity(config.CPU); err != nil {
		return config, fmt.Errorf("Invalid CPU value %v", config.CPU)
	}
	if _, err = resource.ParseQuantity(config.Memory); err != nil {
		return config, fmt.Errorf("Invalid memory value %v", config.Memory)
	}
	if _, err = resource.ParseQuantity(config.Pods); err != nil {
		return config, fmt.Errorf("Invalid pods value %v", config.Pods)
	}
	if !fileExists(config.RemoteKubeConfigPath) {
		return config, fmt.Errorf("Remote Kubeconfig file not found %v", config.RemoteKubeConfigPath)
	}
	if matched, _ := regexp.MatchString(cidrRegEx, config.RemoteNewPodCidr); matched != true {
		return config, fmt.Errorf("Wrong format of RemoteNewPodCidr #{config.RemoteNewPodCidr}")
	}

	return config, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

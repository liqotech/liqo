package authservice

import (
	"reflect"

	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils"
)

// GetAuthServiceConfig starts the watcher to ClusterConfing CR.
func (authService *Controller) GetAuthServiceConfig(kubeconfigPath string) {
	waitFirst := make(chan struct{})
	isFirst := true
	go utils.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		authService.handleDiscoveryConfiguration(&configuration.Spec.DiscoveryConfig)
		authService.handleAPIServerConfiguration(&configuration.Spec.APIServerConfig)
		if isFirst {
			isFirst = false
			close(waitFirst)
		}
	}, nil, kubeconfigPath)
	<-waitFirst
}

// GetAPIServerConfig returns the configuration of the local APIServer (address, port).
func (authService *Controller) GetAPIServerConfig() *configv1alpha1.APIServerConfig {
	authService.configMutex.RLock()
	defer authService.configMutex.RUnlock()
	return authService.apiServerConfig.DeepCopy()
}

func (authService *Controller) handleDiscoveryConfiguration(config *configv1alpha1.DiscoveryConfig) {
	authService.configMutex.Lock()
	defer authService.configMutex.Unlock()
	authService.discoveryConfig = *config.DeepCopy()
}

func (authService *Controller) handleAPIServerConfiguration(config *configv1alpha1.APIServerConfig) {
	authService.configMutex.Lock()
	defer authService.configMutex.Unlock()

	if reflect.DeepEqual(&config, authService.apiServerConfig) {
		klog.V(6).Info("New and old apiServer configs are deep equals")
		klog.V(8).Infof("Old config: %v\nNew config: %v", authService.apiServerConfig, config)
		return
	}

	authService.apiServerConfig = config.DeepCopy()
}

func (authService *Controller) getDiscoveryConfig() configv1alpha1.DiscoveryConfig {
	authService.configMutex.RLock()
	defer authService.configMutex.RUnlock()
	return authService.discoveryConfig
}

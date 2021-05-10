package auth_service

import (
	"reflect"

	"k8s.io/klog"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterConfig"
)

// GetAuthServiceConfig starts the watcher to ClusterConfing CR
func (authService *AuthServiceCtrl) GetAuthServiceConfig(kubeconfigPath string) {
	waitFirst := make(chan struct{})
	isFirst := true
	go clusterConfig.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		authService.handleConfiguration(&configuration.Spec.AuthConfig)
		authService.handleDiscoveryConfiguration(&configuration.Spec.DiscoveryConfig)
		authService.handleAPIServerConfiguration(&configuration.Spec.ApiServerConfig)
		if isFirst {
			isFirst = false
			close(waitFirst)
		}
	}, nil, kubeconfigPath)
	<-waitFirst
}

func (authService *AuthServiceCtrl) handleConfiguration(config *configv1alpha1.AuthConfig) {
	authService.configMutex.Lock()
	defer authService.configMutex.Unlock()
	authService.config = config.DeepCopy()
}

// GetConfig returns the configuration of the local Authentication service
func (authService *AuthServiceCtrl) GetConfig() *configv1alpha1.AuthConfig {
	authService.configMutex.RLock()
	defer authService.configMutex.RUnlock()
	return authService.config.DeepCopy()
}

// GetAPIServerConfig returns the configuration of the local APIServer (address, port)
func (authService *AuthServiceCtrl) GetAPIServerConfig() *configv1alpha1.ApiServerConfig {
	authService.configMutex.RLock()
	defer authService.configMutex.RUnlock()
	return authService.apiServerConfig.DeepCopy()
}

func (authService *AuthServiceCtrl) handleDiscoveryConfiguration(config *configv1alpha1.DiscoveryConfig) {
	authService.configMutex.Lock()
	defer authService.configMutex.Unlock()
	authService.discoveryConfig = *config.DeepCopy()
}

func (authService *AuthServiceCtrl) handleAPIServerConfiguration(config *configv1alpha1.ApiServerConfig) {
	authService.configMutex.Lock()
	defer authService.configMutex.Unlock()

	if reflect.DeepEqual(&config, authService.apiServerConfig) {
		klog.V(6).Info("New and old apiServer configs are deep equals")
		klog.V(8).Infof("Old config: %v\nNew config: %v", authService.apiServerConfig, config)
		return
	}

	authService.apiServerConfig = config.DeepCopy()
}

func (authService *AuthServiceCtrl) getDiscoveryConfig() configv1alpha1.DiscoveryConfig {
	authService.configMutex.RLock()
	defer authService.configMutex.RUnlock()
	return authService.discoveryConfig
}

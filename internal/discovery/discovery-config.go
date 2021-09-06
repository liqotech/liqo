package discovery

import (
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/utils"
)

// ConfigProvider interface provides methods to access the Discovery and API Server configuration.
type ConfigProvider interface {
	GetConfig() *configv1alpha1.DiscoveryConfig
}

// GetConfig returns the configuration of the discovery component.
func (discovery *Controller) GetConfig() *configv1alpha1.DiscoveryConfig {
	discovery.configMutex.RLock()
	defer discovery.configMutex.RUnlock()
	return discovery.Config
}

func (discovery *Controller) getDiscoveryConfig(client *crdclient.CRDClient, kubeconfigPath string) error {
	waitFirst := make(chan bool)
	isFirst := true
	go utils.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		discovery.handleConfiguration(&configuration.Spec.DiscoveryConfig)
		if isFirst {
			waitFirst <- true
			isFirst = false
		}
	}, client, kubeconfigPath)
	<-waitFirst
	close(waitFirst)

	return nil
}

func (discovery *Controller) handleConfiguration(config *configv1alpha1.DiscoveryConfig) {
	discovery.configMutex.Lock()
	defer discovery.configMutex.Unlock()

	reloadServer := false
	reloadClient := false
	if discovery.Config == nil {
		// first iteration
		discovery.Config = config.DeepCopy()
	} else {
		// other iterations
		if discovery.Config.ClusterName != config.ClusterName {
			discovery.Config.ClusterName = config.ClusterName
			reloadClient = true
			reloadServer = true
		}
		if discovery.Config.Domain != config.Domain {
			discovery.Config.Domain = config.Domain
			reloadServer = true
			reloadClient = true
		}
		if discovery.Config.EnableAdvertisement != config.EnableAdvertisement {
			discovery.Config.EnableAdvertisement = config.EnableAdvertisement
			reloadServer = true
		}
		if discovery.Config.Name != config.Name {
			discovery.Config.Name = config.Name
			reloadServer = true
		}
		if discovery.Config.Port != config.Port {
			discovery.Config.Port = config.Port
			reloadServer = true
		}
		if discovery.Config.AuthService != config.AuthService {
			discovery.Config.AuthService = config.AuthService
			reloadServer = true
			reloadClient = true
		}
		if discovery.Config.Service != config.Service {
			discovery.Config.Service = config.Service
			reloadServer = true
			reloadClient = true
		}
		if discovery.Config.TTL != config.TTL {
			discovery.Config.TTL = config.TTL
			reloadServer = true
		}
		if discovery.Config.AutoJoin != config.AutoJoin {
			discovery.Config.AutoJoin = config.AutoJoin
			reloadClient = true
		}
		if discovery.Config.EnableDiscovery != config.EnableDiscovery {
			discovery.Config.EnableDiscovery = config.EnableDiscovery
			reloadClient = true
		}
		if reloadServer {
			discovery.reloadServer()
		}
		if reloadClient {
			discovery.reloadClient()
		}
	}
}

func (discovery *Controller) reloadServer() {
	klog.Info("Reload mDNS server")
	discovery.stopMDNS <- true
	if discovery.Config.EnableAdvertisement {
		close(discovery.stopMDNS)
		discovery.stopMDNS = make(chan bool, 1)
		go discovery.register()
	}
}

func (discovery *Controller) reloadClient() {
	klog.Info("Reload mDNS client")
	discovery.stopMDNSClient <- true
	if discovery.Config.EnableDiscovery {
		close(discovery.stopMDNSClient)
		discovery.stopMDNSClient = make(chan bool, 1)
		go discovery.startResolver(discovery.stopMDNSClient)
	}
}

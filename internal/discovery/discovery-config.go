package discovery

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	"github.com/liqoTech/liqo/pkg/clusterConfig"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	"os"
	"path/filepath"
)

func (discovery *DiscoveryCtrl) GetDiscoveryConfig(crdClient *v1alpha1.CRDClient) error {
	waitFirst := make(chan bool)
	isFirst := true
	go clusterConfig.WatchConfiguration(func(configuration *policyv1.ClusterConfig) {
		discovery.Log.Info("Change Configuration")
		discovery.handleConfiguration(configuration.Spec.DiscoveryConfig)
		if isFirst {
			waitFirst <- true
			isFirst = false
		}
	}, crdClient, filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	<-waitFirst
	close(waitFirst)

	return nil
}

func (discovery *DiscoveryCtrl) handleConfiguration(config policyv1.DiscoveryConfig) {
	reloadServer := false
	reloadClient := false
	if discovery.Config == nil {
		// first iteration
		discovery.Config = &config
	} else {
		// other iterations
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
		if discovery.Config.Service != config.Service {
			discovery.Config.Service = config.Service
			reloadServer = true
			reloadClient = true
		}
		if discovery.Config.UpdateTime != config.UpdateTime {
			discovery.Config.UpdateTime = config.UpdateTime
			reloadClient = true
		}
		if discovery.Config.WaitTime != config.WaitTime {
			discovery.Config.WaitTime = config.WaitTime
			reloadClient = true
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

func (discovery *DiscoveryCtrl) reloadServer() {
	discovery.Log.Info("Reload mDNS server")
	select {
	case discovery.stopMDNS <- true:
		close(discovery.stopMDNS)
	default:
	}
	if discovery.Config.EnableAdvertisement {
		go discovery.Register()
	}
}

func (discovery *DiscoveryCtrl) reloadClient() {
	discovery.Log.Info("Reload mDNS client")
	// settings are automatically updated in next iteration
}

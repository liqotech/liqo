package discovery

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	"github.com/liqoTech/liqo/pkg/clusterConfig"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"k8s.io/klog"
)

func (discovery *DiscoveryCtrl) GetDiscoveryConfig(crdClient *crdClient.CRDClient, kubeconfigPath string) error {
	waitFirst := make(chan bool)
	isFirst := true
	go clusterConfig.WatchConfiguration(func(configuration *policyv1.ClusterConfig) {
		klog.Info("Change Configuration")
		discovery.handleConfiguration(configuration.Spec.DiscoveryConfig)
		if isFirst {
			waitFirst <- true
			isFirst = false
		}
	}, crdClient, kubeconfigPath)
	<-waitFirst
	close(waitFirst)

	return nil
}

func (discovery *DiscoveryCtrl) handleConfiguration(config policyv1.DiscoveryConfig) {
	reloadServer := false
	reloadClient := false
	reloadCa := false
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
		if discovery.Config.AllowUntrustedCA != config.AllowUntrustedCA {
			discovery.Config.AllowUntrustedCA = config.AllowUntrustedCA
			reloadCa = true
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
		if discovery.Config.AutoJoinUntrusted != config.AutoJoinUntrusted {
			discovery.Config.AutoJoinUntrusted = config.AutoJoinUntrusted
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
		if reloadCa {
			err := discovery.SetupCaData()
			if err != nil {
				klog.Error(err, err.Error())
			}
		}
	}
}

func (discovery *DiscoveryCtrl) reloadServer() {
	klog.Info("Reload mDNS server")
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
	klog.Info("Reload mDNS client")
	// settings are automatically updated in next iteration
}

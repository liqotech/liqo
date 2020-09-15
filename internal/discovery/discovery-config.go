package discovery

import (
	"context"
	configv1alpha1 "github.com/liqotech/liqo/api/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterConfig"
	"github.com/liqotech/liqo/pkg/crdClient"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func (discovery *DiscoveryCtrl) GetDiscoveryConfig(crdClient *crdClient.CRDClient, kubeconfigPath string) error {
	waitFirst := make(chan bool)
	isFirst := true
	go clusterConfig.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		klog.Info("Change Configuration")
		discovery.handleConfiguration(configuration.Spec.DiscoveryConfig)
		discovery.handleDispatcherConfig(configuration.Spec.DispatcherConfig)
		if isFirst {
			waitFirst <- true
			isFirst = false
		}
	}, crdClient, kubeconfigPath)
	<-waitFirst
	close(waitFirst)

	return nil
}

func (discovery *DiscoveryCtrl) handleDispatcherConfig(config configv1alpha1.DispatcherConfig) {
	role, err := discovery.crdClient.Client().RbacV1().ClusterRoles().Get(context.TODO(), "crdReplicator-role", metav1.GetOptions{})
	create := false
	if errors.IsNotFound(err) {
		// create it
		role = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "crdReplicator-role",
			},
			Rules: []rbacv1.PolicyRule{},
		}
		create = true
	} else if err != nil {
		// other errors
		klog.Error(err)
		return
	}

	// create rules array
	rules := []rbacv1.PolicyRule{}
	for _, res := range config.ResourcesToReplicate {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:     []string{"*"},
			APIGroups: []string{res.Group},
			Resources: []string{res.Resource, res.Resource + "/status"},
		})
	}
	role.Rules = rules

	if create {
		_, err = discovery.crdClient.Client().RbacV1().ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
	} else {
		_, err = discovery.crdClient.Client().RbacV1().ClusterRoles().Update(context.TODO(), role, metav1.UpdateOptions{})
	}
	if err != nil {
		klog.Error(err)
		return
	}
}

func (discovery *DiscoveryCtrl) handleConfiguration(config configv1alpha1.DiscoveryConfig) {
	reloadServer := false
	reloadClient := false
	if discovery.Config == nil {
		// first iteration
		discovery.Config = &config
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
		if discovery.Config.AllowUntrustedCA != config.AllowUntrustedCA {
			discovery.Config.AllowUntrustedCA = config.AllowUntrustedCA
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

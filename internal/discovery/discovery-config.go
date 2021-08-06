package discovery

import (
	"context"
	"reflect"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/utils"
)

// ConfigProvider interface provides methods to access the Discovery and API Server configuration.
type ConfigProvider interface {
	GetConfig() *configv1alpha1.DiscoveryConfig
	GetAPIServerConfig() *configv1alpha1.APIServerConfig
}

// GetConfig returns the configuration of the discovery component.
func (discovery *Controller) GetConfig() *configv1alpha1.DiscoveryConfig {
	discovery.configMutex.RLock()
	defer discovery.configMutex.RUnlock()
	return discovery.Config
}

// GetAuthConfig returns the authentication configuration.
func (discovery *Controller) GetAuthConfig() *configv1alpha1.AuthConfig {
	discovery.configMutex.RLock()
	defer discovery.configMutex.RUnlock()
	return discovery.authConfig
}

// GetAPIServerConfig returns the configuration of the local APIServer (address, port).
func (discovery *Controller) GetAPIServerConfig() *configv1alpha1.APIServerConfig {
	discovery.configMutex.RLock()
	defer discovery.configMutex.RUnlock()
	return discovery.apiServerConfig
}

func (discovery *Controller) getDiscoveryConfig(client *crdclient.CRDClient, kubeconfigPath string) error {
	waitFirst := make(chan bool)
	isFirst := true
	go utils.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		discovery.handleConfiguration(&configuration.Spec.DiscoveryConfig)
		discovery.handleDispatcherConfig(&configuration.Spec.DispatcherConfig)
		discovery.handleAPIServerConfig(&configuration.Spec.APIServerConfig)
		discovery.handleAuthConfig(&configuration.Spec.AuthConfig)
		if isFirst {
			waitFirst <- true
			isFirst = false
		}
	}, client, kubeconfigPath)
	<-waitFirst
	close(waitFirst)

	return nil
}

func (discovery *Controller) handleAuthConfig(config *configv1alpha1.AuthConfig) {
	discovery.configMutex.Lock()
	defer discovery.configMutex.Unlock()

	if reflect.DeepEqual(&config, discovery.authConfig) {
		klog.V(6).Info("New and old auth configs are deep equals")
		klog.V(8).Infof("Old config: %v\nNew config: %v", discovery.authConfig, config)
		return
	}

	discovery.authConfig = config.DeepCopy()
}

func (discovery *Controller) handleAPIServerConfig(config *configv1alpha1.APIServerConfig) {
	discovery.configMutex.Lock()
	defer discovery.configMutex.Unlock()

	if reflect.DeepEqual(config, discovery.apiServerConfig) {
		klog.V(6).Info("New and old apiServer configs are deep equals")
		klog.V(8).Infof("Old config: %v\nNew config: %v", discovery.apiServerConfig, config)
		return
	}

	discovery.apiServerConfig = config.DeepCopy()
}

func (discovery *Controller) handleDispatcherConfig(config *configv1alpha1.DispatcherConfig) {
	discovery.configMutex.Lock()
	defer discovery.configMutex.Unlock()

	if reflect.DeepEqual(config, discovery.crdReplicatorConfig) {
		klog.V(6).Info("New and old crdReplicator configs are deep equals")
		klog.V(8).Infof("Old config: %v\nNew config: %v", discovery.crdReplicatorConfig, config)
		return
	}

	role, err := discovery.crdClient.Client().RbacV1().ClusterRoles().Get(context.TODO(), "crdreplicator-role",
		metav1.GetOptions{})
	create := false
	if errors.IsNotFound(err) {
		// create it
		role = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "crdreplicator-role",
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
			Verbs:     []string{"get", "update", "patch", "list", "watch", "delete", "create", "deletecollection"},
			APIGroups: []string{res.GroupVersionResource.Group},
			Resources: []string{res.GroupVersionResource.Resource, res.GroupVersionResource.Resource + "/status"},
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

	discovery.crdReplicatorConfig = config.DeepCopy()
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

package utils

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
)

type ApiServerConfigProvider interface {
	GetAPIServerConfig() *configv1alpha1.APIServerConfig
}

// WatchConfiguration watches the ClusterConfig CR, and calls the handler funcition on updates.
func WatchConfiguration(handler func(*configv1alpha1.ClusterConfig), client *crdclient.CRDClient, kubeconfigPath string) {
	var rsyncPeriod = 30 * time.Second
	if client == nil {
		config, err := crdclient.NewKubeconfig(kubeconfigPath, &configv1alpha1.GroupVersion, nil)
		if err != nil {
			klog.Error(err, err.Error())
			os.Exit(1)
		}

		client, err = crdclient.NewFromConfig(config)
		if err != nil {
			klog.Error(err, err.Error())
			os.Exit(1)
		}
	}

	var err error

	ehf := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			configuration, ok := obj.(*configv1alpha1.ClusterConfig)
			if !ok {
				klog.Info("Error casting clusterConfig while handling creation")
				os.Exit(1)
			}
			handler(configuration)
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			configuration, ok := newObj.(*configv1alpha1.ClusterConfig)
			if !ok {
				klog.Info("Error casting clusterConfig while handling an update")
				os.Exit(1)
			}
			handler(configuration)
		},
		DeleteFunc: func(config interface{}) {
			klog.Error("please, do not delete ClusterConfigs")
			configuration := config.(*configv1alpha1.ClusterConfig)
			configuration.ResourceVersion = ""
			_, err = client.Resource("clusterconfigs").Create(configuration, &metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				klog.Error(err, err.Error())
			}
		},
	}
	lo := metav1.ListOptions{}
	client.Store, client.Stop, err = crdclient.WatchResources(client,
		"clusterconfigs", "",
		rsyncPeriod, ehf, lo)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	klog.Info("Cluster Config Informer initialized")
}

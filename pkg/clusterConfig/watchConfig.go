package clusterConfig

import (
	configv1alpha1 "github.com/liqotech/liqo/api/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	"os"
)

func WatchConfiguration(handler func(*configv1alpha1.ClusterConfig), client *crdClient.CRDClient, kubeconfigPath string) {
	if client == nil {
		config, err := crdClient.NewKubeconfig(kubeconfigPath, &configv1alpha1.GroupVersion)
		if err != nil {
			klog.Error(err, err.Error())
			os.Exit(1)
		}

		client, err = crdClient.NewFromConfig(config)
		if err != nil {
			klog.Error(err, err.Error())
			os.Exit(1)
		}
	}

	watcher, err := client.Resource("clusterconfigs").Watch(metav1.ListOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	for event := range watcher.ResultChan() {
		configuration, ok := event.Object.(*configv1alpha1.ClusterConfig)
		if !ok {
			klog.Error("Received object is not a ClusterConfig")
			continue
		}
		klog.V(3).Info("ClusterConfig changed")
		switch event.Type {
		case watch.Added, watch.Modified:
			handler(configuration)
		case watch.Deleted:
			klog.Error("please, do not delete ClusterConfigs")
			configuration.ResourceVersion = ""
			_, err = client.Resource("clusterconfigs").Create(configuration, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				klog.Error(err, err.Error())
				continue
			}
		}
	}
}

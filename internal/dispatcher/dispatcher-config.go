package dispatcher

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	"github.com/liqoTech/liqo/pkg/clusterConfig"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"os"
	"reflect"
)

func (d *DispatcherReconciler) WatchConfiguration(config *rest.Config, gv *schema.GroupVersion) {
	config.ContentConfig.GroupVersion = gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	CRDclient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	go clusterConfig.WatchConfiguration(d.UpdateConfig, CRDclient, "")
}

func (d *DispatcherReconciler) UpdateConfig(cfg *policyv1.ClusterConfig) {
	resources := d.GetConfig(cfg)
	if !reflect.DeepEqual(d.RegisteredResources, resources) {
		klog.Info("updating the list of registered resources to be replicated")
		d.RegisteredResources = resources
	}
}

func (d *DispatcherReconciler) GetConfig(cfg *policyv1.ClusterConfig) []schema.GroupVersionResource {
	resourceList := cfg.Spec.DispatcherConfig
	klog.Info(resourceList)
	config := []schema.GroupVersionResource{}
	for _, res := range resourceList.ResourcesToReplicate {
		config = append(config, schema.GroupVersionResource{
			Group:    res.Group,
			Version:  res.Version,
			Resource: res.Resource,
		})
	}
	return config
}

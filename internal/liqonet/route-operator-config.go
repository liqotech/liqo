package controllers

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	"github.com/liqoTech/liqo/pkg/clusterConfig"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"os"
)

func (r *RouteController) WatchConfiguration(config *rest.Config, gv *schema.GroupVersion) {
	config.ContentConfig.GroupVersion = gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	CRDclient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	go clusterConfig.WatchConfiguration(func(configuration *policyv1.ClusterConfig) {
		if !r.IsConfigured {
			r.ClusterPodCIDR = configuration.Spec.LiqonetConfig.PodCIDR
			r.Configured <- true
		}
		//check if the podCIDR is different from the one on the cluster config
		//TODO: a go routine which removes all the configuration with the old podCIDR and triggers a new configuration for the new podCIDR
		if r.ClusterPodCIDR != configuration.Spec.LiqonetConfig.PodCIDR {
			r.ClusterPodCIDR = configuration.Spec.LiqonetConfig.PodCIDR
		}
	}, CRDclient, "")
}

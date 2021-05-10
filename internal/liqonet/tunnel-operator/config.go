package tunnel_operator

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterConfig"
	"github.com/liqotech/liqo/pkg/crdClient"
)

func (tc *TunnelController) WatchConfiguration(config *rest.Config, gv *schema.GroupVersion) {
	config.ContentConfig.GroupVersion = gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	CRDclient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	go clusterConfig.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		//this section is executed at start-up time
		if !tc.isConfigured {
			tc.isGKE = configuration.Spec.LiqonetConfig.GKEProvider
			tc.configChan <- true
			tc.isConfigured = true
			if tc.isGKE {
				klog.Info("starting gateway for the GKE cluster with kubenet CNI")
			}
		}
	}, CRDclient, "")
}

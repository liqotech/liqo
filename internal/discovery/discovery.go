package discovery

import (
	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterID"
	"github.com/liqotech/liqo/pkg/crdClient"
	"k8s.io/klog"
	"os"
)

type DiscoveryCtrl struct {
	Namespace string

	Config    *configv1alpha1.DiscoveryConfig
	stopMDNS  chan bool
	crdClient *crdClient.CRDClient
	advClient *crdClient.CRDClient
	ClusterId *clusterID.ClusterID
}

func NewDiscoveryCtrl(namespace string, clusterId *clusterID.ClusterID, kubeconfigPath string) (*DiscoveryCtrl, error) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := crdClient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	advClient, err := advtypes.CreateAdvertisementClient(kubeconfigPath, nil)
	if err != nil {
		klog.Error(err, "unable to create local client for Advertisement")
		os.Exit(1)
	}

	discoveryCtrl := GetDiscoveryCtrl(
		namespace,
		discoveryClient,
		advClient,
		clusterId,
	)
	if discoveryCtrl.GetDiscoveryConfig(nil, kubeconfigPath) != nil {
		os.Exit(1)
	}
	return &discoveryCtrl, nil
}

func GetDiscoveryCtrl(namespace string, crdClient *crdClient.CRDClient, advClient *crdClient.CRDClient, clusterId *clusterID.ClusterID) DiscoveryCtrl {
	return DiscoveryCtrl{
		Namespace: namespace,
		crdClient: crdClient,
		advClient: advClient,
		ClusterId: clusterId,
	}
}

// Start register and resolver goroutines
func (discovery *DiscoveryCtrl) StartDiscovery() {
	go discovery.Register()
	go discovery.StartResolver()
}

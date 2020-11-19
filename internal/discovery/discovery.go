package discovery

import (
	"github.com/grandcat/zeroconf"
	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterID"
	"github.com/liqotech/liqo/pkg/crdClient"
	"k8s.io/klog"
	"os"
	"sync"
	"time"
)

type DiscoveryCtrl struct {
	Namespace string

	Config         *configv1alpha1.DiscoveryConfig
	stopMDNS       chan bool
	stopMDNSClient chan bool
	crdClient      *crdClient.CRDClient
	advClient      *crdClient.CRDClient
	ClusterId      *clusterID.ClusterID

	mdnsServer                *zeroconf.Server
	mdnsServerAuth            *zeroconf.Server
	serverMux                 sync.Mutex
	resolveContextRefreshTime int

	dialTcpTimeout time.Duration
}

func NewDiscoveryCtrl(namespace string, clusterId *clusterID.ClusterID, kubeconfigPath string, resolveContextRefreshTime int, dialTcpTimeout time.Duration) (*DiscoveryCtrl, error) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := crdClient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	advClient, err := advtypes.CreateAdvertisementClient(kubeconfigPath, nil, true)
	if err != nil {
		klog.Error(err, "unable to create local client for Advertisement")
		os.Exit(1)
	}

	discoveryCtrl := GetDiscoveryCtrl(
		namespace,
		discoveryClient,
		advClient,
		clusterId,
		resolveContextRefreshTime,
		dialTcpTimeout,
	)
	if discoveryCtrl.GetDiscoveryConfig(nil, kubeconfigPath) != nil {
		os.Exit(1)
	}
	return &discoveryCtrl, nil
}

func GetDiscoveryCtrl(namespace string, crdClient *crdClient.CRDClient, advClient *crdClient.CRDClient, clusterId *clusterID.ClusterID, resolveContextRefreshTime int, dialTcpTimeout time.Duration) DiscoveryCtrl {
	return DiscoveryCtrl{
		Namespace:                 namespace,
		crdClient:                 crdClient,
		advClient:                 advClient,
		ClusterId:                 clusterId,
		stopMDNS:                  make(chan bool, 1),
		stopMDNSClient:            make(chan bool, 1),
		resolveContextRefreshTime: resolveContextRefreshTime,
		dialTcpTimeout:            dialTcpTimeout,
	}
}

// Start register and resolver goroutines
func (discovery *DiscoveryCtrl) StartDiscovery() {
	go discovery.Register()
	go discovery.StartResolver(discovery.stopMDNSClient)
	go discovery.StartGratuitousAnswers()
	go discovery.StartGarbageCollector()
}

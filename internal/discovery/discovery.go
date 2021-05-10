package discovery

import (
	"os"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"k8s.io/klog"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterid"
	"github.com/liqotech/liqo/pkg/crdClient"
)

// Controller is the controller for the discovery functionalities.
type Controller struct {
	Namespace string

	configMutex         sync.RWMutex
	Config              *configv1alpha1.DiscoveryConfig
	crdReplicatorConfig *configv1alpha1.DispatcherConfig
	apiServerConfig     *configv1alpha1.APIServerConfig
	stopMDNS            chan bool
	stopMDNSClient      chan bool
	crdClient           *crdClient.CRDClient
	advClient           *crdClient.CRDClient
	ClusterId           clusterid.ClusterID

	mdnsServerAuth            *zeroconf.Server
	serverMux                 sync.Mutex
	resolveContextRefreshTime int

	dialTCPTimeout time.Duration
}

// NewDiscoveryCtrl returns a new discovery controller.
func NewDiscoveryCtrl(
	namespace string, localClusterID clusterid.ClusterID, kubeconfigPath string,
	resolveContextRefreshTime int, dialTCPTimeout time.Duration) (*Controller, error) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion, nil)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := crdClient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	advClient, err := advtypes.CreateAdvertisementClient(kubeconfigPath, nil, true, nil)
	if err != nil {
		klog.Error(err, "unable to create local client for Advertisement")
		os.Exit(1)
	}

	discoveryCtrl := getDiscoveryCtrl(
		namespace,
		discoveryClient,
		advClient,
		localClusterID,
		resolveContextRefreshTime,
		dialTCPTimeout,
	)
	if discoveryCtrl.getDiscoveryConfig(nil, kubeconfigPath) != nil {
		os.Exit(1)
	}
	return &discoveryCtrl, nil
}

func getDiscoveryCtrl(namespace string, client, advClient *crdClient.CRDClient,
	localClusterID clusterid.ClusterID, resolveContextRefreshTime int, dialTCPTimeout time.Duration) Controller {
	return Controller{
		Namespace:                 namespace,
		crdClient:                 client,
		advClient:                 advClient,
		ClusterId:                 localClusterID,
		stopMDNS:                  make(chan bool, 1),
		stopMDNSClient:            make(chan bool, 1),
		resolveContextRefreshTime: resolveContextRefreshTime,
		dialTCPTimeout:            dialTCPTimeout,
	}
}

// StartDiscovery starts register and resolver goroutines.
func (discovery *Controller) StartDiscovery() {
	go discovery.register()
	go discovery.startResolver(discovery.stopMDNSClient)
	go discovery.startGratuitousAnswers()
	go discovery.startGarbageCollector()
}

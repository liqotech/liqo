package discovery

import (
	"github.com/go-logr/logr"
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
	"github.com/netgroup-polito/dronev2/pkg/clusterID"
	v1 "github.com/netgroup-polito/dronev2/pkg/discovery/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

type DiscoveryCtrl struct {
	Namespace string
	Log       logr.Logger

	config          Config
	client          *kubernetes.Clientset
	clientDiscovery *v1.DiscoveryV1Client
	ClusterId       *clusterID.ClusterID
}

func NewDiscoveryCtrl(namespace string) (*DiscoveryCtrl, error) {
	client, err := clients.NewK8sClient()
	if err != nil {
		return nil, err
	}
	clientDiscovery, err := clients.NewDiscoveryClient()
	if err != nil {
		return nil, err
	}
	clusterId, err := clusterID.NewClusterID()
	if err != nil {
		return nil, err
	}
	discoveryCtrl := DiscoveryCtrl{
		Namespace:       namespace,
		Log:             ctrl.Log.WithName("discovery"),
		client:          client,
		clientDiscovery: clientDiscovery,
		ClusterId:       clusterId,
	}
	discoveryCtrl.GetDiscoveryConfig()
	return &discoveryCtrl, nil
}

// Read ConfigMap and start register and resolver goroutines
func (discovery *DiscoveryCtrl) StartDiscovery() {
	discovery.SetupConfigmap()

	if discovery.config.EnableAdvertisement {
		txtString, err := discovery.config.TxtData.Encode()
		if err != nil {
			discovery.Log.Error(err, err.Error())
			os.Exit(1)
		}

		discovery.Log.Info("Starting service advertisement")
		go discovery.Register(discovery.config.Name, discovery.config.Service, discovery.config.Domain, discovery.config.Port, txtString)
	}

	if discovery.config.EnableDiscovery {
		discovery.Log.Info("Starting service discovery")
		go discovery.StartResolver(discovery.config.Service, discovery.config.Domain, discovery.config.WaitTime, discovery.config.UpdateTime)
	}
}

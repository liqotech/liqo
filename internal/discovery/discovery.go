package discovery

import (
	"github.com/go-logr/logr"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	"github.com/liqoTech/liqo/pkg/clusterID"
	v1 "github.com/liqoTech/liqo/pkg/discovery/v1"
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

func NewDiscoveryCtrl(namespace string, clusterId *clusterID.ClusterID) (*DiscoveryCtrl, error) {
	client, err := clients.NewK8sClient()
	if err != nil {
		return nil, err
	}
	clientDiscovery, err := clients.NewDiscoveryClient()
	if err != nil {
		return nil, err
	}
	discoveryCtrl := GetDiscoveryCtrl(
		namespace,
		ctrl.Log.WithName("discovery"),
		client,
		clientDiscovery,
		clusterId,
	)
	if discoveryCtrl.GetDiscoveryConfig() != nil {
		os.Exit(1)
	}
	return &discoveryCtrl, nil
}

func GetDiscoveryCtrl(namespace string, log logr.Logger, client *kubernetes.Clientset, clientDiscovery *v1.DiscoveryV1Client, clusterId *clusterID.ClusterID) DiscoveryCtrl {
	return DiscoveryCtrl{
		Namespace:       namespace,
		Log:             log,
		client:          client,
		clientDiscovery: clientDiscovery,
		ClusterId:       clusterId,
	}
}

// Read ConfigMap and start register and resolver goroutines
func (discovery *DiscoveryCtrl) StartDiscovery() {
	if discovery.config.EnableAdvertisement {
		txtString, err := discovery.GetTxtData().Encode()
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

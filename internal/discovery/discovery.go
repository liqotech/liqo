package discovery

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	"github.com/liqoTech/liqo/pkg/clusterID"
	v1 "github.com/liqoTech/liqo/pkg/discovery/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"os"
)

type DiscoveryCtrl struct {
	Namespace string

	Config          *policyv1.DiscoveryConfig
	stopMDNS        chan bool
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
		client,
		clientDiscovery,
		clusterId,
	)
	if discoveryCtrl.GetDiscoveryConfig(nil) != nil {
		os.Exit(1)
	}
	return &discoveryCtrl, nil
}

func GetDiscoveryCtrl(namespace string, client *kubernetes.Clientset, clientDiscovery *v1.DiscoveryV1Client, clusterId *clusterID.ClusterID) DiscoveryCtrl {
	return DiscoveryCtrl{
		Namespace:       namespace,
		client:          client,
		clientDiscovery: clientDiscovery,
		ClusterId:       clusterId,
	}
}

// Start register and resolver goroutines
func (discovery *DiscoveryCtrl) StartDiscovery() {
	go discovery.Register()
	go discovery.StartResolver()
}

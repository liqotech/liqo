package discovery

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"os"
	"path/filepath"
)

type DiscoveryCtrl struct {
	Namespace string

	Config    *policyv1.DiscoveryConfig
	stopMDNS  chan bool
	crdClient *crdClient.CRDClient
	ClusterId *clusterID.ClusterID
}

func NewDiscoveryCtrl(namespace string, clusterId *clusterID.ClusterID) (*DiscoveryCtrl, error) {
	config, err := crdClient.NewKubeconfig(filepath.Join(os.Getenv("HOME"), ".kube", "config"), &discoveryv1.GroupVersion)
	if err != nil {
		return nil, err
	}
	crdClient, err := crdClient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}
	discoveryCtrl := GetDiscoveryCtrl(
		namespace,
		crdClient,
		clusterId,
	)
	if discoveryCtrl.GetDiscoveryConfig(nil) != nil {
		os.Exit(1)
	}
	return &discoveryCtrl, nil
}

func GetDiscoveryCtrl(namespace string, crdClient *crdClient.CRDClient, clusterId *clusterID.ClusterID) DiscoveryCtrl {
	return DiscoveryCtrl{
		Namespace: namespace,
		crdClient: crdClient,
		ClusterId: clusterId,
	}
}

// Start register and resolver goroutines
func (discovery *DiscoveryCtrl) StartDiscovery() {
	go discovery.Register()
	go discovery.StartResolver()
}

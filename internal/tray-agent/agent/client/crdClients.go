package client

import (
	"errors"
	"fmt"
	"github.com/liqotech/liqo/pkg/crdClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"os"
)

//CustomResource defines the CRD managed by Liqo Agent.
type CustomResource string

const (
	//CRClusterConfig is the resource id for the ClusterConfig CRD.
	CRClusterConfig CustomResource = "clusterconfigs"
	//CRAdvertisement is the resource id for the Advertisement CRD.
	CRAdvertisement CustomResource = "advertisements"
	//CRForeignCluster is the resource id for the ForeignCluster CRD.
	CRForeignCluster CustomResource = "foreignclusters"
)

//customResources contains all the registered CustomResource managed by the AgentController.
//It is used for init and testing purposes.
var customResources = []CustomResource{
	CRClusterConfig,
	CRAdvertisement,
	CRForeignCluster,
}

//crdManager stores the resources necessary to manage the CRDs.
type crdManager struct {
	//clientMap contains the Controllers for the CRDs managed by the Agent.
	clientMap map[CustomResource]*CRDController
}

//initCRDManager creates and initializes the crdManager, loading the CRDController for each
//required CRD.
func (ctrl *AgentController) initCRDManager() error {
	//struct init
	manager := &crdManager{clientMap: make(map[CustomResource]*CRDController)}
	ctrl.crdManager = manager
	kubeconfig, set := os.LookupEnv(EnvLiqoKConfig)
	if !set {
		return errors.New("no kubeconfig provided")
	}
	//creation of each single CRDController
	var err error
	var crdCtrl *CRDController
	for _, cr := range customResources {
		switch cr {
		case CRClusterConfig:
			crdCtrl, err = createClusterConfigController(kubeconfig)
		case CRAdvertisement:
			crdCtrl, err = createAdvertisementController(kubeconfig)
		case CRForeignCluster:
			crdCtrl, err = createForeignClusterController(kubeconfig)
		}
		if err != nil {
			return errors.New(fmt.Sprint("connection error on ", cr, " client creation"))
		}
		manager.clientMap[cr] = crdCtrl
	}
	return nil
}

//Controller returns (if present) the CRDController for a specific CRD.
func (m *crdManager) Controller(resource CustomResource) *CRDController {
	//controller, present = m.clientMap[resource]
	return m.clientMap[resource]
}

//CRDController handles the Agent interaction with the cluster for a specific CRD.
type CRDController struct {
	//CRDClient to perform CRUD operations on the CRD.
	*crdClient.CRDClient
	//resource is the CRD literal identifier.
	resource string
	//running specifies whether the CRD cache is running.
	running bool
	//addFunc is the handler for the 'resource added' event.
	addFunc func(obj interface{})
	//updateFunc is the handler for the 'resource updated' event.
	updateFunc func(oldObj interface{}, newObj interface{})
	//deleteFunc is the handler for the 'resource deleted' event.
	deleteFunc func(obj interface{})
}

//Running returns whether the controller cache is running.
func (c *CRDController) Running() bool {
	return c.running
}

//StartCache starts the CRD cache and the sending of signals
//on the Controller notifyChannels.
func (c *CRDController) StartCache() error {
	if c.running {
		return nil
	}
	ehf := cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addFunc,
		UpdateFunc: c.updateFunc,
		DeleteFunc: c.deleteFunc,
	}
	lo := metav1.ListOptions{}
	var err error
	c.Store, c.Stop, err = crdClient.WatchResources(
		c.CRDClient, c.resource, "", 0, ehf, lo)
	if err == nil {
		c.running = true
	}
	return err
}

//StopCache stops (if running) the cache associated for the CRD.
func (c *CRDController) StopCache() {
	if c.running {
		close(c.Stop)
		c.running = false
	}
}

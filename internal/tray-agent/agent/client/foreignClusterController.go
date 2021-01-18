package client

import (
	discovery "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharing "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	discovery2 "github.com/liqotech/liqo/pkg/discovery"
)

//createForeignClusterController creates a new CRDController for the Liqo ForeignCluster CRD.
func createForeignClusterController(kubeconfig string) (*CRDController, error) {
	controller := &CRDController{
		addFunc:    foreignclusterAddFunc,
		updateFunc: foreignclusterUpdateFunc,
		deleteFunc: foreignclusterDeleteFunc,
	}
	newClient, err := discovery.CreateForeignClusterClient(kubeconfig)
	if err != nil {
		return nil, err
	}
	controller.CRDClient = newClient
	controller.resource = string(CRForeignCluster)
	return controller, nil
}

//NotifyDataForeignCluster is a NotifyDataGeneric sub-type used to exchange data concerning ForeignClusters events.
type NotifyDataForeignCluster struct {
	//Name of the ForeignCluster CR (to enable further its further retrieval).
	Name        string
	ClusterID   string
	ClusterName string
	//LocalDiscovered identifies whether the peer has been discovered inside the home cluster LAN.
	LocalDiscovered bool
	//Trusted identifies whether the ForeignCluster has a valid certificate.
	Trusted discovery2.TrustMode
	//AuthStatus determines if the home cluster has been correctly authenticated on the foreign cluster.
	//This property determines the possibility to perform an outgoing peering.
	AuthStatus discovery2.AuthStatus
	//OutPeering contains information about the current status of the outgoing peering towards this foreign cluster.
	OutPeering struct {
		//Connected  determines whether the outgoing peering is established and running.
		Connected bool
		//CpuQuota is the literal representation of the CPU quota shared by the foreign cluster in the currently
		//active outgoing peering.
		CpuQuota string
		//MemQuota is the literal representation of the CPU quota shared by the foreign cluster in the currently
		//active outgoing peering.
		MemQuota string
	}
	//InPeering contains information about the current status of the incoming peering from this foreign cluster.
	InPeering struct {
		//Connected determines whether the incoming peering is established and running.
		Connected bool
	}
}

//			**** HELPERS ****
//	The following functions are helpers used to simplify the loading and sharing of information.

//loadPeerInfo loads useful data about the peer represented by a ForeignCluster.
func (d *NotifyDataForeignCluster) loadPeerInfo(fc *discovery.ForeignCluster) {
	d.Name = fc.Name
	d.ClusterID = fc.Spec.ClusterIdentity.ClusterID
	d.ClusterName = fc.Spec.ClusterIdentity.ClusterName
	//only distinguish local discovery which may represent useful information
	if fc.Spec.DiscoveryType == discovery2.LanDiscovery {
		d.LocalDiscovered = true
	}
	d.Trusted = fc.Spec.TrustMode
	d.AuthStatus = fc.Status.AuthStatus
}

//loadPeeringInfo loads useful data about peerings established with a ForeignCluster.
func (d *NotifyDataForeignCluster) loadPeeringInfo(fc *discovery.ForeignCluster) {
	//OUTGOING PEERING
	if fc.Status.Outgoing.Joined && fc.Status.Outgoing.AdvertisementStatus == sharing.AdvertisementAccepted {
		d.OutPeering.Connected = true
		//try to recover details on shared resources
		if advCtl := GetAgentController().Controller(CRAdvertisement); advCtl.Running() {
			if obj, exist, err := advCtl.Store.GetByKey(fc.Status.Outgoing.Advertisement.Name); exist && err == nil {
				if foreignAdv, ok := obj.(*sharing.Advertisement); ok {
					quotas := foreignAdv.Spec.ResourceQuota.Hard
					d.OutPeering.CpuQuota = quotas.Cpu().String()
					d.OutPeering.MemQuota = quotas.Memory().String()
				}
			}
		}
	}
	//INCOMING PEERING
	if fc.Status.Incoming.Joined && fc.Status.Incoming.AdvertisementStatus == sharing.AdvertisementAccepted {
		d.InPeering.Connected = true
	}
}

//			**** EVENT FUNCTIONS ****
//	The following functions are the callbacks the ForeignCluster Controller uses to handle the events
//	of the correspondent cache.

//foreignclusterAddFunc is the ADD event handler for the ForeignCluster CRDController.
func foreignclusterAddFunc(obj interface{}) {
	fc := obj.(*discovery.ForeignCluster)
	/*There are some cases when a just created ForeignCluster already contains information about a peering
	(pending or accepted), e.g. for a FC discovered due to an incoming peering request or with a peering
	established before the Agent start.*/

	/*If the ClusterID is not provided, it means that the FC cluster identity is yet to be retrieved
	from the authN endpoint of the foreign cluster. In this case the "new peer" event is handled when this kind
	of information is provided.*/
	if fc.Spec.ClusterIdentity.ClusterID == "" {
		return
	}
	data := &NotifyDataForeignCluster{}
	data.loadPeerInfo(fc)
	data.loadPeeringInfo(fc)
	agentCtrl.NotifyChannel(ChanPeerAdded) <- data
}

//foreignclusterUpdateFunc is the UPDATE event handler for the ForeignCluster CRDController.
func foreignclusterUpdateFunc(oldObj interface{}, newObj interface{}) {
	fcOld := oldObj.(*discovery.ForeignCluster)
	fcNew := newObj.(*discovery.ForeignCluster)
	if fcNew.Spec.ClusterIdentity.ClusterID == "" {
		return
	}
	data := &NotifyDataForeignCluster{}
	data.loadPeerInfo(fcNew)
	data.loadPeeringInfo(fcNew)
	//recover CREATE case when there was no cluster identity available yet and
	//treat it as a 'peer added' event
	if fcOld.Spec.ClusterIdentity.ClusterID == "" && fcNew.Spec.ClusterIdentity.ClusterID != "" {
		agentCtrl.NotifyChannel(ChanPeerAdded) <- data
	}
	agentCtrl.NotifyChannel(ChanPeerUpdated) <- data
}

//foreignclusterDeleteFunc is the DELETE event handler for the ForeignCluster CRDController.
func foreignclusterDeleteFunc(obj interface{}) {
	fc := obj.(*discovery.ForeignCluster)
	data := &NotifyDataForeignCluster{}
	data.loadPeerInfo(fc)
	data.loadPeeringInfo(fc)
	agentCtrl.NotifyChannel(ChanPeerDeleted) <- data
}

package client

import (
	discovery "github.com/liqotech/liqo/apis/discovery/v1alpha1"
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

//foreignclusterAddFunc is the ADD event handler for the ForeignCluster CRDController.
func foreignclusterAddFunc(obj interface{}) {
	fc := obj.(*discovery.ForeignCluster)
	agentCtrl.NotifyChannel(ChanPeerAdded) <- fc.Name
}

//foreignclusterUpdateFunc is the UPDATE event handler for the ForeignCluster CRDController.
func foreignclusterUpdateFunc(oldObj interface{}, newObj interface{}) {
	fcOld := oldObj.(*discovery.ForeignCluster)
	fcNew := newObj.(*discovery.ForeignCluster)
	//currently the handler only monitors updates on cluster information, not
	//a peering workflow
	//updates on foreign cluster data
	if fcNew.Spec.ClusterIdentity.ClusterName != fcOld.Spec.ClusterIdentity.ClusterName {
		agentCtrl.NotifyChannel(ChanPeerUpdated) <- fcNew.Name
	}
	//monitor changes os peering operations
	//outgoing
	if outPeeringNew, outPeeringOld := fcNew.Status.Outgoing.Joined, fcOld.Status.Outgoing.Joined; outPeeringNew == true && outPeeringOld != outPeeringNew {
		//new outgoing peering active
		agentCtrl.NotifyChannel(ChanPeeringOutgoingNew) <- fcNew.Name
	} else if outPeeringOld == true && outPeeringNew != outPeeringOld {
		//outgoing peering torn down
		agentCtrl.NotifyChannel(ChanPeeringOutgoingDelete) <- fcNew.Name
	}
	if inPeeringNew, inPeeringOld := fcNew.Status.Incoming.Joined, fcOld.Status.Incoming.Joined; inPeeringNew == true && inPeeringOld != inPeeringNew {
		//new incoming peering active
		agentCtrl.NotifyChannel(ChanPeeringIncomingNew) <- fcNew.Name
	} else if inPeeringOld == true && inPeeringNew != inPeeringOld {
		//incoming peering torn down
		agentCtrl.NotifyChannel(ChanPeeringIncomingDelete) <- fcNew.Name
	}

}

//foreignclusterDeleteFunc is the DELETE event handler for the ForeignCluster CRDController.
func foreignclusterDeleteFunc(obj interface{}) {
	fc := obj.(*discovery.ForeignCluster)
	if fc.Status.Outgoing.Joined == true {
		agentCtrl.NotifyChannel(ChanPeeringOutgoingDelete) <- fc.Name
	}
	if fc.Status.Incoming.Joined == true {
		agentCtrl.NotifyChannel(ChanPeeringIncomingDelete) <- fc.Name
	}
	agentCtrl.NotifyChannel(ChanPeerDeleted) <- fc.Name
}

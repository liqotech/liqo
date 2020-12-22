package logic

import (
	"fmt"
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqotech/liqo/internal/tray-agent/app-indicator"
)

//this file contains the callback functions for the Indicator listeners

//******* PEERINGS *******

func listenNewIncomingPeering(obj string, args ...interface{}) {
	i := app.GetIndicator()
	st := i.Status()
	//update Status model
	st.IncDecPeerings(app.PeeringIncoming, true)
	i.RefreshStatus()
	//retrieve peering info
	//retrieve Peer information
	quickNode, present := i.Quick(qPeers)
	if !present {
		return
	}
	//update peer information in the list on the Incoming entry (peers -> this peer -> incoming peering)
	var peerNode, iPeeringNode *app.MenuNode
	peerNode, present = quickNode.ListChild(obj)
	if !present {
		return
	}
	iPeeringNode, present = peerNode.ListChild(tagIncoming)
	if !present {
		panic("no incoming MenuNode for a Peer element")
	}
	iPeeringNode.SetIsChecked(true)
	//todo next version of this feature will display a subelement containing shared resources
	//notify new peering
	i.Notify("INCOMING PEERING ACCEPTED", fmt.Sprintf("You are offering resources to %s", obj), app.NotifyIconDefault, app.IconLiqoPurple)
}

func listenDeleteIncomingPeering(obj string, args ...interface{}) {
	i := app.GetIndicator()
	st := i.Status()
	//update Status model
	st.IncDecPeerings(app.PeeringIncoming, false)
	i.RefreshStatus()
	//retrieve peering info
	//retrieve Peer information
	quickNode, present := i.Quick(qPeers)
	if !present {
		return
	}
	//update peer information in the list on the Incoming entry (peers -> this peer -> incoming peering)
	var peerNode, iPeeringNode *app.MenuNode
	peerNode, present = quickNode.ListChild(obj)
	if !present {
		return
	}
	iPeeringNode, present = peerNode.ListChild(tagIncoming)
	if !present {
		panic("no incoming MenuNode for a Peer element")
	}
	iPeeringNode.SetIsChecked(false)
	//todo next version of this feature will display a subelement containing shared resources
	//notify peering disconnected
	i.Notify("INCOMING PEERING CLOSED", fmt.Sprintf("Stopped sharing resources to %s", obj), app.NotifyIconDefault, app.IconLiqoRed)
}

func listenNewOutgoingPeering(obj string, args ...interface{}) {
	i := app.GetIndicator()
	st := i.Status()
	//update Status model
	st.IncDecPeerings(app.PeeringOutgoing, true)
	i.RefreshStatus()
	//retrieve peering info
	//retrieve Peer information
	quickNode, present := i.Quick(qPeers)
	if !present {
		return
	}
	//update peer information in the list on the Outgoing entry (peers -> this peer -> outgoing peering)
	var peerNode, oPeeringNode *app.MenuNode
	peerNode, present = quickNode.ListChild(obj)
	if !present {
		return
	}
	oPeeringNode, present = peerNode.ListChild(tagOutgoing)
	if !present {
		panic("no outgoing MenuNode for a Peer element")
	}
	oPeeringNode.SetIsChecked(true)
	//todo next version of this feature will display a subelement containing shared resources
	//notify new peering
	i.Notify("OUTGOING PEERING ACCEPTED", fmt.Sprintf("You are receiving resources from %s", obj), app.NotifyIconDefault, app.IconLiqoPurple)
}

func listenDeleteOutgoingPeering(obj string, args ...interface{}) {
	i := app.GetIndicator()
	st := i.Status()
	//update Status model
	st.IncDecPeerings(app.PeeringOutgoing, false)
	i.RefreshStatus()
	//retrieve peering info
	//retrieve Peer information
	quickNode, present := i.Quick(qPeers)
	if !present {
		return
	}
	//update peer information in the list on the Incoming entry (peers -> this peer -> incoming peering)
	var peerNode, oPeeringNode *app.MenuNode
	peerNode, present = quickNode.ListChild(obj)
	if !present {
		return
	}
	oPeeringNode, present = peerNode.ListChild(tagOutgoing)
	if !present {
		panic("no outgoing MenuNode for a Peer element")
	}
	oPeeringNode.SetIsChecked(false)
	//todo next version of this feature will display a subelement containing shared resources
	//notify peering disconnected
	i.Notify("OUTGOING PEERING CLOSED", fmt.Sprintf("Stopped receiving resources from %s", obj), app.NotifyIconDefault, app.IconLiqoRed)
}

//******* PEERS *******

func listenNewPeer(objName string, args ...interface{}) {
	i := app.GetIndicator()
	//retrieve Peer information
	quickNode, present := i.Quick(qPeers)
	if !present {
		return
	}
	fcCtrl := i.AgentCtrl().Controller(client.CRForeignCluster)
	obj, exist, err := fcCtrl.Store.GetByKey(objName)
	if err == nil && exist {
		fCluster := obj.(*v1alpha1.ForeignCluster)
		clusterID := fCluster.Spec.ClusterIdentity.ClusterID
		clusterName := fCluster.Spec.ClusterIdentity.ClusterName
		//avoid potential duplicate
		_, present = quickNode.ListChild(clusterID)
		if present {
			return
		}
		//show cluster name as main information. The clusterID is inserted inside a status sub-element for consultation.
		peerNode := quickNode.UseListChild(clusterName, clusterID)
		statusNode := peerNode.UseListChild(clusterID, tagStatus)
		statusNode.SetIsEnabled(false)
		peerNode.UseListChild(tagIncoming, tagIncoming)
		peerNode.UseListChild(tagOutgoing, tagOutgoing)
		//update the counter in the menu entry
		i.Status().IncDecPeers(true)
		i.RefreshStatus()
		refreshPeerCount(quickNode)
	}
}

func listenUpdatedPeer(objName string, args ...interface{}) {
	i := app.GetIndicator()
	//retrieve Peer information
	quickNode, present := i.Quick(qPeers)
	if !present {
		return
	}
	//in this case it is not necessary to get the ClusterID information (which is the required key to access the
	//dynamic list), since the ForeignCluster 'Name' metadata coincides with it.
	quickNode.FreeListChild(objName)
	//update the counter in the menu entry
	i.Status().IncDecPeers(false)
	i.RefreshStatus()
	refreshPeerCount(quickNode)
}

func listenDeletedPeer(objName string, args ...interface{}) {
	i := app.GetIndicator()
	//retrieve Peer information
	quickNode, present := i.Quick(qPeers)
	if !present {
		return
	}
	fcCtrl := i.AgentCtrl().Controller(client.CRForeignCluster)
	obj, exist, err := fcCtrl.Store.GetByKey(objName)
	if err == nil && exist {
		fCluster := obj.(*v1alpha1.ForeignCluster)
		clusterID := fCluster.Spec.ClusterIdentity.ClusterID
		clusterName := fCluster.Spec.ClusterIdentity.ClusterName
		var peerNode *app.MenuNode
		peerNode, present = quickNode.ListChild(clusterID)
		if !present {
			return
		}
		//show cluster name as main information. The clusterID is inserted inside a status sub-element for consultation.
		peerNode.SetTitle(clusterName)
	}
}

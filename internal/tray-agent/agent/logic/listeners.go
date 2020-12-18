package logic

import (
	"fmt"
	app "github.com/liqotech/liqo/internal/tray-agent/app-indicator"
)

//this file contains the callback functions for the Indicator listeners

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

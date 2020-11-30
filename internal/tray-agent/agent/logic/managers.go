package logic

import (
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqotech/liqo/internal/tray-agent/app-indicator"
	"github.com/skratchdot/open-golang/open"
)

//OnReady is the routine orchestrating Liqo Agent execution.
func OnReady() {
	// Indicator configuration
	i := app.GetIndicator()
	i.RefreshStatus()
	startListenerAdvertisements(i)
	startListenerPeersList(i)
	startQuickOnOff(i)
	startQuickChangeMode(i)
	startQuickDashboard(i)
	startQuickSetNotifications(i)
	startQuickLiqoWebsite(i)
	startQuickShowPeers(i)
	startQuickQuit(i)
	//try to start Liqo and main ACTION
	quickTurnOnOff(i)
}

//OnExit is the routine containing clean-up operations to be performed at Liqo Agent exit.
func OnExit() {
	app.GetIndicator().Disconnect()
}

//startQuickOnOff is the wrapper function to register the QUICK "START/STOP LIQO".
func startQuickOnOff(i *app.Indicator) {
	i.AddQuick("", qOnOff, func(args ...interface{}) {
		quickTurnOnOff(args[0].(*app.Indicator))
	}, i)
	//the Quick MenuNode title is refreshed
	updateQuickTurnOnOff(i)
}

//startQuickChangeMode is the wrapper function to register the QUICK "CHANGE LIQO MODE"
func startQuickChangeMode(i *app.Indicator) {
	i.AddQuick("", qMode, func(args ...interface{}) {
		quickChangeMode(i)
	}, i)
	//the Quick MenuNode title is refreshed
	updateQuickChangeMode(i)
}

//startQuickLiqoWebsite is the wrapper function to register QUICK "About Liqo".
func startQuickLiqoWebsite(i *app.Indicator) {
	i.AddQuick("ⓘ ABOUT LIQO", qWeb, func(args ...interface{}) {
		_ = open.Start("http://liqo.io")
	})
}

//startQuickDashboard is the wrapper function to register QUICK "LAUNCH Liqo Dash".
func startQuickDashboard(i *app.Indicator) {
	i.AddQuick("LIQODASH", qDash, func(args ...interface{}) {
		quickConnectDashboard(i)
	})
}

//startQuickSetNotifications is the wrapper function to register QUICK "Change Notification settings".
func startQuickSetNotifications(i *app.Indicator) {
	i.AddQuick("NOTIFICATIONS SETTINGS", qNotify, func(args ...interface{}) {
		quickChangeNotifyLevel()
	})
}

//startQuickQuit is the wrapper function to register QUICK "QUIT".
func startQuickQuit(i *app.Indicator) {
	i.AddQuick("QUIT", qQuit, func(args ...interface{}) {
		i := args[0].(*app.Indicator)
		i.Quit()
	}, i)
}

//startQuickShowPeers is the wrapper function to register QUICK "PEERS".
func startQuickShowPeers(i *app.Indicator) {
	i.AddQuick("AVAILABLE PEERS", qPeers, nil)
}

//LISTENERS

// wrapper that starts the Listeners for the events regarding the Advertisement CRD
func startListenerAdvertisements(i *app.Indicator) {
	i.Listen(client.ChanAdvNew, i.AgentCtrl().NotifyChannel(client.ChanAdvNew), func(objName string, args ...interface{}) {
		ctrl := i.AgentCtrl()
		if !ctrl.Mocked() {
			advStore := ctrl.Controller(client.CRAdvertisement).Store
			_, exist, err := advStore.GetByKey(objName)
			if err != nil {
				i.NotifyNoConnection()
				return
			}
			if !exist {
				return
			}
		}
		i.NotifyNewAdv(objName)
	})
	i.Listen(client.ChanAdvAccepted, i.AgentCtrl().NotifyChannel(client.ChanAdvAccepted), func(objName string, args ...interface{}) {
		ctrl := i.AgentCtrl()
		if !ctrl.Mocked() {
			advStore := ctrl.Controller(client.CRAdvertisement).Store
			_, exist, err := advStore.GetByKey(objName)
			if err != nil {
				i.NotifyNoConnection()
				return
			}
			if !exist {
				return
			}
		}
		i.NotifyAcceptedAdv(objName)
		i.Status().IncConsumePeerings()
	})
	i.Listen(client.ChanAdvRevoked, i.AgentCtrl().NotifyChannel(client.ChanAdvRevoked), func(objName string, args ...interface{}) {
		ctrl := i.AgentCtrl()
		if !ctrl.Mocked() {
			advStore := ctrl.Controller(client.CRAdvertisement).Store
			_, exist, err := advStore.GetByKey(objName)
			if err != nil {
				i.NotifyNoConnection()
				return
			}
			if !exist {
				return
			}
		}
		i.NotifyRevokedAdv(objName)
		i.Status().DecConsumePeerings()
	})
	i.Listen(client.ChanAdvDeleted, i.AgentCtrl().NotifyChannel(client.ChanAdvDeleted), func(objName string, args ...interface{}) {
		i.NotifyDeletedAdv(objName)
		i.Status().DecConsumePeerings()
	})
}

/*startListenerPeersList is a wrapper that starts the listeners regarding the dynamic listing of Liqo discovered Liqo peers.
  Since these listeners work on a specific QUICK MenuNode, the associated handlers works only if that QUICK
  is registered in the Indicator.*/
func startListenerPeersList(i *app.Indicator) {
	i.Listen(client.ChanPeerAdded, i.AgentCtrl().NotifyChannel(client.ChanPeerAdded), func(objName string, args ...interface{}) {
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
		}
	})
	i.Listen(client.ChanPeerDeleted, i.AgentCtrl().NotifyChannel(client.ChanPeerDeleted), func(objName string, args ...interface{}) {
		//retrieve Peer information
		quickNode, present := i.Quick(qPeers)
		if !present {
			return
		}
		//in this case it is not necessary to get the ClusterID information (which is the required key to access the
		//dynamic list), since the ForeignCluster 'Name' metadata coincides with it.
		quickNode.FreeListChild(objName)
	})
	i.Listen(client.ChanPeerUpdated, i.AgentCtrl().NotifyChannel(client.ChanPeerUpdated), func(objName string, args ...interface{}) {
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
	})
}

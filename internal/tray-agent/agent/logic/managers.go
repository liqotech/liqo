package logic

import (
	"github.com/liqoTech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqoTech/liqo/internal/tray-agent/app-indicator"
	"os/exec"
)

// set of action tags
const (
	aShowPeers = "A_SHOW_PEERS"
)

// set of options
const (
	oAddPeer = "O_ADD_PEER"
)

//set of timer tags
const (
	timerStatus = "T_STATUS"
)

//OnReady is the routine orchestrating Liqo Agent execution.
func OnReady() {
	// Indicator configuration
	i := app.GetIndicator()
	i.SetMenuTitle("Liqo Agent")
	i.RefreshStatus()
	startListenerAdvertisements(i)
	startQuickOnOff(i)
	startQuickChangeMode(i)
	startQuickDashboard(i)
	startQuickSetNotifications(i)
	startQuickLiqoWebsite(i)
	startQuickQuit(i)
	//todo auto selection of startActionPeers
	startActionPeers(i)
	//
	//start liqo
	quickTurnOnOff(i)
}

//OnReady is the routine orchestrating Liqo Tray Agent execution.
func OldReady() {
	// Indicator configuration
	i := app.GetIndicator()
	i.SetMenuTitle("Liqo Agent")
	// LISTENERS insertion
	startListenerAdvertisements(i)
	// QUICKS insertion

	startQuickDashboard(i)
	startQuickQuit(i)
	//
	i.AddSeparator()
	//
	// ACTIONS insertion
	startActionPeers(i)
	//todo add optional start liqo
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
	i.AddQuick("",qMode, func(args ...interface{}) {
		quickChangeMode(i)
	},i)
	//the Quick MenuNode title is refreshed
	updateQuickChangeMode(i)
}

//startQuickLiqoWebsite is the wrapper function to register QUICK "About Liqo".
func startQuickLiqoWebsite(i *app.Indicator) {
	i.AddQuick("ⓘ About Liqo", qWeb, func(args ...interface{}) {
		cmd := exec.Command("xdg-open", "http://liqo.io")
		_ = cmd.Run()
	})
}

//startQuickDashboard is the wrapper function to register QUICK "LAUNCH Liqo Dash".
func startQuickDashboard(i *app.Indicator) {
	i.AddQuick("LiqoDash", qDash, func(args ...interface{}) {
		cmd := exec.Command("xdg-open", "http://liqo.io")
		_ = cmd.Run()
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

//startActionPeers is the wrapper function to register ACTION "Show available peers".
func startActionPeers(i *app.Indicator) {
	i.AddAction("Show Advertisements", aShowPeers, func(args ...interface{}) {
		actionShowAdv()
	})
}

//LISTENERS

// wrapper that starts the Listeners for the events regarding the Advertisement CRD
func startListenerAdvertisements(i *app.Indicator) {
	i.Listen(client.ChanAdvNew, i.AgentCtrl().AdvCache().NotifyChannels[client.ChanAdvNew], func(objName string, args ...interface{}) {
		ctrl := i.AgentCtrl()
		if !ctrl.Mocked() {
			advStore := ctrl.AdvCache().Store
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
	i.Listen(client.ChanAdvAccepted, i.AgentCtrl().AdvCache().NotifyChannels[client.ChanAdvAccepted], func(objName string, args ...interface{}) {
		ctrl := i.AgentCtrl()
		if !ctrl.Mocked() {
			advStore := ctrl.AdvCache().Store
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
	i.Listen(client.ChanAdvRevoked, i.AgentCtrl().AdvCache().NotifyChannels[client.ChanAdvRevoked], func(objName string, args ...interface{}) {
		ctrl := i.AgentCtrl()
		if !ctrl.Mocked() {
			advStore := ctrl.AdvCache().Store
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
	i.Listen(client.ChanAdvDeleted, i.AgentCtrl().AdvCache().NotifyChannels[client.ChanAdvDeleted], func(objName string, args ...interface{}) {
		i.NotifyDeletedAdv(objName)
	})
}

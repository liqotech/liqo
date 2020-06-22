package logic

import (
	"github.com/liqoTech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqoTech/liqo/internal/tray-agent/app-indicator"
	"os/exec"
)

// set of quick tags
const (
	qQuit = "Q_QUIT"
	qHome = "Q_HOME"
	qDash = "Q_LAUNCH_DASH"
)

// set of action tags
const (
	aShowAdv  = "A_SHOW_ADV"
	aSettings = "A_SETTINGS"
)

// set of options
const (
	oChangeNotify = "O_CHANGE_NOTIFY"
)

//OnReady is the routine orchestrating Liqo Tray Agent execution.
func OnReady() {
	// Indicator configuration
	i := app.GetIndicator()
	i.SetMenuTitle("Liqo Agent")
	// LISTENERS insertion
	startListenerAdvertisements(i)
	// QUICKS insertion
	startQuickHome(i)
	startQuickDashboard(i)
	startQuickQuit(i)
	//
	i.AddSeparator()
	//
	// ACTIONS insertion
	startActionAdvertisements(i)
	startActionSettings(i)
}

//OnExit is the routine containing clean-up operations to be performed at Liqo Tray Agent exit
func OnExit() {
	app.GetIndicator().Disconnect()
}

func startQuickHome(i *app.Indicator) {
	i.AddQuick("HOME", qHome, func(args ...interface{}) {
		i.DeselectAction()
	})
}

func startQuickDashboard(i *app.Indicator) {
	i.AddQuick("LAUNCH LiqoDash", qDash, func(args ...interface{}) {
		cmd := exec.Command("xdg-open", "http://liqo.io")
		_ = cmd.Run()
	})
}

func startQuickQuit(i *app.Indicator) {
	i.AddQuick("QUIT", qQuit, func(args ...interface{}) {
		app.Quit()
	})
}

func startActionAdvertisements(i *app.Indicator) {
	i.AddAction("Show Advertisements", aShowAdv, func(args ...interface{}) {
		actionShowAdv()
	})
}

func startActionSettings(i *app.Indicator) {
	act := i.AddAction("Settings", aSettings, func(args ...interface{}) {
		actionSettings()
	})
	act.AddOption("notifications", oChangeNotify, func(args ...interface{}) {
		optionChangeNotifyLevel()
	})
}

//LISTENERS

func startListenerAdvertisements(i *app.Indicator) {
	i.Listen(client.ChanAdvNew, i.AgentCtrl().AdvCache().NotifyChannels[client.ChanAdvNew], func(objName string, args ...interface{}) {
		advStore := i.AgentCtrl().AdvCache().Store
		_, exist, err := advStore.GetByKey(objName)
		if err != nil {
			i.NotifyNoConnection()
			return
		}
		if !exist {
			return
		}
		i.NotifyNewAdv(objName)
	})
	i.Listen(client.ChanAdvAccepted, i.AgentCtrl().AdvCache().NotifyChannels[client.ChanAdvAccepted], func(objName string, args ...interface{}) {
		advStore := i.AgentCtrl().AdvCache().Store
		_, exist, err := advStore.GetByKey(objName)
		if err != nil {
			i.NotifyNoConnection()
			return
		}
		if !exist {
			return
		}
		i.NotifyAcceptedAdv(objName)
	})
	i.Listen(client.ChanAdvRevoked, i.AgentCtrl().AdvCache().NotifyChannels[client.ChanAdvRevoked], func(objName string, args ...interface{}) {
		advStore := i.AgentCtrl().AdvCache().Store
		_, exist, err := advStore.GetByKey(objName)
		if err != nil {
			i.NotifyNoConnection()
			return
		}
		if !exist {
			return
		}
		i.NotifyRevokedAdv(objName)
	})
	i.Listen(client.ChanAdvDeleted, i.AgentCtrl().AdvCache().NotifyChannels[client.ChanAdvDeleted], func(objName string, args ...interface{}) {

	})
}

package logic

import (
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/gen2brain/dlgs"
	"github.com/liqotech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqotech/liqo/internal/tray-agent/app-indicator"
	"github.com/skratchdot/open-golang/open"
	"os"
	"strings"
)

// set of quick tags
const (
	qOnOff  = "Q_ON_OFF"
	qMode   = "Q_MODE"
	qDash   = "Q_LAUNCH_DASH"
	qWeb    = "Q_WEBSITE"
	qNotify = "Q_NOTIFY"
	qQuit   = "Q_QUIT"
	qPeers  = "Q_PEERS"
)

// set of frequently used tags inside application logic
const (
	tagStatus = "status"
)

//quickTurnOnOff is the callback for the QUICK "START/STOP LIQO".
func quickTurnOnOff(i *app.Indicator) {
	runSt := i.Status().Running()
	switch runSt {
	case app.StatRunOff:
		//turning ON Liqo if possible
		if i.AgentCtrl().Connected() {
			i.Status().SetRunning(app.StatRunOn)
			updateQuickTurnOnOff(i)
			i.RefreshStatus()
		}
	case app.StatRunOn:
		//turning OFF Liqo
		//todo insert "shutdown peerings" logic
		i.Status().SetRunning(app.StatRunOff)
		updateQuickTurnOnOff(i)
		i.RefreshStatus()
		i.SetIcon(app.IconLiqoMain)
		//the active ACTION is turned off
		i.DeselectAction()
	}
}

//updateQuickTurnOnOff is the callback that refreshes the QUICK MenuNode
//"START/STOP LIQO" accordingly to internal status information.
func updateQuickTurnOnOff(i *app.Indicator) {
	if q, present := i.Quick(qOnOff); present {
		title := strings.Builder{}
		switch i.Status().Running() {
		case app.StatRunOff:
			title.WriteString("START")
		case app.StatRunOn:
			title.WriteString("STOP")
		}
		title.WriteString(" LIQO")
		q.SetTitle(title.String())
	}
}

//quickChangeMode is the callback that manages the QUICK "Change Liqo Mode".
func quickChangeMode(i *app.Indicator) {
	stat := i.Status()
	mode := stat.Mode()
	switch mode {
	case app.StatModeAutonomous:
		//transition to TETHERED mode
		if err := stat.SetMode(app.StatModeTethered); err == nil {
			//todo transition logic
			updateQuickChangeMode(i)
			i.RefreshStatus()
		} else {
			i.ShowWarningForbiddenTethered()
		}
	case app.StatModeTethered:
		//transition to AUTONOMOUS mode
		if err := stat.SetMode(app.StatModeAutonomous); err == nil {
			//todo transition logic
			updateQuickChangeMode(i)
			i.RefreshStatus()
		} else {
			i.ShowWarning("LIQO AGENT", "Mode change not allowed.")
		}

	}
}

//updateQuickChangeMode refreshes the QUICK MenuNode "Change Liqo Mode"
//accordingly to internal status information.
func updateQuickChangeMode(i *app.Indicator) {
	if q, present := i.Quick(qMode); present {
		title := strings.Builder{}
		title.WriteString("SET ")
		mode := i.Status().Mode()
		switch mode {
		case app.StatModeAutonomous:
			title.WriteString(app.StatModeTetheredHeaderDescription)
			//check if TETHERED mode is eligible
			q.SetIsEnabled(i.Status().IsTetheredCompliant())
		case app.StatModeTethered:
			title.WriteString(app.StatModeAutonomousHeaderDescription)
			//In the current implementation, it is always possible ti switch into AUTONOMOUS mode.
			q.SetIsEnabled(true)
		}
		title.WriteString(" MODE")
		q.SetTitle(title.String())
	}
}

//quickChangeNotifyLevel is the callback function for the QUICK "Notifications Settings".
func quickChangeNotifyLevel() {
	i := app.GetIndicator()
	if !app.GetGuiProvider().Mocked() {
		notifyDescription := i.Config().NotifyDescriptions()
		level, ok, _ := dlgs.List("NOTIFICATION SETTINGS", fmt.Sprintf("Choose how you would like to receive "+
			"notifications from Liqo.\n"+
			"CURRENT: %s", i.Config().NotifyTranslate(i.Config().NotifyLevel())), notifyDescription)
		if ok {
			i.NotificationSetLevel(i.Config().NotifyTranslateReverse(level))
		}
	}
}

//quickConnectDashboard is the callback function for the QUICK "Launch LiqoDash".
//
//- If LiqoDash connection parameters are set (or can be retrieved), it opens the LiqoDash address
//in the default browser.
//
//- Then, it searches for an access token in the cluster and provides it to the user directly in the
//clipboard, ready to be pasted.
func quickConnectDashboard(i *app.Indicator) {
	ctrl := i.AgentCtrl()
	//Check if connection parameters are already set in the env vars to speed execution up.
	host, ok1 := os.LookupEnv(client.EnvLiqoDashHost)
	port, ok2 := os.LookupEnv(client.EnvLiqoDashPort)
	if !ok1 || !ok2 {
		if err := ctrl.AcquireDashboardConfig(); err != nil {
			i.Notify("Liqo Agent: SERVICE UNAVAILABLE", err.Error(),
				app.NotifyIconDefault, app.IconLiqoNil)
			return
		}
		host = os.Getenv(client.EnvLiqoDashHost)
		port = os.Getenv(client.EnvLiqoDashPort)
	}
	dashUrlBuilder := strings.Builder{}
	dashUrlBuilder.WriteString(host)
	if port != "" {
		dashUrlBuilder.WriteString(":" + port)
	}
	dashUrl := dashUrlBuilder.String()
	if err := open.Run(dashUrl); err == nil {
		//try to recover access token
		if token, errNFound := ctrl.GetLiqoDashSecret(); errNFound == nil {
			if err = clipboard.WriteAll(*token); err == nil {
				i.Notify("Liqo Agent", "The LiqoDash access token was copied in your clipboard",
					app.NotifyIconDefault, app.IconLiqoNil)
			} else {
				i.ShowWarning("LIQO AGENT", "Liqo Agent could not copy LiqoDash access token\n"+
					"to the clipboard")
			}
		} else {
			i.Notify("Liqo Agent", "LiqoDash access token was not found",
				app.NotifyIconDefault, app.IconLiqoNil)
		}
	}
}

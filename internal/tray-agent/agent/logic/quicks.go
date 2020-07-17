package logic

import (
	"fmt"
	"github.com/gen2brain/dlgs"
	app "github.com/liqoTech/liqo/internal/tray-agent/app-indicator"
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
)

//quickTurnOnOff is the callback for the QUICK "START/STOP LIQO".
func quickTurnOnOff(i *app.Indicator) {
	runSt := i.Status().Running()
	switch runSt {
	case app.StatRunOff:
		if i.AgentCtrl().Connected() {
			i.Status().SetRunning(app.StatRunOn)
			updateQuickTurnOnOff(i)
			//user can access the ACTION
			i.SelectAction(aShowPeers)
		}
	case app.StatRunOn:
		//todo insert "shutdown peerings" logic
		i.Status().SetRunning(app.StatRunOff)
		updateQuickTurnOnOff(i)
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
			i.ShowWarning("LIQO AGENT","Mode change not allowed.")
		}
	case app.StatModeTethered:
		//transition to AUTONOMOUS mode
		if err := stat.SetMode(app.StatModeAutonomous); err == nil {
			//todo transition logic
			updateQuickChangeMode(i)
			i.RefreshStatus()
		} else {
			i.ShowWarningForbiddenTethered()
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

//quickChangeNotifyLevel is the callback function for the OPTION "notifications" of "settings" action.
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

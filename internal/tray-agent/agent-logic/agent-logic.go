package agent_logic

import (
	agent "github.com/liqoTech/liqo/internal/tray-agent/agent-client"
	app "github.com/liqoTech/liqo/internal/tray-agent/app-indicator"
	"os/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// set of quick tags
const (
	qQuit = "Q_QUIT"
	qHome = "Q_HOME"
	qDash = "Q_LAUNCH_DASH"
)

// set of action tags
const (
	aShowAdv = "A_SHOW_ADV"
)

func OnReady() {
	indicator := app.GetIndicator()
	indicator.NotificationSetLevel(app.NotifyLevelMax)
	indicator.SetLabel("Liqo")
	indicator.SetMenuTitle("Liqo Agent")
	//insert Quicks
	indicator.AddQuick("HOME", qHome, func(args ...interface{}) {
		indicator.DeselectAction()
	})
	indicator.AddQuick("LAUNCH LiqoDash", qDash, func(args ...interface{}) {
		cmd := exec.Command("xdg-open", "http://liqo.io")
		_ = cmd.Run()
	})
	indicator.AddQuick("QUIT", qQuit, func(args ...interface{}) {
		app.Quit()
	})
	indicator.AddSeparator()
	//insert Actions
	AdvClient, err := agent.CreateClient(agent.AcquireConfig())
	if err != nil {
		indicator.Notify("Liqo", "Agent could not connect to the cluster", app.NotifyIconNoConn, app.IconLiqoNoConn)
		return
	}
	advAction := indicator.AddAction("Show Advertisements", aShowAdv, nil)
	advAction.Connect(func(args ...interface{}) {
		actionShowAdv(args[0].(*client.Client))
	}, &AdvClient)
}

func OnExit() {
	i := app.GetIndicator()
	i.Disconnect()
}

func actionShowAdv(c *client.Client) {
	liqo := app.GetIndicator()
	act, pres := liqo.Action(aShowAdv)
	if !pres {
		return
	}
	advList, err := agent.ListAdvertisements(c)
	if err != nil {
		liqo.Notify("Liqo", "Agent could not connect to the cluster", app.NotifyIconNoConn, app.IconLiqoNoConn)
		return
	} else {
		app.GetIndicator().SelectAction(aShowAdv)
		for _, adv := range advList {
			element := act.UseListChild()
			element.SetTitle(adv)
		}
	}
}

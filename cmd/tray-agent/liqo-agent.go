package main

import (
	agent "github.com/liqoTech/liqo/internal/tray-agent/agent-client"
	"github.com/liqoTech/liqo/internal/tray-agent/app-indicator"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AShowAdv = "A_SHOW_ADV"
)

func main() {
	app_indicator.Run(onReady, func() {})
}

func onReady() {
	//set menu header and Quicks
	indicator := app_indicator.GetIndicator()
	indicator.SetLabel("Liqo")
	indicator.SetMenuTitle("Liqo Agent")
	indicator.AddQuick("QUIT", "Q_QUIT", func(args ...interface{}) {
		app_indicator.Quit()
	})
	indicator.AddQuick("HOME", "Q_HOME", func(args ...interface{}) {
		indicator.DeselectAction()
	})
	indicator.AddSeparator()

	//insert Actions

	AdvClient, err := agent.CreateClient(agent.AcquireConfig())
	if err != nil {
		indicator.Notify("Liqo", "Agent could not connect to the cluster", "")
		return
	}
	advAction := indicator.AddAction("Show Advertisements", AShowAdv, nil)
	advAction.Connect(func(args ...interface{}) {
		actionShowAdv(args[0].(*client.Client))
	}, &AdvClient)

}

func actionShowAdv(c *client.Client) {
	liqo := app_indicator.GetIndicator()
	act, pres := liqo.Action(AShowAdv)
	if !pres {
		return
	}
	advList, err := agent.ListAdvertisements(c)
	if err != nil {
		liqo.Notify("Liqo", "Agent could not connect to the cluster", "")
		return
	} else {
		app_indicator.GetIndicator().SelectAction(AShowAdv)
		for _, adv := range advList {
			element := act.UseListChild()
			element.SetTitle(adv)
		}
	}
}

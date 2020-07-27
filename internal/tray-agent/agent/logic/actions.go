package logic

import (
	advtypes "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	"github.com/liqoTech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqoTech/liqo/internal/tray-agent/app-indicator"
)

// callback function for the ACTION "Show Advertisements". It shows the Advertisements CRs currently in the cluster,
// indicating whether they are 'ACCEPTED' or not.
func actionShowAdv() {
	i := app.GetIndicator()
	if ctrl := i.AgentCtrl(); ctrl != nil {
		advCache := ctrl.AdvCache()
		if ctrl.Connected() && advCache.Running {
			// start indicator ACTION
			act, pres := i.Action(aShowPeers)
			if !pres {
				return
			}
			i.SelectAction(aShowPeers)
			i.SetIcon(app.IconLiqoMain)
			// exec ACTION
			if !ctrl.Mocked() {
				for _, obj := range advCache.Store.List() {
					adv := obj.(*advtypes.Advertisement)
					element := act.UseListChild()
					element.SetTitle(client.DescribeAdvertisement(adv))
				}
			}
		} else {
			i.NotifyNoConnection()
		}
	} else {
		i.NotifyNoConnection()
	}

}

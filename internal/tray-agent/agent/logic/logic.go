package logic

import (
	"fmt"
	"github.com/gen2brain/dlgs"
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
			act, pres := i.Action(aShowAdv)
			if !pres {
				return
			}
			i.SelectAction(aShowAdv)
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

/*** SETTINGS ***/

// callback function for the ACTION "settings" that displays the settings submenu.
func actionSettings() {
	i := app.GetIndicator()
	if _, pres := i.Action(aSettings); !pres {
		return
	}
	i.SelectAction(aSettings)
}

// callback function for the OPTION "notifications" of "settings" action.
func optionChangeNotifyLevel() {
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

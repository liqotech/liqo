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
			// exec ACTION
			for _, obj := range advCache.Store.List() {
				adv := obj.(*advtypes.Advertisement)
				element := act.UseListChild()
				element.SetTitle(client.DescribeAdvertisement(adv))
			}
		} else {
			i.NotifyNoConnection()
		}
	}

}

/*** SETTINGS ***/

// callback function for the ACTION "settings" that displays the settings submenu.
func actionSettings() {
	i := app.GetIndicator()
	_, pres := i.Action(aSettings)
	if !pres {
		return
	}
	i.SelectAction(aSettings)
}

// callback function for the OPTION "notifications" of "settings" action.
func optionChangeNotifyLevel() {
	i := app.GetIndicator()
	str := make([]string, 0)
	nt := i.Config().NotifyTranslate()
	for _, v := range nt {
		str = append(str, v)
	}
	level, ok, _ := dlgs.List("NOTIFICATION SETTINGS", fmt.Sprintf("Choose how you would like to receive "+
		"notifications from Liqo.\n"+
		"CURRENT: %s", nt[i.Config().NotifyLevel()]), str)
	if ok {
		i.NotificationSetLevel(i.Config().NotifyTranslateReverse()[level])
	}
}

package app_indicator

import (
	"fmt"
	bip "github.com/gen2brain/beeep"
	"path/filepath"
)

//NotifyLevel is the level of the indicator notification system:
type NotifyLevel int

//NotifyIcon represents the Liqo icon set
type NotifyIcon int

const (
	//NotifyLevelOff: disable all notifications
	NotifyLevelOff NotifyLevel = iota
	//NotifyLevelMin: Indicator notifies events only using Indicator's icon and label
	NotifyLevelMin
	//NotifyLevelMax: Indicator notifies events using Indicator's icon and label and desktop banners
	NotifyLevelMax
)

// Liqo icon set for the Indicator notification system
const (
	NotifyIconNil NotifyIcon = iota
	NotifyIconNoConn
	NotifyIconDefault
	NotifyIconGreen
	NotifyIconGray
	NotifyIconOrange
)

//Notify manages Indicator notification logic. Depending on the current NotifyLevel of the Indicator,
//it switches the Indicator tray icon and displays a desktop banner, having title 'title, 'message' as body. If present
//in $LIQO_PATH, also 'notifyIcon' is shown inside the banner.
//
//The "nil" values can be used for both 'notifyIcon' and 'indicatorIcon':
//
//- "NotifyIconNil" : don't show a notification icon
//
//- "IconLiqoNil" : don't change current Indicator icon
func (i *Indicator) Notify(title string, message string, notifyIcon NotifyIcon, indicatorIcon Icon) {
	switch i.config.notifyLevel {
	case NotifyLevelOff:
		return
	case NotifyLevelMin:
		i.SetIcon(indicatorIcon)
	case NotifyLevelMax:
		i.SetIcon(indicatorIcon)
		var icoName string
		switch notifyIcon {
		case NotifyIconNil:
			icoName = ""
		case NotifyIconNoConn:
			icoName = "liqo-no_conn.png"
		case NotifyIconDefault:
			icoName = "liqo-black.png"
		case NotifyIconGreen:
			icoName = "liqo-green.png"
		case NotifyIconGray:
			icoName = "liqo-gray.png"
		case NotifyIconOrange:
			icoName = "liqo-orange.png"
		default:
			icoName = "liqo-black.png"
		}
		_ = bip.Notify(title, message, filepath.Join(i.config.notifyIconPath, icoName))
	default:
		return
	}
}

//NotificationSetLevel sets the level of the indicator notification system:
//
// NotifyLevelOff: disable all notifications
//
// NotifyLevelMin: Indicator notifies events only using Indicator's icon and label
//
// NotifyLevelMax: Indicator notifies events using Indicator's icon and label and desktop banners
func (i *Indicator) NotificationSetLevel(level NotifyLevel) {
	switch level {
	case NotifyLevelOff:
		i.config.notifyLevel = level
	case NotifyLevelMin:
		i.config.notifyLevel = level
	case NotifyLevelMax:
		i.config.notifyLevel = level
	default:
		return
	}
}

func (i *Indicator) NotifyNoConnection() {
	i.Notify("Liqo Agent: NO CONNECTION", "Agent could not connect to the desired cluster",
		NotifyIconNoConn, IconLiqoNoConn)
}

func (i *Indicator) NotifyNewAdv(name string) {
	i.Notify("Liqo Agent: NEW ADVERTISEMENT", fmt.Sprintf("You received a new advertisement %s", name),
		NotifyIconOrange, IconLiqoAdvNew)
}

func (i *Indicator) NotifyAcceptedAdv(name string) {
	i.Notify("Liqo Agent: ACCEPTED ADVERTISEMENT", fmt.Sprintf("advertisement %s has been accepted", name),
		NotifyIconGreen, IconLiqoAdvAccepted)
}

func (i *Indicator) NotifyRevokedAdv(name string) {
	i.Notify("Liqo Agent: REVOKED ADVERTISEMENT", fmt.Sprintf("advertisement %s revoked", name),
		NotifyIconOrange, IconLiqoAdvNew)
}

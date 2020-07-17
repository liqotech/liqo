package app_indicator

import (
	"fmt"
	bip "github.com/gen2brain/beeep"
	"github.com/gen2brain/dlgs"
	"github.com/ozgio/strutil"
	"path/filepath"
)

//NotifyLevel is the level of the indicator notification system:
type NotifyLevel int

//NotifyIcon represents the Liqo set of icons displayed in the desktop banners.
type NotifyIcon int

//Allowed modes for the Indicator notification system
const (
	//NotifyLevelOff: disable all notifications
	NotifyLevelOff NotifyLevel = iota
	//NotifyLevelMin: Indicator notifies events only using Indicator's icon and label
	NotifyLevelMin
	//NotifyLevelMax: Indicator notifies events using Indicator's icon and label and desktop banners
	NotifyLevelMax
	//notifyLevelUnknown: undefined level. User should not use it directly
	notifyLevelUnknown
)

// Textual descriptions of the NotifyLevel values
const (
	NotifyLevelOffDescription     = "Notifications OFF"
	NotifyLevelMinDescription     = "Notify with icon"
	NotifyLevelMaxDescription     = "Notify with icon and banner"
	notifyLevelUnknownDescription = "unknown"
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
//it changes the Indicator tray icon and displays a desktop banner, having title 'title' and 'message' as body.
//If present in $LIQO_PATH, also 'notifyIcon' is shown inside the banner.
//
//The "nil" values can be used for both 'notifyIcon' and 'indicatorIcon':
//
//	NotifyIconNil : don't show a notification icon
//
//	IconLiqoNil : don't change current Indicator icon
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
		if !i.gProvider.Mocked() {
			_ = bip.Notify(title, message, filepath.Join(i.config.notifyIconPath, icoName))
		}
	default:
		return
	}
}

//NotificationSetLevel sets the level of the indicator notification system:
//
//	NotifyLevelOff: disable all notifications
//
//	NotifyLevelMin: Indicator notifies events only using Indicator's icon and label
//
//	NotifyLevelMax: Indicator notifies events using Indicator's icon and label and desktop banners
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

//NotifyNoConnection is an already configured Notify() call to notify the absence of
//connection with the cluster pointed by $LIQO_KCONFIG.
func (i *Indicator) NotifyNoConnection() {
	i.Notify("Liqo Agent: NO CONNECTION", "Agent could not connect to the desired cluster",
		NotifyIconNoConn, IconLiqoNoConn)
}

//NotifyNewAdv is an already configured Notify() call to notify the creation of a
//new Advertisement CRD in the cluster.
func (i *Indicator) NotifyNewAdv(name string) {
	i.Notify("Liqo Agent: NEW ADVERTISEMENT", fmt.Sprintf("You received a new advertisement %s", name),
		NotifyIconOrange, IconLiqoAdvNew)
}

//NotifyAcceptedAdv is an already configured Notify() call to notify that an
//Advertisement CRD in the cluster has changed its status in "ACCEPTED".
func (i *Indicator) NotifyAcceptedAdv(name string) {
	i.Notify("Liqo Agent: ACCEPTED ADVERTISEMENT", fmt.Sprintf("advertisement %s has been accepted", name),
		NotifyIconGreen, IconLiqoAdvAccepted)
}

//NotifyRevokedAdv is an already configured Notify() call to notify that an Advertisement
//CRD in the cluster is not in "ACCEPTED" status anymore.
func (i *Indicator) NotifyRevokedAdv(name string) {
	i.Notify("Liqo Agent: REVOKED ADVERTISEMENT", fmt.Sprintf("advertisement %s revoked", name),
		NotifyIconOrange, IconLiqoAdvNew)
}

//NotifyDeletedAdv is an already configured Notify() call to notify that an Advertisement
//CRD in the cluster has been deleted.
func (i *Indicator) NotifyDeletedAdv(name string) {
	i.Notify("Liqo Agent: DELETED ADVERTISEMENT", fmt.Sprintf("advertisement %s deleted", name),
		NotifyIconOrange, IconLiqoAdvNew)
}

//ShowWarning displays a Warning window box.
func (i *Indicator) ShowWarning(title, message string) {
	if !GetGuiProvider().Mocked() {
		dlgs.Warning(title, fmt.Sprintln(strutil.CenterText("", menuWidth*2), message))
	}
}

//ShowWarningForbiddenTethered is an already configured ShowWarning() call to warn users
//when they attempt to set the TETHERED mode without the required conditions.
func (i *Indicator) ShowWarningForbiddenTethered() {
	i.ShowWarning("LIQO AGENT: mode change not allowed", "TETHERED mode is only available"+
		"with 1 active peering, offering resources.\n\nPlease disconnect from other peerings and retry.")
}

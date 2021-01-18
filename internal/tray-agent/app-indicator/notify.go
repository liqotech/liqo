package app_indicator

import (
	"fmt"
	"github.com/agrison/go-commons-lang/stringUtils"
	bip "github.com/gen2brain/beeep"
	"github.com/gen2brain/dlgs"
	"github.com/ozgio/strutil"
	"path/filepath"
	"strconv"
	"strings"
)

//NotifyLevel is the level of the indicator notification system:
type NotifyLevel int

//NotifyIcon represents the Liqo set of icons displayed in the desktop banners.
type NotifyIcon int

//NotifyPeeringEvent defines a type of event regarding a peering with a foreign cluster.
type NotifyPeeringEvent int

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

//Liqo icon set for the Indicator notification system.
const (
	NotifyIconNil NotifyIcon = iota
	NotifyIconDefault
	NotifyIconWhite
	NotifyIconError
	NotifyIconWarning
)

const (
	//NotifyEventPeeringOn defines the event of a peering that has been established.
	NotifyEventPeeringOn NotifyPeeringEvent = iota
	//NotifyEventPeeringOff defines the event of a peering that has been torn down.
	NotifyEventPeeringOff
)

//Notify manages Indicator notification logic. Depending on the current NotifyLevel of the Indicator,
//it changes the Indicator tray icon and displays a desktop banner, having title 'title' and 'message' as body.
//If present in client.EnvLiqoPath, also 'notifyIcon' is shown inside the banner.
//
//The "nil" values can be used for both 'notifyIcon' and 'indicatorIcon':
//
//	NotifyIconNil : don't show a notification icon
//
//	IconLiqoNil : don't change current Indicator icon
func (i *Indicator) Notify(title string, message string, notifyIcon NotifyIcon, indicatorIcon Icon) {
	gr := i.graphicResource[resourceDesktop]
	gr.Lock()
	defer gr.Unlock()
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
		case NotifyIconDefault:
			icoName = "liqo-main-black.png"
		case NotifyIconWarning:
			icoName = "liqo-warning.png"
		case NotifyIconWhite:
			icoName = "liqo-main-white.png"
		case NotifyIconError:
			icoName = "liqo-error.png"
		default:
			icoName = "liqo-main-black.png"
		}
		if !i.gProvider.Mocked() {
			/*The golang guidelines suggests error messages should not start with a capitalized letter.
			Therefore, since Notify sometimes receives an error as 'message', the Capitalize() function
			overcomes this problem, correctly displaying the string to the user.*/
			_ = bip.Notify(title, stringUtils.Capitalize(message), filepath.Join(i.config.notifyIconPath, icoName))
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
		NotifyIconWarning, IconLiqoNoConn)
}

//NotifyPeering is a semi-configured Notify() call to notify events related to peerings involving a specific peer.
func (i *Indicator) NotifyPeering(direction PeeringType, event NotifyPeeringEvent, peer *PeerInfo) {
	var (
		header      []string
		body        []string
		peerName    string
		desktopIcon NotifyIcon
		trayIcon    Icon
	)
	peer.RLock()
	defer peer.RUnlock()
	if peer.Unknown {
		peerName = strings.Join([]string{"UNKNOWN", strconv.Itoa(peer.UnknownId)}, " ")
	} else {
		peerName = peer.ClusterName
	}
	switch event {
	case NotifyEventPeeringOn:
		header = append(header, "NEW")
		if direction == PeeringOutgoing {
			header = append(header, "OUTGOING PEERING ESTABLISHED")
			body = append(body, peerName, "is now sharing its resources")
		} else {
			header = append(header, "PEERING ACCEPTED")
			body = append(body, "You are now sharing resources to", peerName)
		}
		desktopIcon = NotifyIconDefault
		trayIcon = IconLiqoPurple
	case NotifyEventPeeringOff:
		if direction == PeeringOutgoing {
			header = append(header, "OUTGOING")
			body = append(body, peerName, "resources are no more available")
		} else {
			header = append(header, "INCOMING")
			body = append(body, "You stopped sharing resources to", peerName)
		}
		header = append(header, "PEERING CLOSED")
		desktopIcon = NotifyIconDefault
		trayIcon = IconLiqoPurple
		//expand for additional events
	}
	i.Notify(strings.Join(header, " "), strings.Join(body, " "), desktopIcon, trayIcon)
}

//ShowWarning displays a Warning window box.
func (i *Indicator) ShowWarning(title, message string) {
	gr := i.graphicResource[resourceDesktop]
	gr.Lock()
	defer gr.Unlock()
	if !GetGuiProvider().Mocked() {
		_, _ = dlgs.Warning(title, fmt.Sprintln(strutil.CenterText("", menuWidth*2), message))
	}
}

//ShowWarningForbiddenTethered is an already configured ShowWarning() call to warn users
//when they attempt to set the TETHERED mode without the required conditions.
func (i *Indicator) ShowWarningForbiddenTethered() {
	i.ShowWarning("LIQO AGENT: mode change not allowed", "TETHERED mode is only available"+
		"with 1 active peering, offering resources.\n\nPlease disconnect from other peerings and retry.")
}

//ShowError displays an Error window box.
func (i *Indicator) ShowError(title, message string) {
	gr := i.graphicResource[resourceDesktop]
	gr.Lock()
	defer gr.Unlock()
	if !GetGuiProvider().Mocked() {
		_, _ = dlgs.Error(title, fmt.Sprintln(strutil.CenterText("", menuWidth*2), message))
	}
}

//ShowErrorNoConnection is an already configured ShowError() call to warn
//the user about kubeconfig misconfiguration.
func (i *Indicator) ShowErrorNoConnection() {
	if !GetGuiProvider().Mocked() {
		_, _ = dlgs.Error("LIQO AGENT", fmt.Sprintln(strutil.CenterText("", menuWidth*2),
			"Liqo Agent could not find a valid kubeconfig file.\n",
			"Please restart the Agent after providing a correct configuration."))
	}
}

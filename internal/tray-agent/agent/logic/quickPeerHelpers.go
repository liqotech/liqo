package logic

import (
	app "github.com/liqotech/liqo/internal/tray-agent/app-indicator"
	"strconv"
	"strings"
)

/*This file contains internal variables and helper functions for the QUICK qPeers in charge of displaying
discovered peers.*/

// set of frequently used tags inside application logic
const (
	tagStatus  = "status"
	titlePeers = "PEERS"
)

//refreshPeerCount updates the visual counter of the discovered peers (even not peered).
//Currently the number if retrieved directly from the MenuNode. In a future update the Indicator.Status component
//will provide it.
func refreshPeerCount(quick *app.MenuNode) {
	peerCount := quick.ListChildrenLen()
	str := strings.Join([]string{"(", strconv.Itoa(peerCount), ")"}, "")
	quick.SetTitle(strings.Join([]string{titlePeers, str}, " "))
	//the menu entry is disabled when the counter reaches 0 to avoid useless clicks
	if peerCount > 0 {
		quick.SetIsEnabled(true)
	} else {
		quick.SetIsEnabled(false)
	}

}

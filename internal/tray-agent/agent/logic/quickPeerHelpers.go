package logic

import (
	"fmt"
	"github.com/liqotech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqotech/liqo/internal/tray-agent/app-indicator"
	discovery2 "github.com/liqotech/liqo/pkg/discovery"
	"strconv"
	"strings"
	"sync"
)

/*This file contains internal variables and helper functions for the QUICK qPeers in charge of displaying
discovered peers.*/

// set of frequently used tags for menu entries regarding peers management
const (
	tagStatus          = "status"
	tagPeerAuthToken   = "auth"
	tagPeeringIncoming = "inPeering"
	tagPeeringOutgoing = "outPeering"
	tagPeeringCmd      = "cmd"
)

// set of frequently used title strings for menu entries regarding peers management
const (
	titlePeers           = "PEERS"
	titlePeerAuthToken   = "Insert auth token manually "
	titlePeeringOutgoing = "OUTGOING PEERING"
	titlePeeringIncoming = "INCOMING PEERING"
	titlePeeringCmdStart = "Request peering"
	titlePeeringCmdStop  = "Stop peering"
)

// set of frequently used text strings for menu entries regarding peers management
const (
	//labelPeerUntrusted is the label used to describe a peer whose certificate for the authn endpoint is trusted
	//(e.g. self-signed)
	labelPeerUntrusted = "NO"
	//labelPeerLAN is the label used in the peers list to indicate a peer has been discovered inside user's LAN.
	labelPeerLAN = "[LAN]"
	//labelPeerUnknown is the replacement label used in the peers list when any ClusterName is provided for a peer.
	labelPeerUnknown = "UNKNOWN"
	//labelPeerTrusted is the label used to describe a peer whose certificate for the authn endpoint can be trusted
	labelPeerTrusted = "YES"
	//labelAuthTokenAccepted is the label used when the Authn Token to perform Peering towards a peer has been accepted.
	labelAuthTokenAccepted = "ACCEPTED"
	//labelAuthTokenRefused is the label used when the Authn Token to perform Peering towards a peer has been refused
	//(wrong or empty token).
	labelAuthTokenRefused = "REFUSED"
	//labelAuthTokenPending is the label used when the validation process of the Authn Token towards a peer is still pending.
	labelAuthTokenPending = "PENDING"
	//labelResourceQuotaUnavailable is a placeholder text for a shared resource quota.
	labelResourceQuotaUnavailable = "unavailable"
)

//refreshPeerCount updates the visual counter of the discovered peers (even not peered).
//Currently the number if retrieved directly from the MenuNode. In a future update the Indicator.Status component
//will provide it.
func refreshPeerCount(quick *app.MenuNode) {
	peerCount := app.GetIndicator().Status().Peers()
	str := strings.Join([]string{"(", strconv.Itoa(peerCount), ")"}, "")
	quick.SetTitle(strings.Join([]string{titlePeers, str}, " "))
	//the menu entry is disabled when the counter reaches 0 to avoid useless clicks
	if peerCount > 0 {
		quick.SetIsEnabled(true)
	} else {
		quick.SetIsEnabled(false)
	}
}

/*createPeerNode creates an entry in the tray menu peers list for a newly discovered peer.
Each peer entry has the following structure:
	- 	peer name
	1-		STATUS: peer information
	2-		AUTHN TOKEN MANUAL INSERTION: button to enable manual insertion of the auth token for the foreign cluster if the
			AuthN request has been refused
	3-		OUTGOING PEERING: display information and commands for an outgoing peering towards this peer
	3.1-	START/STOP peering
	3.2-	PEERING STATUS: details on the active peering (e.g. consumed resources)
	4-		INCOMING PEERING: display information and commands for an incoming peering from this peer
	4.1-	STOP PEERING
*/
func createPeerNode(peerList *app.MenuNode, data *client.NotifyDataForeignCluster) *app.MenuNode {
	//create the structure for a single peer
	peerNode := peerList.UseListChild("", data.ClusterID)
	//1- STATUS
	statusNode := peerNode.UseListChild("", tagStatus)
	statusNode.SetIsEnabled(false)
	//2- AUTHN TOKEN MANUAL INSERTION
	insertAuthToken := peerNode.UseListChild(titlePeerAuthToken, tagPeerAuthToken)
	insertAuthToken.SetIsEnabled(false)
	//todo further connection of the "insert auth token" entry with a callback
	//3- OUTGOING PEERING
	outgoingNode := peerNode.UseListChild(titlePeeringOutgoing, tagPeeringOutgoing)
	//3.1- START/STOP PEERING
	outgoingNode.UseListChild(titlePeeringCmdStart, tagPeeringCmd)
	//todo further connection of the "start/stop peering" entry with a callback
	//3.2- STATUS
	outgoingStatus := outgoingNode.UseListChild("", tagStatus)
	outgoingStatus.SetIsVisible(false)
	outgoingStatus.SetIsEnabled(false)
	//4- INCOMING PEERING
	incomingNode := peerNode.UseListChild(titlePeeringIncoming, tagPeeringIncoming)
	//4.1- STOP PEERING
	incomingCmd := incomingNode.UseListChild(titlePeeringCmdStop, tagPeeringCmd)
	//todo further connection of the "stop peering" entry with a callback
	//the "stop peering" entry is by default disabled since its callback can be executed only in presence
	//of an active incoming peering
	incomingCmd.SetIsEnabled(false)
	return peerNode
}

//refreshPeerName refreshes the content of the main menu entry of a peer, displaying:
//
//1- The ClusterName of the correspondent ForeignCluster (or a text replacement labelPeerUnknown indicating its name is unknown
//
//2- A tag labelPeerLAN indicating whether the peer is located in the same LAN of the home cluster
func refreshPeerName(peerNode *app.MenuNode, peer *app.PeerInfo, data *client.NotifyDataForeignCluster, wg *sync.WaitGroup) {
	defer wg.Done()
	var title []string
	//- check unknown identity
	if peer.Unknown {
		title = append(title, labelPeerUnknown, strconv.Itoa(peer.UnknownId))
	} else {
		title = append(title, data.ClusterName)
	}
	//check if the cluster is located inside the LAN
	if data.LocalDiscovered {
		title = append(title, labelPeerLAN)
	}
	peerNode.SetTitle(strings.Join(title, " "))
}

//refreshPeerStatus refreshes the content of the peer status entry.
func refreshPeerStatus(peerNode *app.MenuNode, data *client.NotifyDataForeignCluster, wg *sync.WaitGroup) {
	defer wg.Done()
	statusNode, present := peerNode.ListChild(tagStatus)
	content := strings.Builder{}
	if present {
		//a) ClusterID
		content.WriteString(fmt.Sprintf("%s\n", data.ClusterID))
		//b) TrustMode: identify whether the foreign cluster has a trusted signed certificate
		var trustMode string
		switch data.Trusted {
		case discovery2.TrustModeTrusted:
			trustMode = labelPeerTrusted
		case discovery2.TrustModeUntrusted:
			trustMode = labelPeerUntrusted
		default:
			trustMode = labelPeerUnknown
		}
		//c) Status of the Authentication process of the Home cluster on the Foreign cluster.
		content.WriteString(fmt.Sprintf("Trusted: %s\n", trustMode))
		var authStat string
		switch data.AuthStatus {
		case discovery2.AuthStatusAccepted:
			authStat = labelAuthTokenAccepted
		case discovery2.AuthStatusEmptyRefused:
			authStat = labelAuthTokenRefused
		case discovery2.AuthStatusRefused:
			authStat = labelAuthTokenRefused
		default:
			authStat = labelAuthTokenPending
		}
		content.WriteString(fmt.Sprintf("Auth token: %s", authStat))
	}
	statusNode.SetTitle(content.String())
}

//refreshPeerInfo reloads into the tray menu details on identity and status of a specific peer.
func refreshPeerInfo(peerNode *app.MenuNode, peer *app.PeerInfo, data *client.NotifyDataForeignCluster, wg *sync.WaitGroup) {
	peerWg := &sync.WaitGroup{}
	peerWg.Add(2)
	defer wg.Done()
	//set content of the NAME entry
	go refreshPeerName(peerNode, peer, data, peerWg)
	//set content of the STATUS entry
	go refreshPeerStatus(peerNode, data, peerWg)
	peerWg.Wait()
}

//refreshPeerInfo reloads into the tray menu details on peering status of a specific peer.
func refreshPeeringInfo(peerNode *app.MenuNode, peer *app.PeerInfo, data *client.NotifyDataForeignCluster, wg *sync.WaitGroup) {
	defer wg.Done()
	//outgoing peering
	outgoingEntry, outPresent := peerNode.ListChild(tagPeeringOutgoing)
	if outPresent {
		cmdNode, ok1 := outgoingEntry.ListChild(tagPeeringCmd)
		statusNode, ok2 := outgoingEntry.ListChild(tagStatus)
		if peer.OutPeeringConnected {
			outgoingEntry.SetIsChecked(true)
			if ok1 {
				//handle start/stop peering button
				cmdNode.SetTitle(titlePeeringCmdStop)
			}
			if ok2 {
				//show shared resources in active peering
				statusNode.SetTitle(describeOutResources(data))
				statusNode.SetIsVisible(true)
			}
		} else {
			outgoingEntry.SetIsChecked(false)
			if ok1 {
				//handle start/stop peering button
				cmdNode.SetTitle(titlePeeringCmdStart)
			}
			if ok2 {
				//no resource is being shared
				statusNode.SetIsVisible(false)
				statusNode.SetTitle("")
			}
		}
	}
	//incoming peering
	incomingEntry, inPresent := peerNode.ListChild(tagPeeringIncoming)
	if inPresent {
		cmdNode, present := incomingEntry.ListChild(tagPeeringCmd)
		if peer.InPeeringConnected {
			incomingEntry.SetIsChecked(true)
			if present {
				cmdNode.SetIsEnabled(true)
			}
		} else {
			incomingEntry.SetIsChecked(false)
			if present {
				cmdNode.SetIsEnabled(false)
			}
		}
	}
}

//describeOutResources returns the formatted content of an outgoing peering status,
//describing the amount of shared resources.
func describeOutResources(data *client.NotifyDataForeignCluster) string {
	content := strings.Builder{}
	content.WriteString("CPU: ")
	if data.OutPeering.CpuQuota != "" {
		content.WriteString(data.OutPeering.CpuQuota)
	} else {
		content.WriteString(labelResourceQuotaUnavailable)
	}
	content.WriteString("\nRAM: ")
	if data.OutPeering.MemQuota != "" {
		content.WriteString(data.OutPeering.MemQuota)
	} else {
		content.WriteString(labelResourceQuotaUnavailable)
	}
	return content.String()
}

package logic

import (
	"github.com/liqotech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqotech/liqo/internal/tray-agent/app-indicator"
	"github.com/liqotech/liqo/internal/tray-agent/test"
	"github.com/stretchr/testify/assert"
	"testing"
)

//test the routines OnReady that is called in the app-indicator/Run() loop and manages the Liqo Agent logic.
func TestOnReady(t *testing.T) {
	app.UseMockedGuiProvider()
	client.UseMockedAgentController()
	app.DestroyMockedIndicator()
	client.DestroyMockedAgentController()
	app.DestroyStatus()
	OnReady()
	i := app.GetIndicator()
	//test startup Icon
	startIcon := i.Icon()
	assert.Equal(t, app.IconLiqoMain, startIcon, "startup Indicator icon is not IconLiqoMain")
	//test ACTIONs and QUICKs registrations
	var exist bool
	_, exist = i.Quick(qOnOff)
	assert.Truef(t, exist, "QUICK %s not registered", qOnOff)
	_, exist = i.Quick(qMode)
	assert.Truef(t, exist, "QUICK %s not registered", qMode)
	_, exist = i.Quick(qWeb)
	assert.Truef(t, exist, "QUICK %s not registered", qWeb)
	_, exist = i.Quick(qQuit)
	assert.Truef(t, exist, "QUICK %s not registered", qQuit)
	_, exist = i.Quick(qDash)
	assert.Truef(t, exist, "QUICK %s not registered", qDash)
	_, exist = i.Quick(qNotify)
	assert.Truef(t, exist, "QUICK %s not registered", qNotify)
	_, exist = i.Quick(qPeers)
	assert.Truef(t, exist, "QUICK %s not registered", qPeers)

	// test Listeners registrations
	_, exist = i.Listener(client.ChanAdvNew)
	assert.True(t, exist, "Listener for NotifyChanType ChanAdvNew not registered")
	_, exist = i.Listener(client.ChanAdvAccepted)
	assert.True(t, exist, "Listener for NotifyChanType ChanAdvAccepted not registered")
	_, exist = i.Listener(client.ChanAdvRevoked)
	assert.True(t, exist, "Listener for NotifyChanType ChanAdvRevoked not registered")
	_, exist = i.Listener(client.ChanAdvDeleted)
	assert.True(t, exist, "Listener for NotifyChanType ChanAdvDeleted not registered")
	i.Quit()
}

//test notification system for the Advertisements-related events, monitoring icon changes
func TestAdvertisementNotify(t *testing.T) {
	app.UseMockedGuiProvider()
	client.UseMockedAgentController()
	app.DestroyMockedIndicator()
	client.DestroyMockedAgentController()
	eventTester := app.GetGuiProvider().NewEventTester()
	eventTester.Test()
	i := app.GetIndicator()
	startListenerAdvertisements(i)
	assert.Equal(t, app.IconLiqoMain, i.Icon(), "startup Indicator icon is not IconLiqoMain")
	ctrl := i.AgentCtrl()
	if err := ctrl.StartCaches(); err != nil {
		t.Fatal("caches not started")
	}
	testAdvName := "test"
	//
	eventTester.Add(1)
	ctrl.NotifyChannel(client.ChanAdvNew) <- testAdvName
	eventTester.Wait()
	assert.Equal(t, app.IconLiqoOrange, i.Icon(), "Icon not correctly set on New Advertisement")
	i.SetIcon(app.IconLiqoMain)
	//
	eventTester.Add(1)
	ctrl.NotifyChannel(client.ChanAdvAccepted) <- testAdvName
	eventTester.Wait()
	assert.Equal(t, app.IconLiqoGreen, i.Icon(), "Icon not correctly set on Accepted Advertisement")
	i.SetIcon(app.IconLiqoMain)
	//
	eventTester.Add(1)
	ctrl.NotifyChannel(client.ChanAdvRevoked) <- testAdvName
	eventTester.Wait()
	assert.Equal(t, app.IconLiqoOrange, i.Icon(), "Icon not correctly set on Revoked Advertisement")
	i.SetIcon(app.IconLiqoMain)
	//
	eventTester.Add(1)
	ctrl.NotifyChannel(client.ChanAdvDeleted) <- testAdvName
	eventTester.Wait()
	assert.Equal(t, app.IconLiqoOrange, i.Icon(), "Icon not correctly set on Deleted Advertisement")
	i.SetIcon(app.IconLiqoMain)
	i.Quit()
}

func TestPeersListeners(t *testing.T) {
	app.UseMockedGuiProvider()
	client.UseMockedAgentController()
	app.DestroyMockedIndicator()
	client.DestroyMockedAgentController()
	app.DestroyStatus()
	eventTester := app.GetGuiProvider().NewEventTester()
	eventTester.Test()
	OnReady()
	i := app.GetIndicator()
	//test peers list when no ForeignCluster is present
	quickNode, present := i.Quick(qPeers)
	if !present {
		t.Fatal("Show Peers QUICK not registered")
	}
	assert.Equal(t, 0, quickNode.ListChildrenLen(), "peers list is not empty when 0 ForeignCluster(s) exist [init phase]")
	assert.False(t, quickNode.IsEnabled(), "peers menu entry should be disabled when 0 ForeignCluster(s) exist [init phase]")

	//test addition of ForeignClusters
	clusterID1 := "cl1"
	clusterName1 := "test1"
	clusterName2 := "test2"
	fc1 := test.CreateForeignCluster(clusterID1, clusterName1)
	fcCtrl := i.AgentCtrl().Controller(client.CRForeignCluster)
	//peerAddChan := i.AgentCtrl().NotifyChannel(client.ChanPeerAdded)
	//peerUpdateChan := i.AgentCtrl().NotifyChannel(client.ChanPeerUpdated)
	//peerDeleteChan := i.AgentCtrl().NotifyChannel(client.ChanPeerDeleted)
	eventTester.Add(1)
	err := fcCtrl.Store.Add(fc1)
	eventTester.Wait()
	//time.Sleep(time.Second * 3)
	//peerAddChan <- clusterID1
	assert.NoError(t, err, "ForeignCluster addition failed")

	assert.True(t, quickNode.IsEnabled(), "peers menu entry should be enabled when 1+ ForeignCluster(s) exist [init phase]")
	assert.Equal(t, 1, quickNode.ListChildrenLen(), "peers list is empty when 1+ ForeignCluster(s) exist")

	//check if inserted element is present
	var fc1Node *app.MenuNode
	fc1Node, present = quickNode.ListChild(clusterID1)
	assert.Truef(t, present, "LIST MenuNode for ForeignCluster %v not present", clusterID1)
	assert.Equal(t, clusterName1, fc1Node.Title(), "peers menu entry displays wrong content")
	fc2 := fc1.DeepCopy()
	fc2.Spec.ClusterIdentity.ClusterName = clusterName2
	eventTester.Add(1)
	err = fcCtrl.Store.Update(fc2)
	eventTester.Wait()
	//time.Sleep(time.Second * 3)
	assert.NoError(t, err, "ForeignCluster update failed")
	//peerUpdateChan <- clusterID1

	//check update of the text content
	fc1Node, present = quickNode.ListChild(clusterID1)
	assert.Truef(t, present, "LIST MenuNode for ForeignCluster %v not present", clusterID1)
	assert.Equal(t, clusterName2, fc1Node.Title(), "peers menu entry displays wrong content after update")

	//delete element
	eventTester.Add(1)
	err = fcCtrl.Store.Delete(fc2)
	eventTester.Wait()
	//time.Sleep(time.Second * 3)
	assert.NoError(t, err, "ForeignCluster deletion failed")
	//peerDeleteChan <- clusterID1

	//check peers list back at initial condition
	endCount := quickNode.ListChildrenLen()
	assert.Equal(t, 0, endCount, "peers list is not empty when 0 ForeignCluster(s) exist [init phase]")
	assert.False(t, quickNode.IsEnabled(), "peers menu entry should be disabled when 0 ForeignCluster(s) exist")
	i.Quit()
}

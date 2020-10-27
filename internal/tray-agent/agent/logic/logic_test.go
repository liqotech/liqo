package logic

import (
	"github.com/liqotech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqotech/liqo/internal/tray-agent/app-indicator"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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
	//
	_, exist = i.Action(aShowPeers)
	assert.Truef(t, exist, "ACTION %s not registered", aShowPeers)

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
	i := app.GetIndicator()
	startListenerAdvertisements(i)
	assert.Equal(t, app.IconLiqoMain, i.Icon(), "startup Indicator icon is not IconLiqoMain")
	ctrl := i.AgentCtrl()
	if err := ctrl.StartCaches(); err != nil {
		t.Fatal("caches not started")
	}
	testAdvName := "test"
	//
	ctrl.NotifyChannel(client.ChanAdvNew) <- testAdvName
	time.Sleep(time.Second * 4)
	assert.Equal(t, app.IconLiqoOrange, i.Icon(), "Icon not correctly set on New Advertisement")
	i.SetIcon(app.IconLiqoMain)
	//
	ctrl.NotifyChannel(client.ChanAdvAccepted) <- testAdvName
	time.Sleep(time.Second * 4)
	assert.Equal(t, app.IconLiqoGreen, i.Icon(), "Icon not correctly set on Accepted Advertisement")
	i.SetIcon(app.IconLiqoMain)
	//
	ctrl.NotifyChannel(client.ChanAdvRevoked) <- testAdvName
	time.Sleep(time.Second * 4)
	assert.Equal(t, app.IconLiqoOrange, i.Icon(), "Icon not correctly set on Revoked Advertisement")
	i.SetIcon(app.IconLiqoMain)
	//
	ctrl.NotifyChannel(client.ChanAdvDeleted) <- testAdvName
	time.Sleep(time.Second * 4)
	assert.Equal(t, app.IconLiqoOrange, i.Icon(), "Icon not correctly set on Deleted Advertisement")
	i.SetIcon(app.IconLiqoMain)
	i.Quit()
}

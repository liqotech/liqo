package logic

import (
	"github.com/liqoTech/liqo/internal/tray-agent/agent/client"
	app "github.com/liqoTech/liqo/internal/tray-agent/app-indicator"
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
	OldReady()
	i := app.GetIndicator()
	//test startup Icon
	startIcon := i.Icon()
	assert.Equal(t, app.IconLiqoMain, startIcon, "startup Indicator icon is not IconLiqoMain")
	//test ACTIONs and QUICKs registrations
	var exist bool
	_, exist = i.Quick(qHome)
	assert.Truef(t, exist, "QUICK %s not registered", qHome)
	_, exist = i.Quick(qQuit)
	assert.Truef(t, exist, "QUICK %s not registered", qQuit)
	_, exist = i.Quick(qDash)
	assert.Truef(t, exist, "QUICK %s not registered", qDash)
	//
	_, exist = i.Action(aShowPeers)
	assert.Truef(t, exist, "ACTION %s not registered", aShowPeers)
	_, exist = i.Action(aSettings)
	assert.Truef(t, exist, "ACTION %s not registered", aSettings)
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
	i.AgentCtrl().StartCaches()
	advChannels := i.AgentCtrl().AdvCache().NotifyChannels
	testAdvName := "test"
	//
	advChannels[client.ChanAdvNew] <- testAdvName
	time.Sleep(time.Second * 4)
	assert.Equal(t, app.IconLiqoAdvNew, i.Icon(), "Icon not correctly set on New Advertisement")
	i.SetIcon(app.IconLiqoMain)
	//
	advChannels[client.ChanAdvAccepted] <- testAdvName
	time.Sleep(time.Second * 4)
	assert.Equal(t, app.IconLiqoAdvAccepted, i.Icon(), "Icon not correctly set on Accepted Advertisement")
	i.SetIcon(app.IconLiqoMain)
	//
	advChannels[client.ChanAdvRevoked] <- testAdvName
	time.Sleep(time.Second * 4)
	assert.Equal(t, app.IconLiqoAdvNew, i.Icon(), "Icon not correctly set on Revoked Advertisement")
	i.SetIcon(app.IconLiqoMain)
	//
	advChannels[client.ChanAdvDeleted] <- testAdvName
	time.Sleep(time.Second * 4)
	assert.Equal(t, app.IconLiqoAdvNew, i.Icon(), "Icon not correctly set on Deleted Advertisement")
	i.SetIcon(app.IconLiqoMain)
	i.Quit()
}

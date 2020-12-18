package app_indicator

import (
	"github.com/liqotech/liqo/internal/tray-agent/agent/client"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStatus(t *testing.T) {
	UseMockedGuiProvider()
	client.UseMockedAgentController()
	DestroyMockedIndicator()
	client.DestroyMockedAgentController()
	stat := GetStatus()
	//test default configuration
	assert.Equal(t, StatRunOff, stat.Running(),
		"default running state should be OFF")
	assert.Equal(t, StatModeAutonomous, stat.Mode(),
		"default working mode should be AUTONOMOUS")
	assert.Equal(t, 0, stat.Peerings(PeeringOutgoing))
	assert.Equal(t, 0, stat.Peerings(PeeringIncoming))
	//test status operations
	stat.SetRunning(StatRunOn)
	assert.Equal(t, StatRunOn, stat.Running(), "running status should be ON")
	assert.Equal(t, 0, stat.Peerings(PeeringOutgoing))
	assert.Equal(t, 0, stat.Peerings(PeeringIncoming))
	//addition of consuming and offering peerings are allowed while
	//in AUTONOMOUS mode.
	stat.IncDecPeerings(PeeringOutgoing, true)
	stat.IncDecPeerings(PeeringIncoming, true)
	assert.Equal(t, 1, stat.Peerings(PeeringOutgoing), "addition of consuming peer in"+
		"AUTONOMOUS mode is not allowed")
	assert.Equal(t, 1, stat.Peerings(PeeringIncoming), "addition of offering peer in"+
		"AUTONOMOUS mode is not allowed")
	//test transition to TETHERED mode
	assert.Errorf(t, stat.SetMode(StatModeTethered), "TETHERED mode should not be allowed with active consuming peerings.")
	stat.IncDecPeerings(PeeringOutgoing, false)
	//without active consuming peerings, TETHERED mode is allowed
	_ = stat.SetMode(StatModeTethered)
	assert.Equal(t, StatModeTethered, stat.Mode(), "transition to TETHERED should be"+
		"allowed with matching requirements")
	assert.True(t, stat.IsTetheredCompliant(), "TETHERED mode requirements not matching")
	//turning the system off should shut down all active peerings
	stat.SetRunning(StatRunOff)
	assert.Equal(t, 0, stat.Peerings(PeeringOutgoing), "there should be no active consuming peerings"+
		"after turning off the system")
	assert.Equal(t, 0, stat.Peerings(PeeringIncoming), "there should be no active offering peerings"+
		"after turning off the system")
}

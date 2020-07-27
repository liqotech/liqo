package app_indicator

import (
	"github.com/liqoTech/liqo/internal/tray-agent/agent/client"
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
	assert.Equal(t, 0, stat.ConsumePeerings())
	assert.Equal(t, 0, stat.OfferPeerings())
	//test status operations
	stat.SetRunning(StatRunOn)
	assert.Equal(t, StatRunOn, stat.Running(), "running status should be ON")
	assert.Equal(t, 0, stat.ConsumePeerings())
	assert.Equal(t, 0, stat.OfferPeerings())
	//addition of consuming and offering peerings are allowed while
	//in AUTONOMOUS mode.
	stat.IncConsumePeerings()
	stat.IncOfferPeerings()
	assert.Equal(t, 1, stat.ConsumePeerings(), "addition of consuming peer in"+
		"AUTONOMOUS mode is not allowed")
	assert.Equal(t, 1, stat.OfferPeerings(), "addition of offering peer in"+
		"AUTONOMOUS mode is not allowed")
	//test transition to TETHERED mode
	assert.Errorf(t, stat.SetMode(StatModeTethered), "TETHERED mode should not be allowed with active consuming peerings.")
	stat.DecConsumePeerings()
	//without active consuming peerings, TETHERED mode is allowed
	_ = stat.SetMode(StatModeTethered)
	assert.Equal(t, StatModeTethered, stat.Mode(), "transition to TETHERED should be"+
		"allowed with matching requirements")
	assert.True(t, stat.IsTetheredCompliant(), "TETHERED mode requirements not matching")
	//turning the system off should shut down all active peerings
	stat.SetRunning(StatRunOff)
	assert.Equal(t, 0, stat.ConsumePeerings(), "there should be no active consuming peerings"+
		"after turning off the system")
	assert.Equal(t, 0, stat.OfferPeerings(), "there should be no active offering peerings"+
		"after turning off the system")
}

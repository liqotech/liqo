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
}

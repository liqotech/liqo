package client

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAgentControllerComponentsReadiness(t *testing.T) {
	UseMockedAgentController()
	DestroyMockedAgentController()
	// test AgentController components
	ctrl := GetAgentController()
	assert.True(t, ctrl.Connected(), "AgentController is not connected")
	for _, crName := range customResources {
		crdCtrl := ctrl.Controller(crName)
		assert.NotNilf(t, crdCtrl, "%v CRDController is nil", crName)
		assert.Truef(t, crdCtrl.Running(), "%v CRDController is not running", crName)
	}
}

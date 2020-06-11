package main

import (
	agent_logic "github.com/liqoTech/liqo/internal/tray-agent/agent-logic"
	"github.com/liqoTech/liqo/internal/tray-agent/app-indicator"
)

func main() {
	app_indicator.Run(agent_logic.OnReady, agent_logic.OnExit)
}

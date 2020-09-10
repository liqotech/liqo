package main

import (
	"github.com/liqotech/liqo/internal/tray-agent/agent/logic"
	"github.com/liqotech/liqo/internal/tray-agent/app-indicator"
)

func main() {
	app_indicator.Run(logic.OnReady, logic.OnExit)
}

package monitoring

type ProcessMonitoring interface {
	init()
	Start()
	Complete(component LiqoComponent)
	EventRegister(component LiqoComponent, event EventType, status EventStatus)
}

package monitoring

type ProcessMonitoring interface {
	init(bool) error
	Start()
	StartComp(component LiqoComponent)
	Complete(component LiqoComponent)
	EventRegister(component LiqoComponent, event EventType, status EventStatus)
}

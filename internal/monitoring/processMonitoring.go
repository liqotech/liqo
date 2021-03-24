package monitoring

type ProcessMonitoring interface {
	init(bool) error
	Start()
	Complete(component LiqoComponent)
	EventRegister(component LiqoComponent, event EventType, status EventStatus)
}

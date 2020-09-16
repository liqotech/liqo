package apiReflection

const (
	Configmaps = iota
	EndpointSlices
	Pods
	Services
)

type ApiType int

const (
	LiqoLabelKey   = "liqo/reflection"
	LiqoLabelValue = "reflected"
)

type ApiEvent struct {
	Event interface{}
	Api   ApiType
}

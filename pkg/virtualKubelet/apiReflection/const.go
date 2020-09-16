package apiReflection

const (
	Configmaps = iota
	Endpoints
	EndpointSlices
	Pods
	Services
	Secrets
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

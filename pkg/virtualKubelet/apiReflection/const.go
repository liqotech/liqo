package apiReflection

const (
	Configmaps = iota
	EndpointSlices
	Pods
	ReplicaControllers
	Services
	Secrets
)

type ApiType int

var ApiNames = map[ApiType]string{
	Configmaps:         "configmaps",
	EndpointSlices:     "endpointslices",
	Pods:               "pods",
	ReplicaControllers: "replicacontrollers",
	Services:           "services",
	Secrets:            "secrets",
}

const (
	LiqoLabelKey   = "virtualkubelet.liqo.io/reflection"
	LiqoLabelValue = "reflected"
)

type ApiEvent struct {
	Event interface{}
	Api   ApiType
}

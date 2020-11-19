package apiReflection

const (
	Configmaps = iota
	EndpointSlices
	Pods
	ReplicaSets
	Services
	Secrets
)

type ApiType int

var ApiNames = map[ApiType]string{
	Configmaps:     "configmaps",
	EndpointSlices: "endpointslices",
	Pods:           "pods",
	ReplicaSets:    "replicasets",
	Services:       "services",
	Secrets:        "secrets",
}

type ApiEvent struct {
	Event interface{}
	Api   ApiType
}

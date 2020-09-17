package apiReflector

import "reflect"

const (
	Configmaps = iota
	Endpoints
	Secrets
	Services
)

var apiTypes = map[int]reflect.Type{
	Configmaps: reflect.TypeOf(ConfigmapsReflector{}),
	Endpoints: nil,
	Secrets: nil,
	Services: nil,
}

package api

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

type ApiType int

const (
	Configmaps = iota
	Endpoints
	Secrets
	Services
)

type ApiEvent struct {
	Event interface{}
	Api   ApiType
}

var ApiMapping = map[ApiType]func(reflector *GenericAPIReflector) APIReflector{
	Configmaps: func(reflector *GenericAPIReflector) APIReflector {
		return &ConfigmapsReflector{GenericAPIReflector: *reflector}
	},
}

var InformerBuilding = map[ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer{
	Configmaps: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().ConfigMaps().Informer()
	},
}

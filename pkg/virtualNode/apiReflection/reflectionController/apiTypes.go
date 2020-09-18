package reflectionController

import (
	"github.com/liqotech/liqo/pkg/virtualNode/apiReflection/apis"
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
	event interface{}
	api   ApiType
}

var apiMapping = map[ApiType]func(reflector *GenericAPIReflector) APIReflector{
	Configmaps: func(reflector *GenericAPIReflector) APIReflector {
		return &apis.ConfigmapsReflector{GenericAPIReflector: *reflector}
	},
}

var apiPreProcessingHandlers = map[ApiType]PreProcessingHandlers{
	Configmaps: {
		addFunc:    nil,
		updateFunc: nil,
		deleteFunc: nil,
	},
}

var informerBuilding = map[ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer{
	Configmaps: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().ConfigMaps().Informer()
	},
}

package apiReflector

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	cache "k8s.io/client-go/tools/cache"
)

type ApiType int

const (
	Configmaps = iota
	Endpoints
	Secrets
	Services
)

var apiMapping = map[ApiType]func(reflector *GenericAPIReflector) APIReflector{
	Configmaps: func(reflector *GenericAPIReflector) APIReflector{
		return ConfigmapsReflector{GenericAPIReflector: reflector}
	},
	Endpoints: nil,
	Secrets: nil,
	Services: nil,
}

var apiPreProcessingHandlers = map[ApiType]PreProcessingHandlers{
	Configmaps: nil,
	Endpoints: nil,
	Secrets: nil,
	Services: nil,
}

var informerBuilding = map[ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer {
	Configmaps: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().ConfigMaps().Informer()
	},
}

type APIReflectorBuilder struct {
	client kubernetes.Interface
	informerFactories map[string]informers.SharedInformerFactory
}

func NewAPIREflectorBuilder(client kubernetes.Interface) *APIReflectorBuilder {
	return &APIReflectorBuilder{
		client:            client,
		informerFactories: make(map[string]informers.SharedInformerFactory),
	}
}

func (b *APIReflectorBuilder) BuildApiReflector(api ApiType) APIReflector {
	apiReflector :=  &GenericAPIReflector{
		preProcessingHandlers: apiPreProcessingHandlers[api],
		waitGroup:             nil,
		outputChan:            nil,
		client:                b.client,
		informer:              make(map[string]cache.SharedIndexInformer),
	}
	return apiMapping[api](apiReflector)
}

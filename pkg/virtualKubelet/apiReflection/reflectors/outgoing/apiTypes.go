package outgoing

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var ApiMapping = map[apimgmt.ApiType]func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector{
	apimgmt.Configmaps: func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
		return &ConfigmapsReflector{APIReflector: reflector}
	},
	apimgmt.EndpointSlices: func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
		return &EndpointSlicesReflector{
			APIReflector:         reflector,
			LocalRemappedPodCIDR: opts[types.LocalRemappedPodCIDR],
			NodeName:             opts[types.NodeName],
		}
	},
	apimgmt.Services: func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
		return &ServicesReflector{APIReflector: reflector}
	},
	apimgmt.Secrets: func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
		return &SecretsReflector{APIReflector: reflector}
	},
}

var HomeInformerBuilders = map[apimgmt.ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer{
	apimgmt.Configmaps: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().ConfigMaps().Informer()
	},
	apimgmt.EndpointSlices: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Discovery().V1beta1().EndpointSlices().Informer()
	},
	apimgmt.Services: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().Services().Informer()
	},
	apimgmt.Secrets: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().Secrets().Informer()
	},
}

var ForeignInformerBuilders = map[apimgmt.ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer{
	apimgmt.Configmaps: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().ConfigMaps().Informer()
	},
	apimgmt.EndpointSlices: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Discovery().V1beta1().EndpointSlices().Informer()
	},
	apimgmt.Services: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().Services().Informer()
	},
	apimgmt.Secrets: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().Secrets().Informer()
	},
}

var HomeIndexers = map[apimgmt.ApiType]func() cache.Indexers{
	apimgmt.Configmaps:     addConfigmapsIndexers,
	apimgmt.EndpointSlices: addEndpointSlicesIndexers,
	apimgmt.Secrets:        addSecretsIndexers,
	apimgmt.Services:       addServicesIndexers,
}

var ForeignIndexers = map[apimgmt.ApiType]func() cache.Indexers{}

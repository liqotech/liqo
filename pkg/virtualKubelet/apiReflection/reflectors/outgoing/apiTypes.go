package outgoing

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var ApiMapping = map[apimgmt.ApiType]func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector{
	apimgmt.Configmaps: func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
		return &ConfigmapsReflector{APIReflector: reflector}
	},
	/*
		apimgmt.EndpointSlices: func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
			return &EndpointSlicesReflector{
				APIReflector: reflector,
				localRemappedPodCIDR: opts[types.LocalRemappedPodCIDR],
				nodeName: opts[types.NodeName],
			}
		},
	*/
}

var InformerBuilders = map[apimgmt.ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer{
	apimgmt.Configmaps: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().ConfigMaps().Informer()
	},
	/*
		apimgmt.EndpointSlices: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
			return factory.Discovery().V1beta1().EndpointSlices().Informer()
		},
		apimgmt.Services: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
			return factory.Core().V1().Services().Informer()
		},*/
}

var Indexers = map[apimgmt.ApiType]func() cache.Indexers{
	apimgmt.Configmaps: addConfigmapsIndexers,
	apimgmt.Services:   addServicesIndexers,
}

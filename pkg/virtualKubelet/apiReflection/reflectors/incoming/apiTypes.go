package incoming

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var ApiMapping = map[apimgmt.ApiType]func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector{
	apimgmt.Pods: func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector {
		return &PodsIncomingReflector{
			APIReflector:          reflector,
			RemoteRemappedPodCIDR: opts[types.RemoteRemappedPodCIDR]}
	},
}

var HomeInformerBuilders = map[apimgmt.ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer{
	apimgmt.Pods: func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().Pods().Informer()
	},
}

var HomeIndexers = map[apimgmt.ApiType]func() cache.Indexers{
	apimgmt.Pods: AddPodsIndexers,
}

var ForeignInformerBuilders = map[apimgmt.ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer{}

var ForeignIndexers = map[apimgmt.ApiType]func() cache.Indexers{}

package outgoing

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
)

var ReflectorBuilders = map[apimgmt.ApiType]func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector{
	apimgmt.Configmaps:     configmapsReflectorBuilder,
	apimgmt.EndpointSlices: endpointslicesReflectorBuilder,
	apimgmt.Secrets:        secretsReflectorBuilder,
	apimgmt.Services:       servicesReflectorBuilder,
}

func configmapsReflectorBuilder(reflector ri.APIReflector, _ map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	return &ConfigmapsReflector{APIReflector: reflector}
}

func endpointslicesReflectorBuilder(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	return &EndpointSlicesReflector{
		APIReflector:         reflector,
		LocalRemappedPodCIDR: opts[types.LocalRemappedPodCIDR],
		NodeName:             opts[types.NodeName],
	}
}

func secretsReflectorBuilder(reflector ri.APIReflector, _ map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	return &SecretsReflector{APIReflector: reflector}
}

func servicesReflectorBuilder(reflector ri.APIReflector, _ map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	return &ServicesReflector{APIReflector: reflector}
}

package incoming

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
)

var ReflectorBuilder = map[apimgmt.ApiType]func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector{
	apimgmt.Pods: podsReflectorBuilder,
}

func podsReflectorBuilder(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector {
	return &PodsIncomingReflector{
		APIReflector:          reflector,
		RemoteRemappedPodCIDR: opts[types.RemoteRemappedPodCIDR]}
}

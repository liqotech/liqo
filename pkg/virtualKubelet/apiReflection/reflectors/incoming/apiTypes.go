package incoming

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
)

var ReflectorBuilder = map[apimgmt.ApiType]func(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector{
	apimgmt.Pods:        podsReflectorBuilder,
	apimgmt.ReplicaSets: replicaSetsReflectorBuilder,
}

func podsReflectorBuilder(reflector ri.APIReflector, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector {
	return &PodsIncomingReflector{
		APIReflector:  reflector,
		HomePodGetter: GetHomePodFunc,
	}
}

func replicaSetsReflectorBuilder(reflector ri.APIReflector, _ map[options.OptionKey]options.Option) ri.IncomingAPIReflector {
	return &ReplicaSetsIncomingReflector{
		APIReflector: reflector,
	}
}

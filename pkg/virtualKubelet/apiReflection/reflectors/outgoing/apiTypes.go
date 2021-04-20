package outgoing

import (
	"fmt"

	"google.golang.org/grpc"
	"k8s.io/klog"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet"
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
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", liqoconst.NetworkManagerServiceName, liqoconst.NetworkManagerIpamPort),
		grpc.WithInsecure(),
		grpc.WithBlock())
	if err != nil {
		klog.Error(err)
	}
	ipamClient := liqonet.NewIpamClient(conn)

	return &EndpointSlicesReflector{
		APIReflector:         reflector,
		LocalRemappedPodCIDR: opts[types.LocalRemappedPodCIDR],
		VirtualNodeName:      opts[types.VirtualNodeName],
		IpamClient:           ipamClient,
	}
}

func secretsReflectorBuilder(reflector ri.APIReflector, _ map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	return &SecretsReflector{APIReflector: reflector}
}

func servicesReflectorBuilder(reflector ri.APIReflector, _ map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	return &ServicesReflector{APIReflector: reflector}
}

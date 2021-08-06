package outgoing

import (
	"fmt"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
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
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", opts[options.OptionKey(types.LiqoIpamServer)].Value(), liqoconst.NetworkManagerIpamPort),
		grpc.WithInsecure(),
		grpc.WithBlock())
	if err != nil {
		klog.Error(err)
	}
	ipamClient := liqonetIpam.NewIpamClient(conn)

	return &EndpointSlicesReflector{
		APIReflector:    reflector,
		VirtualNodeName: opts[types.VirtualNodeName],
		IpamClient:      ipamClient,
	}
}

func secretsReflectorBuilder(reflector ri.APIReflector, _ map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	return &SecretsReflector{APIReflector: reflector}
}

func servicesReflectorBuilder(reflector ri.APIReflector, _ map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	return &ServicesReflector{APIReflector: reflector}
}

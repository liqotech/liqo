package main

import (
	"flag"
	advertisement_operator "github.com/liqoTech/liqo/internal/advertisement-operator"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var localKubeconfig, foreignKubeconfig, clusterId string
	var gatewayIP, gatewayPrivateIP string
	var peeringRequestName string

	flag.StringVar(&localKubeconfig, "local-kubeconfig", "", "The path to the kubeconfig of your local cluster.")
	flag.StringVar(&foreignKubeconfig, "foreign-kubeconfig", "", "The path to the kubeconfig of the foreign cluster.")
	flag.StringVar(&clusterId, "cluster-id", "", "The cluster ID of your cluster")
	flag.StringVar(&gatewayIP, "gateway-ip", "", "The IP address of the gateway node")
	flag.StringVar(&gatewayPrivateIP, "gateway-private-ip", "", "The private IP address of the gateway node")
	flag.StringVar(&peeringRequestName, "peering-request", "", "Name of PeeringRequest CR where get configurations")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	advertisement_operator.StartBroadcaster(clusterId, localKubeconfig, foreignKubeconfig, gatewayIP, gatewayPrivateIP, peeringRequestName)
}

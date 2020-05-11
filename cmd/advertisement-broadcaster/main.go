package main

import (
	"flag"
	advertisement_operator "github.com/netgroup-polito/dronev2/internal/advertisement-operator"
)

func main(){
	var localKubeconfig, foreignKubeconfig, clusterId string
	var gatewayIP, gatewayPrivateIP string

	flag.StringVar(&localKubeconfig, "local-kubeconfig", "", "The path to the kubeconfig of your local cluster.")
	flag.StringVar(&foreignKubeconfig, "foreign-kubeconfig", "", "The path to the kubeconfig of the foreign cluster.")
	flag.StringVar(&clusterId, "cluster-id", "", "The cluster ID of your cluster")
	flag.StringVar(&gatewayIP, "gateway-ip", "", "The IP address of the gateway node")
	flag.StringVar(&gatewayPrivateIP, "gateway-private-ip", "", "The private IP address of the gateway node")
	flag.Parse()

	advertisement_operator.StartBroadcaster(clusterId, localKubeconfig, foreignKubeconfig, gatewayIP, gatewayPrivateIP)
}
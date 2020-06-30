package main

import (
	"flag"
	advertisement_operator "github.com/liqoTech/liqo/internal/advertisement-operator"
	"k8s.io/klog"
	"os"
)

func main() {
	var localKubeconfig, foreignKubeconfig, clusterId string
	var gatewayIP, gatewayPrivateIP string
	var peeringRequestName string
	var saName string

	flag.StringVar(&localKubeconfig, "local-kubeconfig", "", "The path to the kubeconfig of your local cluster.")
	flag.StringVar(&foreignKubeconfig, "foreign-kubeconfig", "", "The path to the kubeconfig of the foreign cluster.")
	flag.StringVar(&clusterId, "cluster-id", "", "The cluster ID of your cluster")
	flag.StringVar(&gatewayIP, "gateway-ip", "", "The IP address of the gateway node")
	flag.StringVar(&gatewayPrivateIP, "gateway-private-ip", "", "The private IP address of the gateway node")
	flag.StringVar(&peeringRequestName, "peering-request", "", "Name of PeeringRequest CR containing configurations")
	flag.StringVar(&saName, "service-account", "broadcaster", "The name of the ServiceAccount used to create the kubeconfig that will be sent to the foreign cluster")
	flag.Parse()

	if peeringRequestName == "" {
		klog.Error("no peering request provided, exiting")
		os.Exit(1)
	}

	err := advertisement_operator.StartBroadcaster(clusterId, localKubeconfig, foreignKubeconfig, gatewayIP, gatewayPrivateIP, peeringRequestName, saName)
	if err != nil {
		klog.Errorln(err, "Unable to start broadcaster: exiting")
		os.Exit(1)
	}
}

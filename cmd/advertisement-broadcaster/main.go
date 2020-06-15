package main

import (
	"errors"
	"flag"
	advertisement_operator "github.com/liqoTech/liqo/internal/advertisement-operator"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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

	log := ctrl.Log.WithName("setup")
	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	if peeringRequestName == "" {
		log.Error(errors.New("no peering request provided, exiting"), "")
		os.Exit(1)
	}

	err := advertisement_operator.StartBroadcaster(clusterId, localKubeconfig, foreignKubeconfig, gatewayIP, gatewayPrivateIP, peeringRequestName, saName)
	if err != nil {
		log.Error(err, "Unable to start broadcaster: exiting")
		os.Exit(1)
	}
}

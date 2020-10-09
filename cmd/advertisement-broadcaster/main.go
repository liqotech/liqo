package main

import (
	"flag"
	advop "github.com/liqotech/liqo/internal/advertisement-operator"
	"k8s.io/klog"
	"os"
)

func main() {
	var localKubeconfig, clusterId string
	var peeringRequestName string
	var saName string

	flag.StringVar(&localKubeconfig, "local-kubeconfig", "", "The path to the kubeconfig of your local cluster.")
	flag.StringVar(&clusterId, "cluster-id", "", "The cluster ID of your cluster")
	flag.StringVar(&peeringRequestName, "peering-request", "", "Name of PeeringRequest CR containing configurations")
	flag.StringVar(&saName, "service-account", "vk-remote", "The name of the ServiceAccount used to create the kubeconfig that will be sent to the foreign cluster")
	flag.Parse()

	if peeringRequestName == "" {
		klog.Error("no peering request provided, exiting")
		os.Exit(1)
	}

	err := advop.StartBroadcaster(clusterId, localKubeconfig, peeringRequestName, saName)
	if err != nil {
		klog.Errorln(err, "Unable to start broadcaster: exiting")
		os.Exit(1)
	}
}

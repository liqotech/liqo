package main

import (
	"flag"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

type liqonetCommonFlags struct {
	metricsAddr string
	runAs       string
}

func addCommonFlags(liqonet *liqonetCommonFlags) {
	flag.StringVar(&liqonet.metricsAddr, "metrics-bind-addr", ":0", "The address the metric endpoint binds to.")
	flag.StringVar(&liqonet.runAs, "run-as", liqoconst.LiqoGatewayOperatorName,
		"The accepted values are: liqo-gateway, liqo-route, tunnelEndpointCreator-operator. The default value is \"liqo-gateway\"")
}

package main

import (
	"flag"
	"fmt"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

type liqonetCommonFlags struct {
	metricsAddr string
	runAs       string
}

func addCommonFlags(liqonet *liqonetCommonFlags) {
	flag.StringVar(&liqonet.metricsAddr, "metrics-bind-addr", ":0", "The address the metric endpoint binds to.")
	flag.StringVar(&liqonet.runAs, "run-as", liqoconst.LiqoGatewayOperatorName,
		fmt.Sprintf("The accepted values are: %q, %q, %q.",
			liqoconst.LiqoGatewayOperatorName, liqoconst.LiqoRouteOperatorName, liqoconst.LiqoNetworkManagerName))
}

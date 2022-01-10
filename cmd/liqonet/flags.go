// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

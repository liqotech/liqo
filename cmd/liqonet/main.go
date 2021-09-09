// Copyright 2019-2021 The Liqo Authors
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

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	// This labels are the ones set during the deployment of liqo using the helm chart.
	// Any change to those labels on the helm chart has also to be reflected here.
	podInstanceLabelKey     = "app.kubernetes.io/instance"
	routeInstanceLabelValue = "liqo-route"
	podNameLabelKey         = "app.kubernetes.io/name"
	routeNameLabelValue     = "route"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = netv1alpha1.AddToScheme(scheme)
}

func main() {
	klog.InitFlags(nil)

	commonFlags := &liqonetCommonFlags{}
	routeFlags := &routeOperatorFlags{}
	gatewayFlags := &gatewayOperatorFlags{}
	managerFlags := &networkManagerFlags{}

	addCommonFlags(commonFlags)
	addGatewayOperatorFlags(gatewayFlags)
	addRouteOperatorFlags(routeFlags)
	addNetworkManagerFlags(managerFlags)

	flag.Parse()

	switch commonFlags.runAs {
	case liqoconst.LiqoRouteOperatorName:
		runRouteOperator(commonFlags, routeFlags)
	case liqoconst.LiqoGatewayOperatorName:
		runGatewayOperator(commonFlags, gatewayFlags)
	case liqoconst.LiqoNetworkManagerName:
		runEndpointCreatorOperator(commonFlags, managerFlags)
	}
}

/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	"github.com/liqoTech/liqo/api/tunnel-endpoint/v1"
	"github.com/liqoTech/liqo/internal/liqonet"
	"github.com/liqoTech/liqo/pkg/liqonet"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"net"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	defaultConfig = liqonet.VxlanNetConfig{
		Network:    "192.168.200.0/24",
		DeviceName: "liqonet",
		Port:       "4789", //IANA assigned
		Vni:        "200",
	}
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = v1.AddToScheme(scheme)

	_ = protocolv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var runAsRouteOperator bool
	var runAs string

	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&runAsRouteOperator, "run-as-route-operator", false,
		"Runs the controller as Route-Operator, the default value is false and will run as Tunnel-Operator")
	flag.StringVar(&runAs, "run-as", "tunnel-operator", "The accepted values are: tunnel-operator, route-operator, tunnelEndpointCreator-operator. The default value is \"tunnel-operator\"")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// creates the in-cluster config or uses the .kube/config file
	config := ctrl.GetConfigOrDie()

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	// +kubebuilder:scaffold:builder
	switch runAs {
	case "route-operator":
		vxlanConfig, err := liqonet.ReadVxlanNetConfig(defaultConfig)
		if err != nil {
			setupLog.Error(err, "an error occurred while getting the vxlan network configuration")
		}
		vxlanPort, err := strconv.Atoi(vxlanConfig.Port)
		if err != nil {
			setupLog.Error(err, "unable to convert vxlan port "+vxlanConfig.Port+" from string to int.")
		}
		err = liqonet.CreateVxLANInterface(clientset, vxlanConfig)
		if err != nil {
			setupLog.Error(err, "an error occurred while creating vxlan interface")
		}
		//Enable loose mode reverse path filtering on the vxlan interfaces
		err = liqonet.Enable_rp_filter()
		if err != nil {
			setupLog.Error(err, "an error occurred while enabling loose mode reverse path filtering")
			os.Exit(3)
		}
		isGatewayNode, err := liqonet.IsGatewayNode(clientset)
		if err != nil {
			setupLog.Error(err, "an error occurred while checking if the node is the GatewayNode")
			os.Exit(2)
		}
		//get node name
		nodeName, err := liqonet.GetNodeName()
		if err != nil {
			setupLog.Error(err, "an error occurred while retrieving node name")
			os.Exit(4)
		}
		//get node name
		podCIDR, err := liqonet.GetClusterPodCIDR()
		if err != nil {
			setupLog.Error(err, "an error occurred while retrieving cluster pod cidr")
			os.Exit(6)
		}
		gatewayVxlanIP, err := liqonet.GetGatewayVxlanIP(clientset, vxlanConfig)
		if err != nil {
			setupLog.Error(err, "unable to derive gatewayVxlanIP")
			os.Exit(5)
		}

		r := &controllers.RouteController{
			Client:                             mgr.GetClient(),
			Log:                                ctrl.Log.WithName("route-operator"),
			Scheme:                             mgr.GetScheme(),
			RouteOperator:                      runAsRouteOperator,
			ClientSet:                          clientset,
			RoutesPerRemoteCluster:             make(map[string][]netlink.Route),
			IsGateway:                          isGatewayNode,
			VxlanNetwork:                       vxlanConfig.Network,
			VxlanIfaceName:                     vxlanConfig.DeviceName,
			VxlanPort:                          vxlanPort,
			IPTablesRuleSpecsReferencingChains: make(map[string]liqonet.IPtableRule),
			IPTablesChains:                     make(map[string]liqonet.IPTableChain),
			IPtablesRuleSpecsPerRemoteCluster:  make(map[string][]liqonet.IPtableRule),
			NodeName:                           nodeName,
			GatewayVxlanIP:                     gatewayVxlanIP,
			ClusterPodCIDR:                     podCIDR,
			RetryTimeout:                       30*time.Second,
		}
		if err = r.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Route")
			os.Exit(1)
		}
		setupLog.Info("Starting manager as Route-Operator")
		if err := mgr.Start(r.SetupSignalHandlerForRouteOperator()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}

	case "tunnel-operator":
		r := &controllers.TunnelController{
			Client:                       mgr.GetClient(),
			Log:                          ctrl.Log.WithName("controllers").WithName("TunnelEndpoint"),
			Scheme:                       mgr.GetScheme(),
			TunnelIFacesPerRemoteCluster: make(map[string]int),
		}
		if err = r.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "TunnelEndpoint")
			os.Exit(1)
		}
		setupLog.Info("Starting manager as Tunnel-Operator")
		if err := mgr.Start(r.SetupSignalHandlerForTunnelOperator()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}

	case "tunnelEndpointCreator-operator":
		r := &controllers.TunnelEndpointCreator{
			Client:            mgr.GetClient(),
			Log:               ctrl.Log.WithName("controllers").WithName("TunnelEndpointCreator"),
			Scheme:            mgr.GetScheme(),
			TunnelEndpointMap: make(map[string]types.NamespacedName),
			UsedSubnets:       make(map[string]*net.IPNet),
			FreeSubnets:       make(map[string]*net.IPNet),
			IPManager: liqonet.IpManager{
				UsedSubnets:      make(map[string]*net.IPNet),
				FreeSubnets:      make(map[string]*net.IPNet),
				SubnetPerCluster: make(map[string]*net.IPNet),
				Log:              ctrl.Log.WithName("IPAM"),
			},
			RetryTimeout:          30*time.Second,
		}
		if err := r.IPManager.Init(); err != nil {
			setupLog.Error(err, "unable to initialize ipam")
			os.Exit(2)
		}
		if err = r.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "TunnelEndpointCreator")
			os.Exit(1)
		}

		setupLog.Info("starting manager as tunnelEndpointCreator-operator")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}

}

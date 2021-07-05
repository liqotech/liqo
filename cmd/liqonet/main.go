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
	"os"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterConfig "github.com/liqotech/liqo/apis/config/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	routeoperator "github.com/liqotech/liqo/internal/liqonet/route-operator"
	tunneloperator "github.com/liqotech/liqo/internal/liqonet/tunnel-operator"
	"github.com/liqotech/liqo/internal/liqonet/tunnelEndpointCreator"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	liqonetns "github.com/liqotech/liqo/pkg/liqonet/netns"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	liqorouting "github.com/liqotech/liqo/pkg/liqonet/routing"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/mapperUtils"
)

var (
	scheme      = runtime.NewScheme()
	vxlanConfig = &overlay.VxlanDeviceAttrs{
		Vni:      18952,
		Name:     liqoconst.VxlanDeviceName,
		VtepPort: 4789,
		VtepAddr: nil,
		Mtu:      1420,
	}
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = netv1alpha1.AddToScheme(scheme)
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var runAs string

	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&runAs, "run-as", liqoconst.LiqoGatewayOperatorName,
		"The accepted values are: liqo-gateway, liqo-route, tunnelEndpointCreator-operator. The default value is \"liqo-gateway\"")
	flag.Parse()
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		MapperProvider:     mapperUtils.LiqoMapperProvider(scheme),
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		klog.Errorf("unable to get manager: %s", err)
		os.Exit(1)
	}
	clientset := kubernetes.NewForConfigOrDie(mgr.GetConfig())

	switch runAs {
	case liqoconst.LiqoRouteOperatorName:
		mutex := &sync.RWMutex{}
		nodeMap := map[string]string{}
		// Get the pod ip and parse to net.IP
		podIP, err := utils.GetPodIP()
		if err != nil {
			klog.Errorf("unable to get podIP: %v", err)
			os.Exit(1)
		}
		nodeName, err := utils.GetNodeName()
		if err != nil {
			klog.Errorf("unable to get node name: %v", err)
			os.Exit(1)
		}
		vxlanConfig.VtepAddr = podIP
		vxlanDevice, err := overlay.NewVxlanDevice(vxlanConfig)
		if err != nil {
			klog.Errorf("an error occurred while creating vxlan device : %v", err)
			os.Exit(1)
		}
		vxlanRoutingManager, err := liqorouting.NewVxlanRoutingManager(liqoconst.RoutingTableID,
			podIP.String(), liqoconst.OverlayNetPrefix, vxlanDevice)
		if err != nil {
			klog.Errorf("an error occurred while creating the vxlan routing manager: %v", err)
			os.Exit(1)
		}
		eventRecorder := mgr.GetEventRecorderFor(liqoconst.LiqoRouteOperatorName + "." + podIP.String())
		routeController := routeoperator.NewRouteController(podIP.String(), vxlanDevice, vxlanRoutingManager, eventRecorder, mgr.GetClient())
		if err = routeController.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup controller: %s", err)
			os.Exit(1)
		}
		overlayController, err := routeoperator.NewOverlayController(podIP.String(),
			routeoperator.PodLabelSelector, vxlanDevice, mutex, nodeMap, mgr.GetClient())
		if err != nil {
			klog.Errorf("an error occurred while creating overlay controller: %v", err)
			os.Exit(3)
		}
		if err = overlayController.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup overlay controller: %s", err)
			os.Exit(1)
		}
		symmetricRoutingOperator, err := routeoperator.NewSymmetricRoutingOperator(nodeName,
			liqoconst.RoutingTableID, vxlanDevice, mutex, nodeMap, mgr.GetClient())
		if err != nil {
			klog.Errorf("an error occurred while creting symmetric routing controller: %v", err)
			os.Exit(4)
		}
		if err = symmetricRoutingOperator.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup overlay controller: %s", err)
			os.Exit(1)
		}
		if err := mgr.Start(routeController.SetupSignalHandlerForRouteOperator()); err != nil {
			klog.Errorf("unable to start controller: %s", err)
			os.Exit(1)
		}
	case liqoconst.LiqoGatewayOperatorName:
		// Get the pod ip and parse to net.IP
		podIP, err := utils.GetPodIP()
		if err != nil {
			klog.Errorf("unable to get podIP: %v", err)
			os.Exit(1)
		}
		podNamespace, err := utils.GetPodNamespace()
		if err != nil {
			klog.Errorf("unable to get pod namespace: %v", err)
			os.Exit(1)
		}
		eventRecorder := mgr.GetEventRecorderFor(liqoconst.LiqoGatewayOperatorName + "." + podIP.String())
		// This map is updated by the tunnel operator after a successful tunnel creation
		// and is consumed by the natmapping operator to check whether the tunnel is ready or not.
		var readyClustersMutex sync.Mutex
		readyClusters := make(map[string]struct{})
		// Create new network namespace for the gateway (gatewayNetns).
		// If the namespace already exists it will be deleted and recreated.
		// It is created here because both the tunnel-operator and the natmapping-operator
		// need to know the namespace to configure.
		gatewayNetns, err := liqonetns.CreateNetns(liqoconst.GatewayNetnsName)
		if err != nil {
			klog.Errorf("an error occurred while creating custom network namespace: %s", err.Error())
			os.Exit(1)
		}
		klog.Infof("created custom network namespace {%s}", liqoconst.GatewayNetnsName)
		tunnelController, err := tunneloperator.NewTunnelController(podIP.String(),
			podNamespace, eventRecorder, clientset, mgr.GetClient(), &readyClustersMutex,
			readyClusters, gatewayNetns)
		if err != nil {
			klog.Errorf("an error occurred while creating the tunnel controller: %v", err)
			_ = tunnelController.CleanUpConfiguration(liqoconst.GatewayNetnsName, liqoconst.HostVethName)
			tunnelController.RemoveAllTunnels()
			os.Exit(1)
		}
		if err = tunnelController.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup tunnel controller: %s", err)
			os.Exit(1)
		}

		nmc, err := tunneloperator.NewNatMappingController(mgr.GetClient(), &readyClustersMutex, readyClusters, gatewayNetns)
		if err != nil {
			klog.Errorf("an error occurred while creating the natmapping controller: %v", err)
			os.Exit(1)
		}
		if err = nmc.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup natmapping controller: %s", err)
			os.Exit(1)
		}
		klog.Info("Starting manager as Tunnel-Operator")
		if err := mgr.Start(tunnelController.SetupSignalHandlerForTunnelOperator()); err != nil {
			klog.Errorf("unable to start tunnel controller: %s", err)
			os.Exit(1)
		}
	case "tunnelEndpointCreator-operator":
		dynClient := dynamic.NewForConfigOrDie(mgr.GetConfig())
		ipam := liqonetIpam.NewIPAM()
		err = ipam.Init(liqonetIpam.Pools, dynClient, liqoconst.NetworkManagerIpamPort)
		if err != nil {
			klog.Errorf("cannot init IPAM:%w", err)
		}
		r := &tunnelEndpointCreator.TunnelEndpointCreator{
			Client:                     mgr.GetClient(),
			Scheme:                     mgr.GetScheme(),
			ClientSet:                  clientset,
			DynClient:                  dynClient,
			Manager:                    mgr,
			Namespace:                  "liqo",
			WaitConfig:                 &sync.WaitGroup{},
			ReservedSubnets:            make([]string, 0),
			AdditionalPools:            make([]string, 0),
			Configured:                 make(chan bool, 1),
			ForeignClusterStartWatcher: make(chan bool, 1),
			ForeignClusterStopWatcher:  make(chan struct{}),

			IPManager:    ipam,
			RetryTimeout: 30 * time.Second,
		}
		r.WaitConfig.Add(3)
		//starting configuration watcher
		config, err := ctrl.GetConfig()
		if err != nil {
			klog.Error(err)
			os.Exit(2)
		}
		r.WatchConfiguration(config, &clusterConfig.GroupVersion)
		if err = r.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to create controller controller TunnelEndpointCreator: %s", err)
			os.Exit(1)
		}
		go r.StartForeignClusterWatcher()
		go r.StartGWPodWatcher()
		go r.StartServiceWatcher()
		go r.StartSecretWatcher()
		klog.Info("starting manager as tunnelEndpointCreator-operator")
		if err := mgr.Start(r.SetupSignalHandlerForTunEndCreator()); err != nil {
			klog.Errorf("an error occurred while starting manager: %s", err)
			os.Exit(1)
		}
	}
}

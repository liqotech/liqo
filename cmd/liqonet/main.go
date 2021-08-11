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
	"strings"
	"sync"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

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
	tunnelwg "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/mapperUtils"
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
	var metricsAddr, runAs string
	var enableLeaderElection bool
	leaseDuration := 7 * time.Second
	renewDeadLine := 5 * time.Second
	retryPeriod := 2 * time.Second
	flag.StringVar(&metricsAddr, "metrics-bind-addr", ":0", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&runAs, "run-as", liqoconst.LiqoGatewayOperatorName,
		"The accepted values are: liqo-gateway, liqo-route, tunnelEndpointCreator-operator. The default value is \"liqo-gateway\"")

	klog.InitFlags(nil)
	flag.Parse()

	switch runAs {
	case liqoconst.LiqoRouteOperatorName:
		mutex := &sync.RWMutex{}
		nodeMap := map[string]string{}
		// Get the pod ip and parse to net.IP.
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
		podNamespace, err := utils.GetPodNamespace()
		if err != nil {
			klog.Errorf("unable to get pod namespace: %v", err)
			os.Exit(1)
		}
		// Asking the api-server to only inform the operator for the pods running in a node different from the one
		// where the operator is running.
		smcFieldSelector, err := fields.ParseSelector(strings.Join([]string{"spec.nodeName", "!=", nodeName}, ""))
		if err != nil {
			klog.Errorf("unable to create label requirement: %v", err)
			os.Exit(1)
		}
		// Asking the api-server to only inform the operator for the pods running in a node different from
		// the virtual nodes. We want to process only the pods running on the local cluster and not the ones
		// offloaded to a remote cluster.
		smcLabelRequirement, err := labels.NewRequirement(liqoconst.LocalPodLabelKey, selection.DoesNotExist, []string{})
		if err != nil {
			klog.Errorf("unable to create label requirement: %v", err)
			os.Exit(1)
		}
		smcLabelSelector := labels.NewSelector().Add(*smcLabelRequirement)
		mainMgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			MapperProvider:     mapperUtils.LiqoMapperProvider(scheme),
			Scheme:             scheme,
			MetricsBindAddress: metricsAddr,
			NewCache: cache.BuilderWithOptions(cache.Options{
				SelectorsByObject: cache.SelectorsByObject{
					&corev1.Pod{}: {
						Field: smcFieldSelector,
						Label: smcLabelSelector,
					},
				},
			}),
		})
		if err != nil {
			klog.Errorf("unable to get manager: %s", err)
			os.Exit(1)
		}
		// Asking the api-server to only inform the operator for the pods that are part of the route component.
		ovcLabelSelector := labels.SelectorFromSet(labels.Set{
			podNameLabelKey:     routeNameLabelValue,
			podInstanceLabelKey: routeInstanceLabelValue,
		})
		// This manager is used by the overlay operator and it is limited to the pods running
		// on the same namespace as the operator.
		overlayMgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			MapperProvider:     mapperUtils.LiqoMapperProvider(scheme),
			Scheme:             scheme,
			MetricsBindAddress: metricsAddr,
			Namespace:          podNamespace,
			NewCache: cache.BuilderWithOptions(cache.Options{
				SelectorsByObject: cache.SelectorsByObject{
					&corev1.Pod{}: {
						Label: ovcLabelSelector,
					},
				},
			}),
		})
		if err != nil {
			klog.Errorf("unable to get manager: %s", err)
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
		eventRecorder := mainMgr.GetEventRecorderFor(liqoconst.LiqoRouteOperatorName + "." + podIP.String())
		routeController := routeoperator.NewRouteController(podIP.String(), vxlanDevice, vxlanRoutingManager, eventRecorder, mainMgr.GetClient())
		if err = routeController.SetupWithManager(mainMgr); err != nil {
			klog.Errorf("unable to setup controller: %s", err)
			os.Exit(1)
		}
		overlayController, err := routeoperator.NewOverlayController(podIP.String(), vxlanDevice, mutex, nodeMap, overlayMgr.GetClient())
		if err != nil {
			klog.Errorf("an error occurred while creating overlay controller: %v", err)
			os.Exit(3)
		}
		if err = overlayController.SetupWithManager(overlayMgr); err != nil {
			klog.Errorf("unable to setup overlay controller: %s", err)
			os.Exit(1)
		}
		symmetricRoutingController, err := routeoperator.NewSymmetricRoutingOperator(nodeName,
			liqoconst.RoutingTableID, vxlanDevice, mutex, nodeMap, mainMgr.GetClient())
		if err != nil {
			klog.Errorf("an error occurred while creting symmetric routing controller: %v", err)
			os.Exit(4)
		}
		if err = symmetricRoutingController.SetupWithManager(mainMgr); err != nil {
			klog.Errorf("unable to setup overlay controller: %s", err)
			os.Exit(1)
		}
		if err := mainMgr.Add(overlayMgr); err != nil {
			klog.Errorf("unable to add the overlay manager to the main manager: %s", err)
			os.Exit(1)
		}
		if err := mainMgr.Start(routeController.SetupSignalHandlerForRouteOperator()); err != nil {
			klog.Errorf("unable to start controller: %s", err)
			os.Exit(1)
		}
	case liqoconst.LiqoGatewayOperatorName:
		// Get the pod ip and parse to net.IP.
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
		main, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			MapperProvider:                mapperUtils.LiqoMapperProvider(scheme),
			Scheme:                        scheme,
			MetricsBindAddress:            metricsAddr,
			LeaderElection:                enableLeaderElection,
			LeaderElectionID:              liqoconst.GatewayLeaderElectionID,
			LeaderElectionNamespace:       podNamespace,
			LeaderElectionReleaseOnCancel: true,
			LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
			LeaseDuration:                 &leaseDuration,
			RenewDeadline:                 &renewDeadLine,
			RetryPeriod:                   &retryPeriod,
			NewCache: cache.BuilderWithOptions(cache.Options{
				SelectorsByObject: tunneloperator.LabelSelector,
			}),
		})
		if err != nil {
			klog.Errorf("unable to get main manager: %s", err)
			os.Exit(1)
		}
		clientset := kubernetes.NewForConfigOrDie(main.GetConfig())
		eventRecorder := main.GetEventRecorderFor(liqoconst.LiqoGatewayOperatorName + "." + podIP.String())
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
		// Get the current network namespace (hostNetns).
		hostNetns, err := ns.GetCurrentNS()
		if err != nil {
			klog.Errorf("an error occurred while getting host network namespace: %s", err.Error())
			os.Exit(1)
		}
		klog.Infof("created custom network namespace {%s}", liqoconst.GatewayNetnsName)

		labelController := tunneloperator.NewLabelerController(podIP.String(), main.GetClient())
		if err = labelController.SetupWithManager(main); err != nil {
			klog.Errorf("unable to setup labeler controller: %s", err)
			os.Exit(1)
		}
		tunnelController, err := tunneloperator.NewTunnelController(podIP.String(), podNamespace, eventRecorder,
			clientset, main.GetClient(), &readyClustersMutex, readyClusters, gatewayNetns, hostNetns)
		// If something goes wrong while creating and configuring the tunnel controller
		// then make sure that we remove all the resources created during the create process.
		if err != nil {
			klog.Errorf("an error occurred while creating the tunnel controller: %v", err)
			klog.Info("cleaning up gateway network namespace")
			if err := liqonetns.DeleteNetns(liqoconst.GatewayNetnsName); err != nil {
				klog.Errorf("an error occurred while deleting netns {%s}: %v", liqoconst.GatewayNetnsName, err)
			}
			klog.Info("cleaning up wireguard tunnel interface")
			if err := utils.DeleteIFaceByName(tunnelwg.DeviceName); err != nil {
				klog.Errorf("an error occurred while deleting iface {%s}: %v", tunnelwg.DriverName, err)
			}
			os.Exit(1)
		}
		if err = tunnelController.SetupWithManager(main); err != nil {
			klog.Errorf("unable to setup tunnel controller: %s", err)
			os.Exit(1)
		}
		natMappingController, err := tunneloperator.NewNatMappingController(main.GetClient(), &readyClustersMutex,
			readyClusters, gatewayNetns)
		if err != nil {
			klog.Errorf("an error occurred while creating the natmapping controller: %v", err)
			os.Exit(1)
		}
		if err = natMappingController.SetupWithManager(main); err != nil {
			klog.Errorf("unable to setup natmapping controller: %s", err)
			os.Exit(1)
		}

		klog.Info("Starting manager as Tunnel-Operator")
		if err := main.Start(tunnelController.SetupSignalHandlerForTunnelOperator()); err != nil {
			klog.Errorf("unable to start tunnel controller: %s", err)
			os.Exit(1)
		}
	case "tunnelEndpointCreator-operator":
		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			MapperProvider:     mapperUtils.LiqoMapperProvider(scheme),
			Scheme:             scheme,
			MetricsBindAddress: metricsAddr,
		})
		if err != nil {
			klog.Errorf("unable to get manager: %s", err)
			os.Exit(1)
		}
		clientset := kubernetes.NewForConfigOrDie(mgr.GetConfig())
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
			IPManager:                  ipam,
			RetryTimeout:               30 * time.Second,
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
		go r.StartServiceWatcher()
		go r.StartSecretWatcher()
		klog.Info("starting manager as tunnelEndpointCreator-operator")
		if err := mgr.Start(r.SetupSignalHandlerForTunEndCreator()); err != nil {
			klog.Errorf("an error occurred while starting manager: %s", err)
			os.Exit(1)
		}
	}
}

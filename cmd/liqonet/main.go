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
	route_operator "github.com/liqotech/liqo/internal/liqonet/route-operator"
	tunnel_operator "github.com/liqotech/liqo/internal/liqonet/tunnel-operator"
	"github.com/liqotech/liqo/internal/liqonet/tunnelEndpointCreator"
	liqonetOperator "github.com/liqotech/liqo/pkg/liqonet"
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	"github.com/liqotech/liqo/pkg/mapperUtils"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
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
	flag.StringVar(&runAs, "run-as", "tunnel-operator", "The accepted values are: liqo-gateway, liqo-route, tunnelEndpointCreator-operator. The default value is \"tunnel-operator\"")
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
	// +kubebuilder:scaffold:builder
	clientset := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	switch runAs {
	case route_operator.OperatorName:
		wgc, err := wireguard.NewWgClient()
		if err != nil {
			klog.Errorf("an error occurred while creating wireguard client: %v", err)
			os.Exit(1)
		}
		r, err := route_operator.NewRouteController(mgr, wgc, wireguard.NewNetLinker())
		if err != nil {
			klog.Errorf("an error occurred while creating the route operator -> %v", err)
			os.Exit(1)
		}
		r.StartPodWatcher()
		r.StartServiceWatcher()
		if err = r.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup controller: %s", err)
			os.Exit(1)
		}
		if err := mgr.Start(r.SetupSignalHandlerForRouteOperator()); err != nil {
			klog.Errorf("unable to start controller: %s", err)
			os.Exit(1)
		}
	case tunnel_operator.OperatorName:
		wgc, err := wireguard.NewWgClient()
		if err != nil {
			klog.Errorf("an error occurred while creating wireguard client: %v", err)
			os.Exit(1)
		}
		tc, err := tunnel_operator.NewTunnelController(mgr, wgc, wireguard.NewNetLinker())
		if err != nil {
			klog.Errorf("an error occurred while creating the tunnel controller: %v", err)
			os.Exit(1)
		}
		//starting configuration watcher
		config, err := ctrl.GetConfig()
		if err != nil {
			klog.Error(err)
			os.Exit(2)
		}
		tc.WatchConfiguration(config, &clusterConfig.GroupVersion)
		tc.StartPodWatcher()
		tc.StartServiceWatcher()
		if err := tc.CreateAndEnsureIPTablesChains(tc.DefaultIface); err != nil {
			klog.Errorf("an error occurred while creating iptables handler: %v", err)
			os.Exit(1)
		}
		if err = tc.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup tunnel controller: %s", err)
			os.Exit(1)
		}
		klog.Info("Starting manager as Tunnel-Operator")
		if err := mgr.Start(tc.SetupSignalHandlerForTunnelOperator()); err != nil {
			klog.Errorf("unable to start tunnel controller: %s", err)
			os.Exit(1)
		}

	case "tunnelEndpointCreator-operator":
		dynClient := dynamic.NewForConfigOrDie(mgr.GetConfig())
		ipam := liqonetOperator.NewIPAM()
		err = ipam.Init(liqonetOperator.Pools, dynClient)
		if err != nil {
			klog.Errorf("cannot init IPAM:%s", err.Error())
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

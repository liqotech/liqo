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
	"github.com/netgroup-polito/dronev2/api/tunnel-endpoint/v1"
	"github.com/vishvananda/netlink"
	"k8s.io/client-go/kubernetes"
	"os"

	"github.com/netgroup-polito/dronev2/internal/dronet-operator"

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
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = v1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var runAsRouteOperator bool

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&runAsRouteOperator, "run-as-route-operator", false,
		"Runs the controller as Route-Operator, the default value is false and will run as Tunnel-Operator")
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
	if runAsRouteOperator {
		if err = (&controllers.RouteController{
			Client:        mgr.GetClient(),
			Log:           ctrl.Log.WithName("controllers").WithName("Route"),
			Scheme:        mgr.GetScheme(),
			RouteOperator: runAsRouteOperator,
			ClientSet:     clientset,
			RoutesMap:     make(map[string]netlink.Route),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Route")
			os.Exit(1)
		}
		setupLog.Info("Starting manager as Route-Operator")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}else {
		if err = (&controllers.TunnelController{
			Client:        mgr.GetClient(),
			Log:           ctrl.Log.WithName("controllers").WithName("TunnelEndpoint"),
			Scheme:        mgr.GetScheme(),
			RouteOperator: runAsRouteOperator,
			ClientSet:     clientset,
			EndpointMap:   make(map[string]int),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "TunnelEndpoint")
			os.Exit(1)
		}
		setupLog.Info("Starting manager as Tunnel-Operator")
		setupLog.Info("Starting manager as Route-Operator")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}
}

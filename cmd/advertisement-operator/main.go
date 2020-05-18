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
	"errors"
	"flag"
	"k8s.io/apimachinery/pkg/types"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	dronetv1 "github.com/netgroup-polito/dronev2/api/tunnel-endpoint/v1"
	"github.com/netgroup-polito/dronev2/internal/advertisement-operator"
	// +kubebuilder:scaffold:imports
)

const (
	defaultNamespace = "default"
	defaultMetricsaddr = ":8080"
	defaultVKImage = "dronev2/virtual-kubelet"
	defaultInitVKImage = "dronev2/init-vkubelet"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = protocolv1.AddToScheme(scheme)

	_ = dronetv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr, localKubeconfig, foreignKubeconfig, clusterId string
	var gatewayIP, gatewayPrivateIP string
	var runsAsTunnelEndpointCreator bool
	var enableLeaderElection bool
	var kubeletNamespace string
	var kubeletImage string
	var initKubeletImage string
	var runsInKindEnv bool

	flag.StringVar(&metricsAddr, "metrics-addr", defaultMetricsaddr, "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&localKubeconfig, "local-kubeconfig", "", "The path to the kubeconfig of your local cluster.")
	flag.StringVar(&foreignKubeconfig, "foreign-kubeconfig", "", "The path to the kubeconfig of the foreign cluster.")
	flag.StringVar(&clusterId, "cluster-id", "", "The cluster ID of your cluster")
	flag.StringVar(&gatewayIP, "gateway-ip", "", "The IP address of the gateway node")
	flag.StringVar(&gatewayPrivateIP, "gateway-private-ip", "", "The private IP address of the gateway node")
	flag.StringVar(&kubeletNamespace, "kubelet-namespace", defaultNamespace, "Name of the namespace where Virtual kubelets will be spawned ( the namespace is default if not specified otherwise)")
	flag.StringVar(&kubeletImage, "kubelet-image", defaultVKImage, "The image of the virtual kubelet to be deployed")
	flag.StringVar(&initKubeletImage, "init-kubelet-image", defaultInitVKImage, "The image of the virtual kubelet init container to be deployed")
	flag.BoolVar(&runsAsTunnelEndpointCreator, "run-as-tunnel-endpoint-creator", false, "Runs the controller as TunnelEndpointCreator, the default value is false and will run as Advertisement-Operator")
	flag.BoolVar(&runsInKindEnv, "run-in-kind", false, "The cluster in which the controller runs is managed by kind")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	if clusterId == "" {
		setupLog.Error(errors.New("Cluster ID must be provided "), "")
		os.Exit(1)
	}

	if localKubeconfig != "" {
		if err := os.Setenv("KUBECONFIG", localKubeconfig); err != nil {
			os.Exit(1)
		}
	}

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

	if !runsAsTunnelEndpointCreator {
		if err = (&advertisement_operator.AdvertisementReconciler{
			KubeletNamespace: kubeletNamespace,
			Client:           mgr.GetClient(),
			Log:              ctrl.Log.WithName("controllers").WithName("Advertisement"),
			Scheme:           mgr.GetScheme(),
			EventsRecorder:   mgr.GetEventRecorderFor("AdvertisementOperator"),
			GatewayIP:        gatewayIP,
			GatewayPrivateIP: gatewayPrivateIP,
			KindEnvironment:  runsInKindEnv,
			VKImage: kubeletImage,
			InitVKImage: initKubeletImage,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Advertisement")
			os.Exit(1)
		}
		// +kubebuilder:scaffold:builder

		setupLog.Info("starting manager as advertisement-operator")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	} else {
		if err = (&advertisement_operator.TunnelEndpointCreator{
			Client:            mgr.GetClient(),
			Log:               ctrl.Log.WithName("controllers").WithName("TunnelEndpointCreator"),
			Scheme:            mgr.GetScheme(),
			TunnelEndpointMap: make(map[string]types.NamespacedName),
		}).SetupWithManager(mgr); err != nil {
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

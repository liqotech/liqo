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
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	liqonetv1 "github.com/liqoTech/liqo/api/tunnel-endpoint/v1"
	"github.com/liqoTech/liqo/internal/advertisement-operator"
	"github.com/liqoTech/liqo/pkg/csrApprover"
	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

const (
	defaultNamespace   = "default"
	defaultMetricsaddr = ":8080"
	defaultVKImage     = "liqo/virtual-kubelet"
	defaultInitVKImage = "liqo/init-vkubelet"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = protocolv1.AddToScheme(scheme)

	_ = liqonetv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr, localKubeconfig, foreignKubeconfig, clusterId string
	var enableLeaderElection bool
	var kubeletNamespace, kubeletImage, initKubeletImage string
	var runsInKindEnv bool

	flag.StringVar(&metricsAddr, "metrics-addr", defaultMetricsaddr, "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&localKubeconfig, "local-kubeconfig", "", "The path to the kubeconfig of your local cluster.")
	flag.StringVar(&foreignKubeconfig, "foreign-kubeconfig", "", "The path to the kubeconfig of the foreign cluster.")
	flag.StringVar(&clusterId, "cluster-id", "", "The cluster ID of your cluster")
	flag.StringVar(&kubeletNamespace, "kubelet-namespace", defaultNamespace, "Name of the namespace where Virtual kubelets will be spawned ( the namespace is default if not specified otherwise)")
	flag.StringVar(&kubeletImage, "kubelet-image", defaultVKImage, "The image of the virtual kubelet to be deployed")
	flag.StringVar(&initKubeletImage, "init-kubelet-image", defaultInitVKImage, "The image of the virtual kubelet init container to be deployed")
	flag.BoolVar(&runsInKindEnv, "run-in-kind", false, "The cluster in which the controller runs is managed by kind")
	flag.Parse()

	if clusterId == "" {
		klog.Error("Cluster ID must be provided")
		os.Exit(1)
	}

	if localKubeconfig != "" {
		if err := os.Setenv("KUBECONFIG", localKubeconfig); err != nil {
			os.Exit(1)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	// New Client For CSR Auto-approval
	config := ctrl.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	go csrApprover.WatchCSR(clientset, "virtual-kubelet=true")

	// get the number of already accepted advertisements
	advClient, err := protocolv1.CreateAdvertisementClient(localKubeconfig, nil)
	if err != nil {
		klog.Errorln(err, "unable to create local client for Advertisement")
		os.Exit(1)
	}
	var acceptedAdv int32
	advList, err := advClient.Resource("advertisements").List(metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
	} else {
		for _, adv := range advList.(*protocolv1.AdvertisementList).Items {
			if adv.Status.AdvertisementStatus == "ACCEPTED" {
				acceptedAdv++
			}
		}
	}

	r := &advertisement_operator.AdvertisementReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		EventsRecorder:   mgr.GetEventRecorderFor("AdvertisementOperator"),
		KindEnvironment:  runsInKindEnv,
		KubeletNamespace: kubeletNamespace,
		VKImage:          kubeletImage,
		InitVKImage:      initKubeletImage,
		HomeClusterId:    clusterId,
		AcceptedAdvNum:   acceptedAdv,
		AdvClient:        advClient,
	}

	if err = r.SetupWithManager(mgr); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	r.WatchConfiguration(localKubeconfig)

	klog.Info("starting manager as advertisement-operator")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

}

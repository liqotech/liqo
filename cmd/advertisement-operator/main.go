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
	"context"
	"flag"
	"os"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	advop "github.com/liqotech/liqo/internal/advertisementoperator"
	resourceRequestOperator "github.com/liqotech/liqo/internal/resource-request-operator"
	"github.com/liqotech/liqo/pkg/clusterid"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	resourceoffercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/resourceoffer-controller"
	"github.com/liqotech/liqo/pkg/mapperUtils"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	"github.com/liqotech/liqo/pkg/vkMachinery/csr"
	// +kubebuilder:scaffold:imports
)

const (
	defaultNamespace   = "liqo"
	defaultMetricsaddr = ":8080"
	defaultVKImage     = "liqo/virtual-kubelet"
	defaultInitVKImage = "liqo/init-virtual-kubelet"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = advtypes.AddToScheme(scheme)

	_ = netv1alpha1.AddToScheme(scheme)

	_ = discoveryv1alpha1.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr, localKubeconfig, clusterId string
	var probeAddr string
	var enableLeaderElection bool
	var kubeletNamespace, kubeletImage, initKubeletImage string
	var resyncPeriod int64

	flag.StringVar(&metricsAddr, "metrics-addr", defaultMetricsaddr, "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection,
		"enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	flag.Int64Var(&resyncPeriod, "resyncPeriod", int64(10*time.Hour), "Period after that operators and informers will requeue events.")
	flag.StringVar(&localKubeconfig, "local-kubeconfig", "", "The path to the kubeconfig of your local cluster.")
	flag.StringVar(&clusterId, "cluster-id", "", "The cluster ID of your cluster")
	flag.StringVar(&kubeletNamespace,
		"kubelet-namespace", defaultNamespace,
		"Name of the namespace where Virtual kubelets will be spawned ( the namespace is default if not specified otherwise)")
	flag.StringVar(&kubeletImage, "kubelet-image", defaultVKImage, "The image of the virtual kubelet to be deployed")
	flag.StringVar(&initKubeletImage,
		"init-kubelet-image", defaultInitVKImage,
		"The image of the virtual kubelet init container to be deployed")

	klog.InitFlags(nil)
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
		MapperProvider:         mapperUtils.LiqoMapperProvider(scheme),
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "66cf253f.liqo.io",
		Port:                   9443,
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

	// get the number of already accepted advertisements
	advClient, err := advtypes.CreateAdvertisementClient(localKubeconfig, nil, true, nil)
	if err != nil {
		klog.Errorln(err, "unable to create local client for Advertisement")
		os.Exit(1)
	}
	var acceptedAdv int32
	advList, err := advClient.Resource("advertisements").List(&metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
	} else {
		for index := range advList.(*advtypes.AdvertisementList).Items {
			if advList.(*advtypes.AdvertisementList).Items[index].Status.AdvertisementStatus == advtypes.AdvertisementAccepted {
				acceptedAdv++
			}
		}
	}

	discoveryConfig, err := crdclient.NewKubeconfig(localKubeconfig, &discoveryv1alpha1.GroupVersion, nil)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	discoveryClient, err := crdclient.NewFromConfig(discoveryConfig)
	if err != nil {
		klog.Errorln(err, "unable to create local client for Discovery")
		os.Exit(1)
	}

	clusterID, err := clusterid.NewClusterIDFromClient(discoveryClient.Client())
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	advertisementReconciler := &advop.AdvertisementReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		EventsRecorder:   mgr.GetEventRecorderFor("AdvertisementOperator"),
		KubeletNamespace: kubeletNamespace,
		VKImage:          kubeletImage,
		InitVKImage:      initKubeletImage,
		HomeClusterId:    clusterId,
		AcceptedAdvNum:   acceptedAdv,
		AdvClient:        advClient,
		DiscoveryClient:  discoveryClient,
		RetryTimeout:     time.Duration(resyncPeriod),
	}

	if err = advertisementReconciler.SetupWithManager(mgr); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	newBroadcaster := &resourceRequestOperator.Broadcaster{}

	if err := newBroadcaster.SetupBroadcaster(clientset, time.Duration(resyncPeriod)); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	resourceRequestReconciler := &resourceRequestOperator.ResourceRequestReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		ClusterID:   clusterId,
		Broadcaster: newBroadcaster,
	}

	if err = resourceRequestReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	resourceOfferReconciler := resourceoffercontroller.NewResourceOfferController(
		mgr, clusterID, time.Duration(resyncPeriod), kubeletImage, initKubeletImage)
	if err = resourceOfferReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Error(err, " unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Error(err, " unable to set up ready check")
		os.Exit(1)
	}

	var wg = &sync.WaitGroup{}
	client, err := advertisementReconciler.InitCRDClient(localKubeconfig)
	if err != nil {
		os.Exit(1)
	}
	wg.Add(5)
	ctx, cancel := context.WithCancel(context.Background())
	go advertisementReconciler.CleanOldAdvertisements(ctx.Done(), wg)
	go csr.WatchCSR(ctx, clientset, labels.SelectorFromSet(vkMachinery.CsrLabels).String(), time.Duration(resyncPeriod), wg)
	// TODO: this configuration watcher will be refactored before the release 0.3
	go advertisementReconciler.WatchConfiguration(localKubeconfig, client, wg)
	go newBroadcaster.WatchConfiguration(localKubeconfig, client, wg)
	go resourceOfferReconciler.WatchConfiguration(localKubeconfig, client, wg)
	newBroadcaster.StartBroadcaster(ctx, wg)

	klog.Info("starting manager as advertisementoperator")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	close(client.Stop)
	cancel()
	wg.Wait()
}

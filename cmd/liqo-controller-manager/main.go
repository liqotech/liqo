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

	capsulev1alpha1 "github.com/clastix/capsule/api/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	resourceRequestOperator "github.com/liqotech/liqo/internal/resource-request-operator"
	"github.com/liqotech/liqo/pkg/clusterid"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	namectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespace-controller"
	mapsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespaceMap-controller"
	nsoffctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespaceOffloading-controller"
	offloadingctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloadingStatus-controller"
	resourceoffercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/resourceoffer-controller"
	virtualNodectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/virtualNode-controller"
	"github.com/liqotech/liqo/pkg/mapperUtils"
	errorsmanagement "github.com/liqotech/liqo/pkg/utils/errorsManagement"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	"github.com/liqotech/liqo/pkg/vkMachinery/csr"
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

	_ = sharingv1alpha1.AddToScheme(scheme)
	_ = netv1alpha1.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = offloadingv1alpha1.AddToScheme(scheme)
	_ = virtualkubeletv1alpha1.AddToScheme(scheme)

	_ = capsulev1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr, localKubeconfig, clusterId string
	var probeAddr string
	var enableLeaderElection bool
	var debug bool
	var liqoNamespace, kubeletImage, initKubeletImage string
	var resyncPeriod int64
	var offloadingStatusControllerRequeueTime int64
	var offerUpdateThreshold uint64
	var namespaceMapControllerRequeueTime int64

	flag.BoolVar(&debug, "debug", false, "flag to enable the debug mode")
	flag.StringVar(&metricsAddr, "metrics-addr", defaultMetricsaddr, "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection,
		"enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Uint64Var(&offerUpdateThreshold, "offer-update-threshold-perc", uint64(5),
		"Set the threshold percentage of quantity of resources modified which triggers the resourceOffer update.")
	flag.Int64Var(&resyncPeriod, "resyncPeriod", int64(10*time.Hour), "Period after that operators and informers will requeue events.")
	flag.Int64Var(&offloadingStatusControllerRequeueTime, "offloadingStatusControllerRequeueTime", int64(10*time.Second),
		"Period after that the offloadingStatus Controller is awaken on every NamespaceOffloading to set its status.")
	flag.Int64Var(&namespaceMapControllerRequeueTime, "namespaceMapControllerRequeueTime", int64(30*time.Second),
		"Period after that the namespaceMap Controller is awaken on every NamespaceMap to enforce DesiredMappings.")
	flag.StringVar(&localKubeconfig, "local-kubeconfig", "", "The path to the kubeconfig of your local cluster.")
	flag.StringVar(&clusterId, "cluster-id", "", "The cluster ID of your cluster")
	flag.StringVar(&liqoNamespace,
		"liqo-namespace", defaultNamespace,
		"Name of the namespace where Virtual kubelets will be spawned ( the namespace is default if not specified otherwise)")
	flag.StringVar(&kubeletImage, "kubelet-image", defaultVKImage, "The image of the virtual kubelet to be deployed")
	flag.StringVar(&initKubeletImage,
		"init-kubelet-image", defaultInitVKImage,
		"The image of the virtual kubelet init container to be deployed")

	klog.InitFlags(nil)
	flag.Parse()

	errorsmanagement.SetDebug(debug)

	if clusterId == "" {
		klog.Error("Cluster ID must be provided")
		os.Exit(1)
	}

	if offerUpdateThreshold > 100 {
		klog.Error("offerUpdateThreshold exceeds 100")
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

	newBroadcaster := &resourceRequestOperator.Broadcaster{}
	updater := &resourceRequestOperator.OfferUpdater{}
	updater.Setup(clusterId, mgr.GetScheme(), newBroadcaster, mgr.GetClient())
	if err := newBroadcaster.SetupBroadcaster(clientset, updater, time.Duration(resyncPeriod), offerUpdateThreshold); err != nil {
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
		mgr, clusterID, time.Duration(resyncPeriod), kubeletImage, initKubeletImage, liqoNamespace)
	if err = resourceOfferReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	namespaceReconciler := &namectrl.NamespaceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = namespaceReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	virtualNodeReconciler := &virtualNodectrl.VirtualNodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = virtualNodeReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	namespaceMapReconciler := &mapsctrl.NamespaceMapReconciler{
		Client:                mgr.GetClient(),
		RemoteClients:         make(map[string]kubernetes.Interface),
		LocalClusterID:        clusterId,
		IdentityManagerClient: clientset,
		RequeueTime:           time.Duration(namespaceMapControllerRequeueTime),
	}

	if err = namespaceMapReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	offloadingStatusReconciler := &offloadingctrl.OffloadingStatusReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		RequeueTime: time.Duration(offloadingStatusControllerRequeueTime),
	}

	if err = offloadingStatusReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	namespaceOffloadingReconciler := &nsoffctrl.NamespaceOffloadingReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		LocalClusterID: clusterId,
	}

	if err = namespaceOffloadingReconciler.SetupWithManager(mgr); err != nil {
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
	config, err = crdclient.NewKubeconfig(localKubeconfig, &configv1alpha1.GroupVersion, nil)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	client, err := crdclient.NewFromConfig(config)
	if err != nil {
		os.Exit(1)
	}
	wg.Add(5)
	ctx, cancel := context.WithCancel(context.Background())
	go csr.WatchCSR(ctx, clientset, labels.SelectorFromSet(vkMachinery.CsrLabels).String(), time.Duration(resyncPeriod), wg)
	// TODO: this configuration watcher will be refactored before the release 0.3
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

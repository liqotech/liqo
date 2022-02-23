// Copyright 2019-2022 The Liqo Authors
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
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	foreignclusteroperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/foreign-cluster-operator"
	namectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespace-controller"
	nsoffctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespaceOffloading-controller"
	mapsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespacemap-controller"
	offloadingctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloadingStatus-controller"
	resourceRequestOperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller"
	resourcemonitors "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller/resource-monitors"
	resourceoffercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/resourceoffer-controller"
	searchdomainoperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/search-domain-operator"
	shadowpodctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/shadowpod-controller"
	liqostorageprovisioner "github.com/liqotech/liqo/pkg/liqo-controller-manager/storageprovisioner"
	virtualNodectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/virtualNode-controller"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	"github.com/liqotech/liqo/pkg/vkMachinery/csr"
	"github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

const (
	defaultNamespace   = "liqo"
	defaultVKImage     = "liqo/virtual-kubelet"
	defaultInitVKImage = "liqo/init-virtual-kubelet"
)

var (
	scheme = runtime.NewScheme()

	healthFlag = false
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = sharingv1alpha1.AddToScheme(scheme)
	_ = netv1alpha1.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = offloadingv1alpha1.AddToScheme(scheme)
	_ = virtualkubeletv1alpha1.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

func probe(req *http.Request) error {
	if healthFlag {
		return nil
	}
	return fmt.Errorf("controller manager not yet configured")
}

func main() {
	var clusterLabels argsutils.StringMap
	var kubeletExtraAnnotations, kubeletExtraLabels argsutils.StringMap
	var kubeletExtraArgs argsutils.StringList
	var nodeExtraAnnotations, nodeExtraLabels argsutils.StringMap
	var kubeletCPURequests, kubeletCPULimits = argsutils.NewQuantity("250m"), argsutils.NewQuantity("1000m")
	var kubeletRAMRequests, kubeletRAMLimits = argsutils.NewQuantity("100M"), argsutils.NewQuantity("250M")

	metricsAddr := flag.String("metrics-address", ":8080", "The address the metric endpoint binds to")
	probeAddr := flag.String("health-probe-address", ":8081", "The address the health probe endpoint binds to")

	// Global parameters
	resyncPeriod := flag.Duration("resync-period", 10*time.Hour, "The resync period for the informers")
	clusterIdentityFlags := argsutils.NewClusterIdentityFlags(true, nil)
	liqoNamespace := flag.String("liqo-namespace", defaultNamespace,
		"Name of the namespace where the liqo components are running")
	foreignClusterWorkers := flag.Uint("foreign-cluster-workers", 1, "The number of workers used to reconcile ForeignCluster resources.")
	shadowPodWorkers := flag.Int("shadow-pod-ctrl-workers", 10, "The number of workers used to reconcile ShadowPod resources.")

	// Discovery parameters
	authServiceAddressOverride := flag.String(consts.AuthServiceAddressOverrideParameter, "",
		"The address the authentication service is reachable from foreign clusters (automatically retrieved if not set")
	authServicePortOverride := flag.String(consts.AuthServicePortOverrideParameter, "",
		"The port the authentication service is reachable from foreign clusters (automatically retrieved if not set")
	autoJoin := flag.Bool("auto-join-discovered-clusters", true, "Whether to automatically peer with discovered clusters")

	// Resource sharing parameters
	externalResourceMonitorAddress := flag.String(consts.ExternalResourceMonitorParameter, "",
		"The address of a resource monitor service (default: monitor local resources)")
	flag.Var(&clusterLabels, consts.ClusterLabelsParameter,
		"The set of labels which characterizes the local cluster when exposed remotely as a virtual node")
	resourceSharingPercentage := argsutils.Percentage{Val: 50}
	flag.Var(&resourceSharingPercentage, "resource-sharing-percentage",
		"The amount (in percentage) of cluster resources possibly shared with foreign clusters (ignored when using an external resource monitor)")
	enableIncomingPeering := flag.Bool("enable-incoming-peering", true,
		"Enable remote clusters to establish an incoming peering with the local cluster (can be overwritten on a per foreign cluster basis)")
	offerDisableAutoAccept := flag.Bool("offer-disable-auto-accept", false, "Disable the automatic acceptance of resource offers")
	offerUpdateThreshold := argsutils.Percentage{Val: 5}
	flag.Var(&offerUpdateThreshold, "offer-update-threshold-percentage",
		"The threshold (in percentage) of resources quantity variation which triggers a ResourceOffer update")

	// Namespace management parameters
	offloadingStatusControllerRequeueTime := flag.Duration("offloading-status-requeue-period", 10*time.Second,
		"Period after that the offloading status controller is awaken on every NamespaceOffloading to set its status")

	// Virtual-kubelet parameters
	kubeletImage := flag.String("kubelet-image", defaultVKImage, "The image of the virtual kubelet to be deployed")
	initKubeletImage := flag.String("init-kubelet-image", defaultInitVKImage,
		"The image of the virtual kubelet init container to be deployed")
	disableKubeletCertGeneration := flag.Bool("disable-kubelet-certificate-generation", false,
		"Whether to disable the virtual kubelet certificate generation by means of an init container (used for logs/exec capabilities)")
	flag.Var(&kubeletExtraAnnotations, "kubelet-extra-annotations", "Extra annotations to add to the Virtual Kubelet Deployments and Pods")
	flag.Var(&kubeletExtraLabels, "kubelet-extra-labels", "Extra labels to add to the Virtual Kubelet Deployments and Pods")
	flag.Var(&kubeletExtraArgs, "kubelet-extra-args", "Extra arguments to add to the Virtual Kubelet Deployments and Pods")
	flag.Var(&kubeletCPURequests, "kubelet-cpu-requests", "CPU requests assigned to the Virtual Kubelet Pod")
	flag.Var(&kubeletCPULimits, "kubelet-cpu-limits", "CPU limits assigned to the Virtual Kubelet Pod")
	flag.Var(&kubeletRAMRequests, "kubelet-ram-requests", "RAM requests assigned to the Virtual Kubelet Pod")
	flag.Var(&kubeletRAMLimits, "kubelet-ram-limits", "RAM limits assigned to the Virtual Kubelet Pod")
	flag.Var(&nodeExtraAnnotations, "node-extra-annotations", "Extra annotations to add to the Virtual Node")
	flag.Var(&nodeExtraLabels, "node-extra-labels", "Extra labels to add to the Virtual Node")

	// Storage Provisioner parameters
	enableStorage := flag.Bool("enable-storage", false, "enable the liqo virtual storage class")
	virtualStorageClassName := flag.String("virtual-storage-class-name", "liqo", "Name of the virtual storage class")
	realStorageClassName := flag.String("real-storage-class-name", "", "Name of the real storage class to use for the actual volumes")
	storageNamespace := flag.String("storage-namespace", "liqo-storage", "Namespace where the liqo storage-related resources are stored")

	liqoerrors.InitFlags(nil)
	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	clusterIdentity := clusterIdentityFlags.ReadOrDie()

	ctx := ctrl.SetupSignalHandler()

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	// Create a label selector to filter out the events for pods not managed by a ShadowPod,
	// as those are the only ones we are interested in to implement the resiliency mechanism.
	podsLabelRequirement, err := labels.NewRequirement(consts.ManagedByLabelKey, selection.Equals, []string{consts.ManagedByShadowPodValue})
	utilruntime.Must(err)

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider:         mapper.LiqoMapperProvider(scheme),
		Scheme:                 scheme,
		MetricsBindAddress:     *metricsAddr,
		HealthProbeBindAddress: *probeAddr,
		LeaderElection:         false,
		LeaderElectionID:       "66cf253f.liqo.io",
		Port:                   9443,
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: cache.SelectorsByObject{
				&corev1.Pod{}: {
					Label: labels.NewSelector().Add(*podsLabelRequirement),
				},
			},
		}),
	})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	if err = mgr.AddReadyzCheck("/readyz", probe); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	namespaceManager := tenantnamespace.NewTenantNamespaceManager(clientset)
	idManager := identitymanager.NewCertificateIdentityManager(clientset, clusterIdentity, namespaceManager)

	// populate the lists of ClusterRoles to bind in the different peering states
	permissions, err := peeringroles.GetPeeringPermission(ctx, clientset)
	if err != nil {
		klog.Fatalf("Unable to populate peering permission: %w", err)
	}

	// Configure the tranports used for the intaction with the remote authentication service.
	// Using the same transport allows to reuse the underlying TCP/TLS connections when contacting the same destinations,
	// and reduce the overall handshake overhead, especially with high-latency links.
	secureTransport := &http.Transport{IdleConnTimeout: 1 * time.Minute}
	insecureTransport := &http.Transport{IdleConnTimeout: 1 * time.Minute, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}

	// Setup operators
	searchDomainReconciler := &searchdomainoperator.SearchDomainReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ResyncPeriod:      *resyncPeriod,
		LocalCluster:      clusterIdentity,
		InsecureTransport: insecureTransport,
	}
	if err = searchDomainReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	foreignClusterReconciler := &foreignclusteroperator.ForeignClusterReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		LiqoNamespace: *liqoNamespace,

		ResyncPeriod:               *resyncPeriod,
		HomeCluster:                clusterIdentity,
		AuthServiceAddressOverride: *authServiceAddressOverride,
		AuthServicePortOverride:    *authServicePortOverride,
		AutoJoin:                   *autoJoin,

		NamespaceManager:  namespaceManager,
		IdentityManager:   idManager,
		PeeringPermission: *permissions,

		SecureTransport:   secureTransport,
		InsecureTransport: insecureTransport,
	}
	if err = foreignClusterReconciler.SetupWithManager(mgr, *foreignClusterWorkers); err != nil {
		klog.Fatal(err)
	}

	var resourceRequestReconciler *resourceRequestOperator.ResourceRequestReconciler
	var monitor resourcemonitors.ResourceReader
	if *externalResourceMonitorAddress != "" {
		externalMonitor, err := resourcemonitors.NewExternalMonitor(*externalResourceMonitorAddress)
		if err != nil {
			klog.Fatal(err)
		}
		monitor = externalMonitor
	} else {
		localMonitor := resourcemonitors.NewLocalMonitor(ctx, clientset, *resyncPeriod)
		monitor = &resourcemonitors.ResourceScaler{
			Provider: localMonitor,
			Factor:   float32(resourceSharingPercentage.Val) / 100.,
		}
	}
	offerUpdater := resourceRequestOperator.NewOfferUpdater(ctx, mgr.GetClient(), clusterIdentity,
		clusterLabels.StringMap, monitor, uint(offerUpdateThreshold.Val), *realStorageClassName, *enableStorage)
	resourceRequestReconciler = &resourceRequestOperator.ResourceRequestReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		HomeCluster:           clusterIdentity,
		OfferUpdater:          offerUpdater,
		EnableIncomingPeering: *enableIncomingPeering,
	}

	if err = resourceRequestReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	virtualKubeletOpts := &forge.VirtualKubeletOpts{
		ContainerImage:        *kubeletImage,
		InitContainerImage:    *initKubeletImage,
		DisableCertGeneration: *disableKubeletCertGeneration,
		ExtraAnnotations:      kubeletExtraAnnotations.StringMap,
		ExtraLabels:           kubeletExtraLabels.StringMap,
		ExtraArgs:             kubeletExtraArgs.StringList,
		NodeExtraAnnotations:  nodeExtraAnnotations,
		NodeExtraLabels:       nodeExtraLabels,
		RequestsCPU:           kubeletCPURequests.Quantity,
		RequestsRAM:           kubeletRAMRequests.Quantity,
		LimitsCPU:             kubeletCPULimits.Quantity,
		LimitsRAM:             kubeletRAMLimits.Quantity,
	}

	resourceOfferReconciler := resourceoffercontroller.NewResourceOfferController(
		mgr, clusterIdentity, *resyncPeriod, *liqoNamespace, virtualKubeletOpts, *offerDisableAutoAccept)
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
		Client: mgr.GetClient(),
	}

	if err = namespaceMapReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	offloadingStatusReconciler := &offloadingctrl.OffloadingStatusReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		RequeueTime: *offloadingStatusControllerRequeueTime,
	}

	if err = offloadingStatusReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	namespaceOffloadingReconciler := &nsoffctrl.NamespaceOffloadingReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		LocalCluster: clusterIdentity,
	}

	if err = namespaceOffloadingReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	shadowPodReconciler := &shadowpodctrl.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = shadowPodReconciler.SetupWithManager(mgr, *shadowPodWorkers); err != nil {
		klog.Fatal(err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Error(err, " unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Error(err, " unable to set up ready check")
		os.Exit(1)
	}

	// Start the handler to approve the virtual kubelet certificate signing requests.
	csrWatcher := csr.NewWatcher(clientset, *resyncPeriod, labels.SelectorFromSet(vkMachinery.CsrLabels))
	csrWatcher.RegisterHandler(csr.ApproverHandler(clientset, "LiqoApproval", "This CSR was approved by Liqo"))
	csrWatcher.Start(ctx)

	if err = mgr.Add(offerUpdater); err != nil {
		klog.Fatal(err)
	}

	if enableStorage != nil && *enableStorage {
		liqoProvisioner, err := liqostorageprovisioner.NewLiqoLocalStorageProvisioner(ctx, mgr.GetClient(),
			*virtualStorageClassName, *storageNamespace, *realStorageClassName)
		if err != nil {
			klog.Errorf("unable to start the liqo storage provisioner: %v", err)
			os.Exit(1)
		}

		provisionController := controller.NewProvisionController(clientset, consts.StorageProvisionerName, liqoProvisioner,
			controller.LeaderElection(false),
		)

		if err = mgr.Add(liqostorageprovisioner.StorageControllerRunnable{
			Ctrl: provisionController,
		}); err != nil {
			klog.Fatal(err)
		}
	}

	healthFlag = true

	klog.Info("starting manager as controller manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

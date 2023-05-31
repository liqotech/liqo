// Copyright 2019-2023 The Liqo Authors
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

// Package main contains the main function for the Liqo controller manager.
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	certificates "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	foreignclusteroperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/foreign-cluster-operator"
	mapsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespacemap-controller"
	nsoffctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespaceoffloading-controller"
	nodefailurectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/nodefailure-controller"
	resourceRequestOperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller"
	resourcemonitors "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller/resource-monitors"
	resourceoffercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/resourceoffer-controller"
	shadowepsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/shadowendpointslice-controller"
	shadowpodctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/shadowpod-controller"
	liqostorageprovisioner "github.com/liqotech/liqo/pkg/liqo-controller-manager/storageprovisioner"
	virtualnodectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/virtualnode-controller"
	shadowpodswh "github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/shadowpod"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/csr"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
	"github.com/liqotech/liqo/pkg/utils/indexer"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	"github.com/liqotech/liqo/pkg/vkMachinery/forge"
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
}

func main() {
	var clusterLabels argsutils.StringMap
	var kubeletExtraAnnotations, kubeletExtraLabels argsutils.StringMap
	var kubeletExtraArgs argsutils.StringList
	var nodeExtraAnnotations, nodeExtraLabels argsutils.StringMap
	var kubeletCPURequests, kubeletCPULimits argsutils.Quantity
	var kubeletRAMRequests, kubeletRAMLimits argsutils.Quantity
	var kubeletMetricsAddress string
	var kubeletMetricsEnabled bool

	webhookPort := flag.Uint("webhook-port", 9443, "The port the webhook server binds to")
	metricsAddr := flag.String("metrics-address", ":8080", "The address the metric endpoint binds to")
	probeAddr := flag.String("health-probe-address", ":8081", "The address the health probe endpoint binds to")

	// ShadowPods webhook
	enableResourceValidation := flag.Bool("enable-resource-enforcement", false,
		"Enforce offerer-side that offloaded pods do not exceed offered resources (based on container limits)")
	refreshInterval := flag.Duration("resource-validator-refresh-interval",
		5*time.Minute, "The interval at which the resource validator cache is refreshed")

	// Leader election
	leaderElection := flag.Bool("enable-leader-election", false, "Enable leader election for controller manager")

	// Global parameters
	resyncPeriod := flag.Duration("resync-period", 10*time.Hour, "The resync period for the informers")
	clusterIdentityFlags := argsutils.NewClusterIdentityFlags(true, nil)
	liqoNamespace := flag.String("liqo-namespace", consts.DefaultLiqoNamespace,
		"Name of the namespace where the liqo components are running")
	foreignClusterWorkers := flag.Int("foreign-cluster-workers", 1, "The number of workers used to reconcile ForeignCluster resources.")
	shadowPodWorkers := flag.Int("shadow-pod-ctrl-workers", 10, "The number of workers used to reconcile ShadowPod resources.")
	shadowEndpointSliceWorkers := flag.Int("shadow-endpointslice-ctrl-workers", 10,
		"The number of workers used to reconcile ShadowEndpointSlice resources.")
	disableInternalNetwork := flag.Bool("disable-internal-network", false, "Disable the creation of the internal network")
	foreignClusterPingInterval := flag.Duration("foreign-cluster-ping-interval", 15*time.Second,
		"The frequency of the ForeignCluster API server readiness check. Set 0 to disable the check")
	foreignClusterPingTimeout := flag.Duration("foreign-cluster-ping-timeout", 5*time.Second,
		"The timeout of the ForeignCluster API server readiness check")

	// Discovery parameters
	autoJoin := flag.Bool("auto-join-discovered-clusters", true, "Whether to automatically peer with discovered clusters")

	// Resource sharing parameters
	resourcePluginAddress := flag.String(consts.ResourcePluginAddressParameter, "",
		"The address of a resource plugin service (default: monitor local resources)")
	flag.Var(&clusterLabels, consts.ClusterLabelsParameter,
		"The set of labels which characterizes the local cluster when exposed remotely as a virtual node")
	resourceSharingPercentage := argsutils.Percentage{Val: 50}
	flag.Var(&resourceSharingPercentage, "resource-sharing-percentage",
		"The amount (in percentage) of cluster resources possibly shared with foreign clusters (ignored when using an external resource monitor)")
	enableIncomingPeering := flag.Bool("enable-incoming-peering", true,
		"Enable remote clusters to establish an incoming peering with the local cluster (can be overwritten on a per foreign cluster basis)")
	offerDisableAutoAccept := flag.Bool("offer-disable-auto-accept", false, "Disable the automatic acceptance of resource offers")
	offerUpdateThreshold := argsutils.Percentage{}
	flag.Var(&offerUpdateThreshold, "offer-update-threshold-percentage",
		"The threshold (in percentage) of resources quantity variation which triggers a ResourceOffer update")

	// Virtual-kubelet parameters
	kubeletImage := flag.String("kubelet-image", "ghcr.io/liqotech/virtual-kubelet", "The image of the virtual kubelet to be deployed")
	flag.Var(&kubeletExtraAnnotations, "kubelet-extra-annotations", "Extra annotations to add to the Virtual Kubelet Deployments and Pods")
	flag.Var(&kubeletExtraLabels, "kubelet-extra-labels", "Extra labels to add to the Virtual Kubelet Deployments and Pods")
	flag.Var(&kubeletExtraArgs, "kubelet-extra-args", "Extra arguments to add to the Virtual Kubelet Deployments and Pods")
	flag.Var(&kubeletCPURequests, "kubelet-cpu-requests", "CPU requests assigned to the Virtual Kubelet Pod")
	flag.Var(&kubeletCPULimits, "kubelet-cpu-limits", "CPU limits assigned to the Virtual Kubelet Pod")
	flag.Var(&kubeletRAMRequests, "kubelet-ram-requests", "RAM requests assigned to the Virtual Kubelet Pod")
	flag.Var(&kubeletRAMLimits, "kubelet-ram-limits", "RAM limits assigned to the Virtual Kubelet Pod")
	flag.StringVar(&kubeletMetricsAddress, "kubelet-metrics-address", vkMachinery.MetricsAddress, "The address the kubelet metrics endpoint binds to")
	flag.BoolVar(&kubeletMetricsEnabled, "kubelet-metrics-enabled", false, "Enable the kubelet metrics endpoint")
	flag.Var(&nodeExtraAnnotations, "node-extra-annotations", "Extra annotations to add to the Virtual Node")
	flag.Var(&nodeExtraLabels, "node-extra-labels", "Extra labels to add to the Virtual Node")
	kubeletIpamServer := flag.String("kubelet-ipam-server", "",
		"The address of the IPAM server to use for the virtual kubelet (set to empty string to disable IPAM)")

	// Storage Provisioner parameters
	enableStorage := flag.Bool("enable-storage", false, "enable the liqo virtual storage class")
	virtualStorageClassName := flag.String("virtual-storage-class-name", "liqo", "Name of the virtual storage class")
	realStorageClassName := flag.String("real-storage-class-name", "", "Name of the real storage class to use for the actual volumes")
	storageNamespace := flag.String("storage-namespace", "liqo-storage", "Namespace where the liqo storage-related resources are stored")

	// Node failure controller parameter
	enableNodeFailureController := flag.Bool("enable-node-failure-controller", false, "Enable the node failure controller")

	liqoerrors.InitFlags(nil)
	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	// Options for the virtual kubelet.
	virtualKubeletOpts := &forge.VirtualKubeletOpts{
		ContainerImage:       *kubeletImage,
		ExtraAnnotations:     kubeletExtraAnnotations.StringMap,
		ExtraLabels:          kubeletExtraLabels.StringMap,
		ExtraArgs:            kubeletExtraArgs.StringList,
		NodeExtraAnnotations: nodeExtraAnnotations,
		NodeExtraLabels:      nodeExtraLabels,
		RequestsCPU:          kubeletCPURequests.Quantity,
		RequestsRAM:          kubeletRAMRequests.Quantity,
		LimitsCPU:            kubeletCPULimits.Quantity,
		LimitsRAM:            kubeletRAMLimits.Quantity,
		IpamEndpoint:         *kubeletIpamServer,
		MetricsAddress:       kubeletMetricsAddress,
		MetricsEnabled:       kubeletMetricsEnabled,
	}

	clusterIdentity := clusterIdentityFlags.ReadOrDie()

	ctx := ctrl.SetupSignalHandler()

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	// Create a label selector to filter out the events for pods not managed by a ShadowPod,
	// as those are the only ones we are interested in to implement the resiliency mechanism.
	podsLabelRequirement, err := labels.NewRequirement(consts.ManagedByLabelKey, selection.Equals, []string{consts.ManagedByShadowPodValue})
	utilruntime.Must(err)

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider:                mapper.LiqoMapperProvider(scheme),
		Scheme:                        scheme,
		MetricsBindAddress:            *metricsAddr,
		HealthProbeBindAddress:        *probeAddr,
		LeaderElection:                *leaderElection,
		LeaderElectionID:              "66cf253f.ctrlmgr.liqo.io",
		LeaderElectionNamespace:       *liqoNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		Port:                          int(*webhookPort),
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

	// Register the healthiness probes.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Errorf("Unable to set up healthz probe: %v", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Errorf("Unable to set up readyz probe: %v", err)
		os.Exit(1)
	}

	spv := shadowpodswh.NewValidator(mgr.GetClient(), *enableResourceValidation)

	if err := indexer.IndexField(ctx, mgr, &corev1.Pod{}, indexer.FieldNodeNameFromPod, indexer.ExtractNodeName); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	clientset := kubernetes.NewForConfigOrDie(config)

	namespaceManager := tenantnamespace.NewCachedManager(ctx, clientset)
	idManager := identitymanager.NewCertificateIdentityManager(clientset, clusterIdentity, namespaceManager)

	// populate the lists of ClusterRoles to bind in the different peering states
	permissions, err := peeringroles.GetPeeringPermission(ctx, clientset)
	if err != nil {
		klog.Fatalf("Unable to populate peering permission: %v", err)
	}

	// Configure the tranports used for the intaction with the remote authentication service.
	// Using the same transport allows to reuse the underlying TCP/TLS connections when contacting the same destinations,
	// and reduce the overall handshake overhead, especially with high-latency links.
	secureTransport := &http.Transport{IdleConnTimeout: 1 * time.Minute}
	insecureTransport := &http.Transport{IdleConnTimeout: 1 * time.Minute, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}

	// Setup operators
	foreignClusterReconciler := &foreignclusteroperator.ForeignClusterReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		LiqoNamespace: *liqoNamespace,

		ResyncPeriod:           *resyncPeriod,
		HomeCluster:            clusterIdentity,
		AutoJoin:               *autoJoin,
		DisableInternalNetwork: *disableInternalNetwork,

		NamespaceManager:  namespaceManager,
		IdentityManager:   idManager,
		PeeringPermission: *permissions,

		SecureTransport:   secureTransport,
		InsecureTransport: insecureTransport,

		ForeignClusters: sync.Map{},

		APIServerCheckers: foreignclusteroperator.NewAPIServerCheckers(*foreignClusterPingInterval, *foreignClusterPingTimeout),
	}
	if err = foreignClusterReconciler.SetupWithManager(mgr, *foreignClusterWorkers); err != nil {
		klog.Fatal(err)
	}

	var resourceRequestReconciler *resourceRequestOperator.ResourceRequestReconciler
	var monitor resourcemonitors.ResourceReader
	if *resourcePluginAddress != "" {
		externalMonitor, err := resourcemonitors.NewExternalMonitor(ctx, *resourcePluginAddress, 3*time.Second)
		if err != nil {
			klog.Errorf("error on creating external resource monitor: %s", err)
			os.Exit(1)
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

	// this is a temporary solution to avoid deleting all those flags
	// and recreate them when the VirtualNode operator will be added.
	_ = &forge.VirtualKubeletOpts{
		ContainerImage:       "localhost:5001/virtual-kubelet",
		ExtraAnnotations:     kubeletExtraAnnotations.StringMap,
		ExtraLabels:          kubeletExtraLabels.StringMap,
		ExtraArgs:            kubeletExtraArgs.StringList,
		NodeExtraAnnotations: nodeExtraAnnotations,
		NodeExtraLabels:      nodeExtraLabels,
		RequestsCPU:          kubeletCPURequests.Quantity,
		RequestsRAM:          kubeletRAMRequests.Quantity,
		LimitsCPU:            kubeletCPULimits.Quantity,
		LimitsRAM:            kubeletRAMLimits.Quantity,
		IpamEndpoint:         *kubeletIpamServer,
		MetricsAddress:       kubeletMetricsAddress,
		MetricsEnabled:       kubeletMetricsEnabled,
	}

	resourceOfferReconciler := resourceoffercontroller.NewResourceOfferController(
		mgr, idManager, *resyncPeriod, *offerDisableAutoAccept)
	if err = resourceOfferReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	virtualNodeReconciler := &virtualnodectrl.VirtualNodeReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		EventsRecorder:        mgr.GetEventRecorderFor("virtualnode-controller"),
		HomeClusterIdentity:   &clusterIdentity,
		VirtualKubeletOptions: virtualKubeletOpts,
	}

	if err = virtualNodeReconciler.SetupWithManager(ctx, mgr); err != nil {
		klog.Fatal(err)
	}

	namespaceMapReconciler := &mapsctrl.NamespaceMapReconciler{
		Client: mgr.GetClient(),
	}

	if err = namespaceMapReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	namespaceOffloadingReconciler := &nsoffctrl.NamespaceOffloadingReconciler{
		Client:       mgr.GetClient(),
		Recorder:     mgr.GetEventRecorderFor("namespaceoffloading-controller"),
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

	shadowEpsReconciler := &shadowepsctrl.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = shadowEpsReconciler.SetupWithManager(ctx, mgr, *shadowEndpointSliceWorkers); err != nil {
		klog.Fatal(err)
	}

	// Start the handler to approve the virtual kubelet certificate signing requests.
	csrWatcher := csr.NewWatcher(clientset, *resyncPeriod, labels.Everything(), fields.Everything())
	csrWatcher.RegisterHandler(csr.ApproverHandler(clientset, "LiqoApproval", "This CSR was approved by Liqo",
		// Approve only the CSRs for a requestor living in a liqo tenant namespace (based on the prefix).
		// This is far from elegant, but the client-go utility generating the CSRs does not allow to customize the labels.
		func(csr *certificates.CertificateSigningRequest) bool {
			return strings.HasPrefix(csr.Spec.Username, fmt.Sprintf("system:serviceaccount:%v-", tenantnamespace.NamePrefix))
		}))
	csrWatcher.Start(ctx)

	if err = mgr.Add(offerUpdater); err != nil {
		klog.Fatal(err)
	}

	if err := mgr.Add(manager.RunnableFunc(spv.CacheRefresher(*refreshInterval))); err != nil {
		klog.Errorf("Unable to set up resource validator cache refresher: %v", err)
		os.Exit(1)
	}

	if *enableStorage {
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

	if *enableNodeFailureController {
		nodeFailureReconciler := &nodefailurectrl.NodeFailureReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}
		if err = nodeFailureReconciler.SetupWithManager(ctx, mgr); err != nil {
			klog.Errorf("Unable to start the nodeFailureReconciler", err)
			os.Exit(1)
		}
	}

	klog.Info("DEVELOPMENT VERSION")
	klog.Info("starting manager as controller manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

// Copyright 2019-2021 The Liqo Authors
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
	"flag"
	"os"
	"sync"
	"time"

	capsulev1beta1 "github.com/clastix/capsule/api/v1beta1"
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
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	foreignclusteroperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/foreign-cluster-operator"
	namectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespace-controller"
	mapsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespaceMap-controller"
	nsoffctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespaceOffloading-controller"
	offloadingctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloadingStatus-controller"
	resourceRequestOperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller"
	resourceoffercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/resourceoffer-controller"
	searchdomainoperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/search-domain-operator"
	virtualNodectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/virtualNode-controller"
	"github.com/liqotech/liqo/pkg/mapperUtils"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	errorsmanagement "github.com/liqotech/liqo/pkg/utils/errorsManagement"
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
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = sharingv1alpha1.AddToScheme(scheme)
	_ = netv1alpha1.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = offloadingv1alpha1.AddToScheme(scheme)
	_ = virtualkubeletv1alpha1.AddToScheme(scheme)

	_ = capsulev1beta1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var clusterLabels argsutils.StringMap
	var kubeletExtraAnnotations, kubeletExtraLabels argsutils.StringMap
	var kubeletExtraArgs argsutils.StringList
	var nodeExtraAnnotations, nodeExtraLabels argsutils.StringMap

	metricsAddr := flag.String("metrics-address", ":8080", "The address the metric endpoint binds to")
	probeAddr := flag.String("health-probe-address", ":8081", "The address the health probe endpoint binds to")

	// Global parameters
	resyncPeriod := flag.Duration("resync-period", 10*time.Hour, "The resync period for the informers")
	clusterID := flag.String("cluster-id", "", "The cluster ID identifying the current cluster")
	liqoNamespace := flag.String("liqo-namespace", defaultNamespace,
		"Name of the namespace where the liqo components are running")

	// Discovery parameters
	clusterName := flag.String(consts.ClusterNameParameter, "", "A mnemonic name associated with the current cluster")
	authServiceAddressOverride := flag.String(consts.AuthServiceAddressOverrideParameter, "",
		"The address the authentication service is reachable from foreign clusters (automatically retrieved if not set")
	authServicePortOverride := flag.String(consts.AuthServicePortOverrideParameter, "",
		"The port the authentication service is reachable from foreign clusters (automatically retrieved if not set")
	autoJoin := flag.Bool("auto-join-discovered-clusters", true, "Whether to automatically peer with discovered clusters")
	ownerReferencesPermissionEnforcement := flag.Bool("owner-references-permission-enforcement", false,
		"Enable support for the OwnerReferencesPermissionEnforcement admission controller "+
			"https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#ownerreferencespermissionenforcement")

	// Resource sharing parameters
	flag.Var(&clusterLabels, "cluster-labels",
		"The set of labels which characterizes the local cluster when exposed remotely as a virtual node")
	resourceSharingPercentage := argsutils.Percentage{Val: 50}
	flag.Var(&resourceSharingPercentage, "resource-sharing-percentage",
		"The amount (in percentage) of cluster resources possibly shared with foreign clusters")
	enableIncomingPeering := flag.Bool("enable-incoming-peering", true,
		"Enable remote clusters to establish an incoming peering with the local cluster (can be overwritten on a per foreign cluster basis)")
	offerDisableAutoAccept := flag.Bool("offer-disable-auto-accept", false, "Disable the automatic acceptance of resource offers")
	offerUpdateThreshold := argsutils.Percentage{Val: 5}
	flag.Var(&offerUpdateThreshold, "offer-update-threshold-percentage",
		"The threshold (in percentage) of resources quantity variation which triggers a ResourceOffer update")

	// Namespace management parameters
	offloadingStatusControllerRequeueTime := flag.Duration("offloading-status-requeue-period", 10*time.Second,
		"Period after that the offloading status controller is awaken on every NamespaceOffloading to set its status")
	namespaceMapControllerRequeueTime := flag.Duration("namespace-map-requeue-period", 30*time.Second,
		"Period after that the namespace map controller is awaken on every NamespaceMap to enforce DesiredMappings")

	// Virtual-kubelet parameters
	kubeletImage := flag.String("kubelet-image", defaultVKImage, "The image of the virtual kubelet to be deployed")
	initKubeletImage := flag.String("init-kubelet-image", defaultInitVKImage,
		"The image of the virtual kubelet init container to be deployed")
	disableKubeletCertGeneration := flag.Bool("disable-kubelet-certificate-generation", false,
		"Whether to disable the virtual kubelet certificate generation by means of an init container (used for logs/exec capabilities)")
	flag.Var(&kubeletExtraAnnotations, "kubelet-extra-annotations", "Extra annotations to add to the Virtual Kubelet Deployments and Pods")
	flag.Var(&kubeletExtraLabels, "kubelet-extra-labels", "Extra labels to add to the Virtual Kubelet Deployments and Pods")
	flag.Var(&kubeletExtraArgs, "kubelet-extra-args", "Extra arguments to add to the Virtual Kubelet Deployments and Pods")
	flag.Var(&nodeExtraAnnotations, "node-extra-annotations", "Extra annotations to add to the Virtual Node")
	flag.Var(&nodeExtraLabels, "node-extra-labels", "Extra labels to add to the Virtual Node")

	errorsmanagement.InitFlags(nil)
	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	if *clusterID == "" {
		klog.Error("Cluster ID must be provided")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider:         mapperUtils.LiqoMapperProvider(scheme),
		Scheme:                 scheme,
		MetricsBindAddress:     *metricsAddr,
		HealthProbeBindAddress: *probeAddr,
		LeaderElection:         false,
		LeaderElectionID:       "66cf253f.liqo.io",
		Port:                   9443,
	})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	namespaceManager := tenantnamespace.NewTenantNamespaceManager(clientset)
	idManager := identitymanager.NewCertificateIdentityManager(clientset, *clusterID, namespaceManager)

	// populate the lists of ClusterRoles to bind in the different peering states
	permissions, err := peeringroles.GetPeeringPermission(ctx, clientset)
	if err != nil {
		klog.Fatalf("Unable to populate peering permission: %w", err)
	}

	// Setup operators

	searchDomainReconciler := &searchdomainoperator.SearchDomainReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		ResyncPeriod:   *resyncPeriod,
		LocalClusterID: *clusterID,
	}
	if err = searchDomainReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	foreignClusterReconciler := &foreignclusteroperator.ForeignClusterReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		LiqoNamespace: *liqoNamespace,

		ResyncPeriod:                         *resyncPeriod,
		ClusterID:                            *clusterID,
		ClusterName:                          *clusterName,
		AuthServiceAddressOverride:           *authServiceAddressOverride,
		AuthServicePortOverride:              *authServicePortOverride,
		AutoJoin:                             *autoJoin,
		OwnerReferencesPermissionEnforcement: *ownerReferencesPermissionEnforcement,

		NamespaceManager:  namespaceManager,
		IdentityManager:   idManager,
		PeeringPermission: *permissions,
	}
	if err = foreignClusterReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	broadcaster := &resourceRequestOperator.Broadcaster{}
	updater := &resourceRequestOperator.OfferUpdater{}
	updater.Setup(*clusterID, mgr.GetScheme(), broadcaster, mgr.GetClient(), clusterLabels.StringMap)
	if err := broadcaster.SetupBroadcaster(clientset, updater, *resyncPeriod,
		resourceSharingPercentage.Val, offerUpdateThreshold.Val); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	resourceRequestReconciler := &resourceRequestOperator.ResourceRequestReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		ClusterID:             *clusterID,
		Broadcaster:           broadcaster,
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
	}

	resourceOfferReconciler := resourceoffercontroller.NewResourceOfferController(
		mgr, *clusterID, *resyncPeriod, *liqoNamespace, virtualKubeletOpts, *offerDisableAutoAccept)
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
		LocalClusterID:        *clusterID,
		IdentityManagerClient: clientset,
		RequeueTime:           *namespaceMapControllerRequeueTime,
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
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		LocalClusterID: *clusterID,
	}

	if err = namespaceOffloadingReconciler.SetupWithManager(mgr); err != nil {
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

	var wg = &sync.WaitGroup{}
	broadcaster.StartBroadcaster(ctx, wg)

	klog.Info("starting manager as controller manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	wg.Wait()
}

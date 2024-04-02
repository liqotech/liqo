// Copyright 2019-2024 The Liqo Authors
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
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	certificates "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/cmd/liqo-controller-manager/modules"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/ipam"
	foreignclusteroperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/foreign-cluster-operator"
	mapsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespacemap-controller"
	nsoffctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespaceoffloading-controller"
	nwforge "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	nodefailurectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/nodefailure-controller"
	offloadingipmapping "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/ipmapping"
	podstatusctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/podstatus-controller"
	resourceRequestOperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller"
	resourcemonitors "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller/resource-monitors"
	resourceoffercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/resourceoffer-controller"
	shadowepsctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/shadowendpointslice-controller"
	shadowpodctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/shadowpod-controller"
	liqostorageprovisioner "github.com/liqotech/liqo/pkg/liqo-controller-manager/storageprovisioner"
	virtualnodectrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/virtualnode-controller"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/firewallconfiguration"
	fcwh "github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/foreigncluster"
	ipwh "github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/ip"
	nsoffwh "github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/namespaceoffloading"
	nwwh "github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/network"
	podwh "github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/pod"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/routeconfiguration"
	shadowpodswh "github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/shadowpod"
	virtualnodewh "github.com/liqotech/liqo/pkg/liqo-controller-manager/webhooks/virtualnode"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/csr"
	dynamicutils "github.com/liqotech/liqo/pkg/utils/dynamic"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
	"github.com/liqotech/liqo/pkg/utils/indexer"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	"github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = monitoringv1.AddToScheme(scheme)

	_ = sharingv1alpha1.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = offloadingv1alpha1.AddToScheme(scheme)
	_ = virtualkubeletv1alpha1.AddToScheme(scheme)
	_ = ipamv1alpha1.AddToScheme(scheme)
	_ = networkingv1alpha1.AddToScheme(scheme)
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
	var labelsNotReflected argsutils.StringList
	var annotationsNotReflected argsutils.StringList
	var ingressClasses argsutils.ClassNameList
	var loadBalancerClasses argsutils.ClassNameList
	var addVirtualNodeTolerationOnOffloadedPods bool
	var ipamClient ipam.IpamClient
	var gatewayServerResources argsutils.StringList
	var gatewayClientResources argsutils.StringList
	var apiServerAddressOverride string
	var caOverride string
	var trustedCA bool

	networkingEnabled := flag.Bool("networking-enabled", true, "Enable/disable the networking module")
	authenticationEnabled := flag.Bool("authentication-enabled", true, "Enable/disable the authentication module")

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
	podcidr := flag.String("podcidr", "", "The CIDR to use for the pod network")
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
	flag.Var(&ingressClasses, "ingress-classes", "List of ingress classes offered by the cluster. Example: \"nginx;default,traefik\"")
	flag.Var(&loadBalancerClasses, "load-balancer-classes", "List of load balancer classes offered by the cluster. Example:\"metallb;default\"")

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

	reflectorsWorkers := setReflectorsWorkers()
	reflectorsType := setReflectorsType()

	flag.Var(&labelsNotReflected, "labels-not-reflected", "List of labels (key) that must not be reflected")
	flag.Var(&annotationsNotReflected, "annotations-not-reflected", "List of annotations (key) that must not be reflected")

	// Ipam server endpoint
	ipamServer := flag.String("ipam-server", "", "The address of the IPAM server (set to empty string to disable IPAM)")

	flag.BoolVar(&addVirtualNodeTolerationOnOffloadedPods, "add-virtual-node-toleration-on-offloaded-pods", false,
		"Automatically add the virtual node toleration on offloaded pods")

	// Storage Provisioner parameters
	enableStorage := flag.Bool("enable-storage", false, "enable the liqo virtual storage class")
	virtualStorageClassName := flag.String("virtual-storage-class-name", "liqo", "Name of the virtual storage class")
	realStorageClassName := flag.String("real-storage-class-name", "", "Name of the real storage class to use for the actual volumes")
	storageNamespace := flag.String("storage-namespace", "liqo-storage", "Namespace where the liqo storage-related resources are stored")

	// Node failure controller parameter
	enableNodeFailureController := flag.Bool("enable-node-failure-controller", false, "Enable the node failure controller")

	// Networking module parameters
	flag.Var(&gatewayServerResources, "gateway-server-resources",
		"The list of resource types that implements the gateway server. They must be in the form <group>/<version>/<resource>")
	flag.Var(&gatewayClientResources, "gateway-client-resources",
		"The list of resource types that implements the gateway client. They must be in the form <group>/<version>/<resource>")
	wgGatewayServerClusterRoleName := flag.String("wg-gateway-server-cluster-role-name", "liqo-gateway",
		"The name of the cluster role used by the wireguard gateway servers")
	wgGatewayClientClusterRoleName := flag.String("wg-gateway-client-cluster-role-name", "liqo-gateway",
		"The name of the cluster role used by the wireguard gateway clients")
	fabricFullMasqueradeEnabled := flag.Bool("fabric-full-masquerade-enabled", false, "Enable the full masquerade on the fabric network")
	gatewayServiceType := flag.String("gateway-service-type", string(nwforge.DefaultGwServerServiceType), "The type of the gateway service")
	gatewayServicePort := flag.Int("gateway-service-port", nwforge.DefaultGwServerPort, "The port of the gateway service")
	gatewayMTU := flag.Int("gateway-mtu", nwforge.DefaultMTU, "The MTU of the gateway interface")
	networkWorkers := flag.Int("network-ctrl-workers", 1, "The number of workers used to reconcile Network resources.")
	ipWorkers := flag.Int("ip-ctrl-workers", 1, "The number of workers used to reconcile IP resources.")
	gwmasqbypassEnabled := flag.Bool("gateway-masquerade-bypass-enabled", false, "Enable the gateway masquerade bypass")

	// Authentication module parameters
	flag.StringVar(&apiServerAddressOverride,
		"api-server-address-override", "", "Override the API server address where the Kuberentes APIServer is exposed")
	flag.StringVar(&caOverride, "ca-override", "", "Override the CA certificate used by Kubernetes to sign certificates (base64 encoded)")
	flag.BoolVar(&trustedCA, "trusted-ca", false, "Whether the Kubernetes APIServer certificate is issue by a trusted CA")

	liqoerrors.InitFlags(nil)
	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	log.SetLogger(klog.NewKlogr())

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
		IpamEndpoint:         *ipamServer,
		MetricsAddress:       kubeletMetricsAddress,
		MetricsEnabled:       kubeletMetricsEnabled,
		ReflectorsWorkers:    reflectorsWorkers,
		ReflectorsType:       reflectorsType,
		LocalPodCIDR:         *podcidr,
	}

	clusterIdentity := clusterIdentityFlags.ReadOrDie()

	ctx := ctrl.SetupSignalHandler()

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	dynClient := dynamic.NewForConfigOrDie(config)
	factory := &dynamicutils.RunnableFactory{
		DynamicSharedInformerFactory: dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, 0, corev1.NamespaceAll, nil),
	}

	// Create the main manager.
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: *metricsAddr,
		},
		HealthProbeBindAddress:        *probeAddr,
		LeaderElection:                *leaderElection,
		LeaderElectionID:              "66cf253f.ctrlmgr.liqo.io",
		LeaderElectionNamespace:       *liqoNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port: int(*webhookPort),
			},
		},
	})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	if err = mgr.Add(factory); err != nil {
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

	// Register the webhooks.
	mgr.GetWebhookServer().Register("/validate/foreign-cluster", fcwh.NewValidator())
	mgr.GetWebhookServer().Register("/mutate/foreign-cluster", fcwh.NewMutator())
	mgr.GetWebhookServer().Register("/validate/shadowpods", &webhook.Admission{Handler: spv})
	mgr.GetWebhookServer().Register("/validate/namespace-offloading", nsoffwh.New())
	mgr.GetWebhookServer().Register("/mutate/pod", podwh.New(mgr.GetClient(), addVirtualNodeTolerationOnOffloadedPods))
	mgr.GetWebhookServer().Register("/mutate/virtualnodes", virtualnodewh.New(mgr.GetClient(), &clusterIdentity, virtualKubeletOpts))
	mgr.GetWebhookServer().Register("/validate/networks", nwwh.NewValidator())
	mgr.GetWebhookServer().Register("/validate/ips", ipwh.NewValidator())
	mgr.GetWebhookServer().Register("/validate/firewallconfigurations", firewallconfiguration.NewValidator(mgr.GetClient()))
	mgr.GetWebhookServer().Register("/mutate/firewallconfigurations", firewallconfiguration.NewMutator())
	mgr.GetWebhookServer().Register("/validate/routeconfigurations", routeconfiguration.NewValidator(mgr.GetClient()))

	if err := indexer.IndexField(ctx, mgr, &corev1.Pod{}, indexer.FieldNodeNameFromPod, indexer.ExtractNodeName); err != nil {
		klog.Errorf("Unable to setup the indexer for the Pod nodeName field: %v", err)
		os.Exit(1)
	}

	clientset := kubernetes.NewForConfigOrDie(config)

	// Create an uncached client. Use mgr.GetClient() to get the cached client used in controllers.
	uncachedClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		klog.Errorf("unable to create the client: %s", err)
		os.Exit(1)
	}

	namespaceManager := tenantnamespace.NewCachedManager(ctx, clientset)
	idManager := identitymanager.NewCertificateIdentityManager(clientset, clusterIdentity, namespaceManager)

	// TODO: check if is running on EKS and start the IAM identity provider
	idProvider := identitymanager.NewCertificateIdentityProvider(ctx, clientset, clusterIdentity, namespaceManager)

	// populate the lists of ClusterRoles to bind in the different peering states
	permissions, err := peeringroles.GetPeeringPermission(ctx, clientset)
	if err != nil {
		klog.Fatalf("Unable to populate peering permission: %v", err)
	}

	// Configure the transports used for the interaction with the remote authentication service.
	// Using the same transport allows to reuse the underlying TCP/TLS connections when contacting the same destinations,
	// and reduce the overall handshake overhead, especially with high-latency links.
	secureTransport := &http.Transport{IdleConnTimeout: 1 * time.Minute}
	insecureTransport := &http.Transport{IdleConnTimeout: 1 * time.Minute, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}

	// Setup operators

	// authentication module
	if *authenticationEnabled {
		opts := &modules.AuthOption{
			IdentityProvider:         idProvider,
			NamespaceManager:         namespaceManager,
			LiqoNamespace:            *liqoNamespace,
			APIServerAddressOverride: apiServerAddressOverride,
			CAOverrideB64:            caOverride,
			TrustedCA:                trustedCA,
		}

		if err := modules.SetupAuthenticationModule(ctx, mgr, uncachedClient, opts); err != nil {
			klog.Fatalf("Unable to setup the authentication module: %v", err)
		}
	}

	foreignClusterReconciler := &foreignclusteroperator.ForeignClusterReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		LiqoNamespace: *liqoNamespace,

		ResyncPeriod:      *resyncPeriod,
		HomeCluster:       clusterIdentity,
		AutoJoin:          *autoJoin,
		NetworkingEnabled: *networkingEnabled,

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
		clusterLabels.StringMap, monitor, uint(offerUpdateThreshold.Val), *realStorageClassName, *enableStorage,
		ingressClasses, loadBalancerClasses)
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

	resourceOfferReconciler := resourceoffercontroller.NewResourceOfferController(
		mgr, idManager, *resyncPeriod, *offerDisableAutoAccept,
		labelsNotReflected.StringList, annotationsNotReflected.StringList)
	if err = resourceOfferReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	virtualNodeReconciler, err := virtualnodectrl.NewVirtualNodeReconciler(
		ctx,
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("virtualnode-controller"),
		&clusterIdentity,
		virtualKubeletOpts,
	)
	if err != nil {
		klog.Fatal(err)
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

	podStatusReconciler := &podstatusctrl.PodStatusReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err = podStatusReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the podstatus reconciler: %v", err)
		os.Exit(1)
	}

	if *enableNodeFailureController {
		nodeFailureReconciler := &nodefailurectrl.NodeFailureReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}
		if err = nodeFailureReconciler.SetupWithManager(mgr); err != nil {
			klog.Errorf("Unable to start the nodeFailureReconciler: %v", err)
			os.Exit(1)
		}
	}

	// Connect to the IPAM server if specified.
	if *ipamServer != "" {
		klog.Infof("connecting to the IPAM server %q", *ipamServer)
		dialctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		connection, err := grpc.DialContext(dialctx, *ipamServer,
			grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		cancel()
		if err != nil {
			klog.Errorf("failed to establish a connection to the IPAM %q", *ipamServer)
			os.Exit(1)
		}
		ipamClient = ipam.NewIpamClient(connection)
	}

	if *networkingEnabled {
		if err := modules.SetupNetworkingModule(ctx, mgr, &modules.NetworkingOption{
			DynClient:  dynClient,
			Factory:    factory,
			KubeClient: clientset,

			LiqoNamespace:        *liqoNamespace,
			LocalClusterIdentity: &clusterIdentity,
			IpamClient:           ipamClient,

			GatewayServerResources:         gatewayServerResources.StringList,
			GatewayClientResources:         gatewayClientResources.StringList,
			WgGatewayServerClusterRoleName: *wgGatewayServerClusterRoleName,
			WgGatewayClientClusterRoleName: *wgGatewayClientClusterRoleName,
			GatewayServiceType:             corev1.ServiceType(*gatewayServiceType),
			GatewayServicePort:             int32(*gatewayServicePort),
			GatewayMTU:                     *gatewayMTU,
			NetworkWorkers:                 *networkWorkers,
			IPWorkers:                      *ipWorkers,
			FabricFullMasquerade:           *fabricFullMasqueradeEnabled,
			GwmasqbypassEnabled:            *gwmasqbypassEnabled,
		}); err != nil {
			klog.Fatalf("Unable to setup the networking module: %v", err)
		}
	}

	offloadedPodReconciler := offloadingipmapping.NewOffloadedPodReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("offloadedpod-controller"),
	)
	if err := offloadedPodReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the offloadedPod reconciler: %v", err)
		os.Exit(1)
	}

	configurationReconciler := offloadingipmapping.NewConfigurationReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("configuration-controller"),
	)
	if err := configurationReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the configuration reconciler: %v", err)
		os.Exit(1)
	}

	klog.Info("starting manager as controller manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

// setReflectorsWorkers sets the flags for the number of workers used by the reflectors.
func setReflectorsWorkers() map[string]*uint {
	reflectorsWorkers := make(map[string]*uint, len(generic.Reflectors))
	for i := range generic.Reflectors {
		resource := &generic.Reflectors[i]
		stringFlag := fmt.Sprintf("%s-reflection-workers", *resource)
		defaultValue := root.DefaultReflectorsWorkers[*resource]
		usage := fmt.Sprintf("The number of workers used for the %s reflector", *resource)
		reflectorsWorkers[string(*resource)] = flag.Uint(stringFlag, defaultValue, usage)
	}
	return reflectorsWorkers
}

// setReflectorsType sets the flags for the type of reflection used by the reflectors.
func setReflectorsType() map[string]*string {
	reflectorsType := make(map[string]*string, len(generic.ReflectorsCustomizableType))
	for i := range generic.ReflectorsCustomizableType {
		resource := &generic.ReflectorsCustomizableType[i]
		stringFlag := fmt.Sprintf("%s-reflection-type", *resource)
		defaultValue := string(root.DefaultReflectorsTypes[*resource])
		usage := fmt.Sprintf("The type of reflection used for the %s reflector", *resource)
		reflectorsType[string(*resource)] = flag.String(stringFlag, defaultValue, usage)
	}
	return reflectorsType
}

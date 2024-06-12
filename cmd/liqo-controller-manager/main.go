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
	"flag"
	"os"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/cmd/liqo-controller-manager/modules"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/ipam"
	remoteresourceslicecontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/remoteresourceslice-controller"
	virtualnodecreatorcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/virtualnodecreator-controller"
	foreignclustercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/foreigncluster-controller"
	offloadingipmapping "github.com/liqotech/liqo/pkg/liqo-controller-manager/offloading/ipmapping"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayserver"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	dynamicutils "github.com/liqotech/liqo/pkg/utils/dynamic"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
	"github.com/liqotech/liqo/pkg/utils/indexer"
	ipamips "github.com/liqotech/liqo/pkg/utils/ipam/ips"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = monitoringv1.AddToScheme(scheme)

	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = offloadingv1alpha1.AddToScheme(scheme)
	_ = virtualkubeletv1alpha1.AddToScheme(scheme)
	_ = ipamv1alpha1.AddToScheme(scheme)
	_ = networkingv1alpha1.AddToScheme(scheme)
	_ = authv1alpha1.AddToScheme(scheme)
}

func main() {
	var clusterLabels argsutils.StringMap
	var ingressClasses argsutils.ClassNameList
	var loadBalancerClasses argsutils.ClassNameList
	var defaultNodeResources argsutils.ResourceMap
	var gatewayServerResources argsutils.StringList
	var gatewayClientResources argsutils.StringList
	var apiServerAddressOverride string
	var caOverride string
	var trustedCA bool
	var awsConfig identitymanager.LocalAwsConfig

	// Cluster-wide modules enable/disable flags.
	networkingEnabled := flag.Bool("networking-enabled", true, "Enable/disable the networking module")
	authenticationEnabled := flag.Bool("authentication-enabled", true, "Enable/disable the authentication module")
	offloadingEnabled := flag.Bool("offloading-enabled", true, "Enable/disable the offloading module")

	// Manager flags
	webhookPort := flag.Uint("webhook-port", 9443, "The port the webhook server binds to")
	metricsAddr := flag.String("metrics-address", ":8080", "The address the metric endpoint binds to")
	probeAddr := flag.String("health-probe-address", ":8081", "The address the health probe endpoint binds to")
	leaderElection := flag.Bool("enable-leader-election", false, "Enable leader election for controller manager")

	// Global parameters
	resyncPeriod := flag.Duration("resync-period", 10*time.Hour, "The resync period for the informers")
	clusterIDFlags := argsutils.NewClusterIDFlags(true, nil)
	liqoNamespace := flag.String("liqo-namespace", consts.DefaultLiqoNamespace,
		"Name of the namespace where the liqo components are running")
	foreignClusterWorkers := flag.Int("foreign-cluster-workers", 1, "The number of workers used to reconcile ForeignCluster resources.")
	foreignClusterPingInterval := flag.Duration("foreign-cluster-ping-interval", 15*time.Second,
		"The frequency of the ForeignCluster API server readiness check. Set 0 to disable the check")
	foreignClusterPingTimeout := flag.Duration("foreign-cluster-ping-timeout", 5*time.Second,
		"The timeout of the ForeignCluster API server readiness check")

	// NETWORKING MODULE
	ipamServer := flag.String("ipam-server", "", "The address of the IPAM server (set to empty string to disable IPAM)")
	flag.Var(&gatewayServerResources, "gateway-server-resources",
		"The list of resource types that implements the gateway server. They must be in the form <group>/<version>/<resource>")
	flag.Var(&gatewayClientResources, "gateway-client-resources",
		"The list of resource types that implements the gateway client. They must be in the form <group>/<version>/<resource>")
	wgGatewayServerClusterRoleName := flag.String("wg-gateway-server-cluster-role-name", "liqo-gateway",
		"The name of the cluster role used by the wireguard gateway servers")
	wgGatewayClientClusterRoleName := flag.String("wg-gateway-client-cluster-role-name", "liqo-gateway",
		"The name of the cluster role used by the wireguard gateway clients")
	fabricFullMasqueradeEnabled := flag.Bool("fabric-full-masquerade-enabled", false, "Enable the full masquerade on the fabric network")
	gatewayServiceType := flag.String("gateway-service-type", string(gatewayserver.DefaultServiceType), "The type of the gateway service")
	gatewayServicePort := flag.Int("gateway-service-port", gatewayserver.DefaultPort, "The port of the gateway service")
	gatewayMTU := flag.Int("gateway-mtu", gatewayserver.DefaultMTU, "The MTU of the gateway interface")
	gatewayProxy := flag.Bool("gateway-proxy", gatewayserver.DefaultProxy, "Enable the proxy on the gateway")
	networkWorkers := flag.Int("network-ctrl-workers", 1, "The number of workers used to reconcile Network resources.")
	ipWorkers := flag.Int("ip-ctrl-workers", 1, "The number of workers used to reconcile IP resources.")

	// AUTHENTICATION MODULE
	flag.StringVar(&apiServerAddressOverride,
		"api-server-address-override", "", "Override the API server address where the Kuberentes APIServer is exposed")
	flag.StringVar(&caOverride, "ca-override", "", "Override the CA certificate used by Kubernetes to sign certificates (base64 encoded)")
	flag.BoolVar(&trustedCA, "trusted-ca", false, "Whether the Kubernetes APIServer certificate is issue by a trusted CA")
	// AWS configurations
	flag.StringVar(&awsConfig.AwsAccessKeyID, "aws-access-key-id", "", "AWS IAM AccessKeyID for the Liqo User")
	flag.StringVar(&awsConfig.AwsSecretAccessKey, "aws-secret-access-key", "", "AWS IAM SecretAccessKey for the Liqo User")
	flag.StringVar(&awsConfig.AwsRegion, "aws-region", "", "AWS region where the local cluster is running")
	flag.StringVar(&awsConfig.AwsClusterName, "aws-cluster-name", "", "Name of the local EKS cluster")
	// Resource sharing parameters
	flag.Var(&clusterLabels, consts.ClusterLabelsParameter,
		"The set of labels which characterizes the local cluster when exposed remotely as a virtual node")
	flag.Var(&ingressClasses, "ingress-classes", "List of ingress classes offered by the cluster. Example: \"nginx;default,traefik\"")
	flag.Var(&loadBalancerClasses, "load-balancer-classes", "List of load balancer classes offered by the cluster. Example:\"metallb;default\"")
	flag.Var(&defaultNodeResources, "default-node-resources", "Default resources assigned to the Virtual Node Pod")

	// OFFLOADING MODULE
	// Storage Provisioner parameters
	enableStorage := flag.Bool("enable-storage", false, "enable the liqo virtual storage class")
	virtualStorageClassName := flag.String("virtual-storage-class-name", "liqo", "Name of the virtual storage class")
	realStorageClassName := flag.String("real-storage-class-name", "", "Name of the real storage class to use for the actual volumes")
	storageNamespace := flag.String("storage-namespace", "liqo-storage", "Namespace where the liqo storage-related resources are stored")
	// Service continuity
	enableNodeFailureController := flag.Bool("enable-node-failure-controller", false, "Enable the node failure controller")
	// Controllers workers
	shadowPodWorkers := flag.Int("shadow-pod-ctrl-workers", 10, "The number of workers used to reconcile ShadowPod resources.")
	shadowEndpointSliceWorkers := flag.Int("shadow-endpointslice-ctrl-workers", 10,
		"The number of workers used to reconcile ShadowEndpointSlice resources.")

	liqoerrors.InitFlags(nil)
	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	log.SetLogger(klog.NewKlogr())

	clusterID := clusterIDFlags.ReadOrDie()

	ctx := ctrl.SetupSignalHandler()

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	// Configure clients:

	// clientset
	clientset := kubernetes.NewForConfigOrDie(config)

	// uncached client. Note: Use mgr.GetClient() to get the cached client used in controllers.
	uncachedClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		klog.Errorf("unable to create the client: %s", err)
		os.Exit(1)
	}

	// dynamic client
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

	if err := indexer.IndexField(ctx, mgr, &corev1.Pod{}, indexer.FieldNodeNameFromPod, indexer.ExtractNodeName); err != nil {
		klog.Errorf("Unable to setup the indexer for the Pod nodeName field: %v", err)
		os.Exit(1)
	}

	namespaceManager := tenantnamespace.NewCachedManager(ctx, clientset)

	// Setup operators for each module:

	// NETWORKING MODULE
	if *networkingEnabled {
		// Connect to the IPAM server if specified.
		var ipamClient ipam.IpamClient
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

		if err := modules.SetupNetworkingModule(ctx, mgr, &modules.NetworkingOption{
			DynClient:  dynClient,
			Factory:    factory,
			KubeClient: clientset,

			LiqoNamespace:  *liqoNamespace,
			LocalClusterID: clusterID,
			IpamClient:     ipamClient,

			GatewayServerResources:         gatewayServerResources.StringList,
			GatewayClientResources:         gatewayClientResources.StringList,
			WgGatewayServerClusterRoleName: *wgGatewayServerClusterRoleName,
			WgGatewayClientClusterRoleName: *wgGatewayClientClusterRoleName,
			GatewayServiceType:             corev1.ServiceType(*gatewayServiceType),
			GatewayServicePort:             int32(*gatewayServicePort),
			GatewayMTU:                     *gatewayMTU,
			GatewayProxy:                   *gatewayProxy,
			NetworkWorkers:                 *networkWorkers,
			IPWorkers:                      *ipWorkers,
			FabricFullMasquerade:           *fabricFullMasqueradeEnabled,
		}); err != nil {
			klog.Fatalf("Unable to setup the networking module: %v", err)
		}
	}

	// AUTHENTICATION MODULE
	if *authenticationEnabled {
		var idProvider identitymanager.IdentityProvider
		if awsConfig.IsEmpty() {
			idProvider = identitymanager.NewCertificateIdentityProvider(ctx,
				mgr.GetClient(), clientset, config, clusterID, namespaceManager)
		} else {
			idProvider = identitymanager.NewIAMIdentityProvider(ctx,
				mgr.GetClient(), clientset, clusterID, &awsConfig, namespaceManager)
		}
		opts := &modules.AuthOption{
			IdentityProvider:         idProvider,
			NamespaceManager:         namespaceManager,
			LocalClusterID:           clusterID,
			LiqoNamespace:            *liqoNamespace,
			APIServerAddressOverride: apiServerAddressOverride,
			CAOverrideB64:            caOverride,
			TrustedCA:                trustedCA,
			SliceStatusOptions: &remoteresourceslicecontroller.SliceStatusOptions{
				EnableStorage:             *enableStorage,
				LocalRealStorageClassName: *realStorageClassName,
				IngressClasses:            ingressClasses,
				LoadBalancerClasses:       loadBalancerClasses,
				ClusterLabels:             clusterLabels.StringMap,
				DefaultResourceQuantity:   defaultNodeResources.ToResourceList(),
			},
		}

		if err := modules.SetupAuthenticationModule(ctx, mgr, uncachedClient, opts); err != nil {
			klog.Errorf("Unable to setup the authentication module: %v", err)
			os.Exit(1)
		}
	}

	// OFFLOADING MODULE
	if *offloadingEnabled {
		opts := &modules.OffloadingOption{
			Clientset:                   clientset,
			LocalClusterID:              clusterID,
			NamespaceManager:            namespaceManager,
			EnableStorage:               *enableStorage,
			VirtualStorageClassName:     *virtualStorageClassName,
			RealStorageClassName:        *realStorageClassName,
			StorageNamespace:            *storageNamespace,
			EnableNodeFailureController: *enableNodeFailureController,
			ShadowPodWorkers:            *shadowPodWorkers,
			ShadowEndpointSliceWorkers:  *shadowEndpointSliceWorkers,
			ResyncPeriod:                *resyncPeriod,
		}

		if err := modules.SetupOffloadingModule(ctx, mgr, opts); err != nil {
			klog.Errorf("Unable to setup the offloading module: %v", err)
			os.Exit(1)
		}
	}

	// CROSS MODULE OPERATORS

	// AUTHENTICATION MODULE & OFFLOADING MODULE
	if *authenticationEnabled && *offloadingEnabled {
		// Configure controller that create virtualnodes from resourceslices.
		vnCreatorReconciler := virtualnodecreatorcontroller.NewVirtualNodeCreatorReconciler(
			mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor("virtualnodecreator-controller"))
		if err := vnCreatorReconciler.SetupWithManager(mgr); err != nil {
			klog.Errorf("Unable to setup the virtualnodecreator reconciler: %v", err)
			os.Exit(1)
		}
	}

	// OFFLOADING MODULE & NETWORKING MODULE
	if *offloadingEnabled && *networkingEnabled {
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

		go func() {
			maxRetries := 5
			for {
				// enforce IP remapping for the API Server
				err := ipamips.EnforceAPIServerIPRemapping(ctx, uncachedClient, *liqoNamespace)
				if err == nil {
					break
				}

				klog.Errorf("Unable to enforce the API Server IP remapping: %v, retrying...", err)

				time.Sleep(10 * time.Second)

				maxRetries--
				if maxRetries == 0 {
					os.Exit(1)
				}
			}
		}()
	}

	// Configure the foreigncluster controller.
	idManager := identitymanager.NewCertificateIdentityManager(ctx, mgr.GetClient(), clientset, mgr.GetConfig(), clusterID, namespaceManager)
	foreignClusterReconciler := &foreignclustercontroller.ForeignClusterReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		ResyncPeriod: *resyncPeriod,

		NetworkingEnabled:     *networkingEnabled,
		AuthenticationEnabled: *authenticationEnabled,
		OffloadingEnabled:     *offloadingEnabled,

		APIServerCheckers: foreignclustercontroller.NewAPIServerCheckers(idManager, *foreignClusterPingInterval, *foreignClusterPingTimeout),
	}
	if err = foreignClusterReconciler.SetupWithManager(mgr, *foreignClusterWorkers); err != nil {
		klog.Errorf("Unable to setup the foreigncluster reconciler: %v", err)
		os.Exit(1)
	}

	// Start the manager.
	klog.Info("starting manager as controller manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

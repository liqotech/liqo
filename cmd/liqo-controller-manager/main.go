// Copyright 2019-2025 The Liqo Authors
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
	"fmt"
	"os"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/cobra"
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

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/cmd/liqo-controller-manager/modules"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/ipam"
	liqocontrollermanager "github.com/liqotech/liqo/pkg/liqo-controller-manager"
	foreignclustercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/core/foreigncluster-controller"
	ipmapping "github.com/liqotech/liqo/pkg/liqo-controller-manager/ipmapping"
	quotacreatorcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/quotacreator-controller"
	virtualnodecreatorcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/virtualnodecreator-controller"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	dynamicutils "github.com/liqotech/liqo/pkg/utils/dynamic"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	grpcutils "github.com/liqotech/liqo/pkg/utils/grpc"
	"github.com/liqotech/liqo/pkg/utils/indexer"
	ipamips "github.com/liqotech/liqo/pkg/utils/ipam/mapping"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/resource"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	scheme = runtime.NewScheme()
	opts   = liqocontrollermanager.NewOptions()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = monitoringv1.AddToScheme(scheme)

	_ = liqov1beta1.AddToScheme(scheme)
	_ = offloadingv1beta1.AddToScheme(scheme)
	_ = ipamv1alpha1.AddToScheme(scheme)
	_ = networkingv1beta1.AddToScheme(scheme)
	_ = authv1beta1.AddToScheme(scheme)
}

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-controller-manager",
		RunE: run,
	}

	liqoerrors.InitFlags(cmd.Flags())
	flagsutils.InitKlogFlags(cmd.Flags())
	restcfg.InitFlags(cmd.Flags())

	liqocontrollermanager.InitFlags(cmd.Flags(), opts)

	if err := cmd.Execute(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	cmd.SetContext(ctrl.SetupSignalHandler())

	log.SetLogger(klog.NewKlogr())

	clusterID := opts.ClusterIDFlags.ReadOrDie()

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	// Configure clients:
	clientset := kubernetes.NewForConfigOrDie(config)

	uncachedClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("unable to create uncached client: %w", err)
	}

	dynClient := dynamic.NewForConfigOrDie(config)
	factory := &dynamicutils.RunnableFactory{
		DynamicSharedInformerFactory: dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, 0, corev1.NamespaceAll, nil),
	}

	resource.SetGlobalLabels(opts.GlobalLabels.StringMap)
	resource.SetGlobalAnnotations(opts.GlobalAnnotations.StringMap)

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: opts.MetricsAddr,
		},
		HealthProbeBindAddress:        opts.ProbeAddr,
		LeaderElection:                opts.LeaderElection,
		LeaderElectionID:              "66cf253f.ctrlmgr.liqo.io",
		LeaderElectionNamespace:       opts.LiqoNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port: opts.WebhookPort,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("manager creation failed: %w", err)
	}

	if err = mgr.Add(factory); err != nil {
		return fmt.Errorf("unable to add factory to manager: %w", err)
	}

	// Register the healthiness probes.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up healthz probe: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up readyz probe: %w", err)
	}

	if err := indexer.IndexField(cmd.Context(), mgr, &corev1.Pod{}, indexer.FieldNodeNameFromPod, indexer.ExtractNodeName); err != nil {
		return fmt.Errorf("unable to setup the indexer for the Pod nodeName field: %w", err)
	}

	namespaceManager := tenantnamespace.NewCachedManager(cmd.Context(), clientset, scheme)

	// Setup operators for each module:

	// NETWORKING MODULE
	if opts.NetworkingEnabled {
		// Connect to the IPAM server if specified.
		var ipamClient ipam.IPAMClient
		if opts.IPAMServer != "" {
			klog.Infof("connecting to the IPAM server %q", opts.IPAMServer)
			conn, err := grpc.NewClient(opts.IPAMServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return fmt.Errorf("failed to establish a connection to the IPAM %q: %w", opts.IPAMServer, err)
			}

			if err := grpcutils.WaitForConnectionReady(cmd.Context(), conn, 10*time.Second); err != nil {
				return fmt.Errorf("failed to establish a connection to the IPAM server %q: %w", opts.IPAMServer, err)
			}
			klog.Infof("connected to the IPAM server (status: %s)", conn.GetState())

			defer conn.Close()

			ipamClient = ipam.NewIPAMClient(conn)
		}

		netOpts := modules.NewNetworkingOption(factory, dynClient, ipamClient, opts)

		if err := modules.SetupNetworkingModule(cmd.Context(), mgr, uncachedClient, netOpts); err != nil {
			return fmt.Errorf("unable to setup the networking module: %w", err)
		}
	}

	// AUTHENTICATION MODULE
	if opts.AuthenticationEnabled {
		var idProvider identitymanager.IdentityProvider
		if opts.AWSConfig.IsEmpty() {
			idProvider = identitymanager.NewCertificateIdentityProvider(cmd.Context(),
				mgr.GetClient(), clientset, config, clusterID, namespaceManager)
		} else {
			idProvider = identitymanager.NewIAMIdentityProvider(cmd.Context(),
				mgr.GetClient(), clientset, clusterID, opts.AWSConfig, namespaceManager)
		}

		authOpts := modules.NewAuthOption(idProvider, namespaceManager, clusterID, opts)

		if err := modules.SetupAuthenticationModule(cmd.Context(), mgr, uncachedClient, authOpts); err != nil {
			return fmt.Errorf("unable to setup the authentication module: %w", err)
		}
	}

	// OFFLOADING MODULE
	if opts.OffloadingEnabled {
		offOpts := modules.NewOffloadingOption(clientset, clusterID, namespaceManager, opts)

		if err := modules.SetupOffloadingModule(cmd.Context(), mgr, offOpts); err != nil {
			return fmt.Errorf("unable to setup the offloading module: %w", err)
		}
	}

	// CROSS MODULE OPERATORS

	// AUTHENTICATION MODULE & OFFLOADING MODULE
	if opts.AuthenticationEnabled && opts.OffloadingEnabled {
		// Configure controller that create virtualnodes from resourceslices.
		vnCreatorReconciler := virtualnodecreatorcontroller.NewVirtualNodeCreatorReconciler(
			mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor("virtualnodecreator-controller"))
		if err := vnCreatorReconciler.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("unable to setup the virtualnodecreator reconciler: %w", err)
		}

		// Configure controller that create quotas from resourceslices.
		quotaCreatorReconciler := quotacreatorcontroller.NewQuotaCreatorReconciler(
			mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor("quotacreator-controller"),
			offloadingv1beta1.LimitsEnforcement(opts.DefaultLimitsEnforcement))
		if err := quotaCreatorReconciler.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("unable to setup the quotacreator reconciler: %w", err)
		}
	}

	// OFFLOADING MODULE & NETWORKING MODULE
	if opts.OffloadingEnabled && opts.NetworkingEnabled {
		offloadedPodReconciler := ipmapping.NewOffloadedPodReconciler(
			mgr.GetClient(),
			mgr.GetScheme(),
			mgr.GetEventRecorderFor("offloadedpod-controller"),
		)
		if err := offloadedPodReconciler.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("unable to start the offloadedPod reconciler: %w", err)
		}

		configurationReconciler := ipmapping.NewConfigurationReconciler(
			mgr.GetClient(),
			mgr.GetScheme(),
			mgr.GetEventRecorderFor("configuration-controller"),
		)
		if err := configurationReconciler.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("unable to start the configuration reconciler: %w", err)
		}

		if opts.EnableAPIServerIPRemapping {
			if err := ipamips.EnforceAPIServerIPRemapping(cmd.Context(), uncachedClient, opts.LiqoNamespace); err != nil {
				return fmt.Errorf("unable to enforce the API server IP remapping: %w", err)
			}
		}

		if err := ipamips.EnforceAPIServerProxyIPRemapping(cmd.Context(), uncachedClient, opts.LiqoNamespace); err != nil {
			return fmt.Errorf("unable to enforce the API server proxy IP remapping: %w", err)
		}
	}

	// CORE OPERATORS
	// Configure the foreigncluster controller.
	idManager := identitymanager.NewCertificateIdentityManager(cmd.Context(), mgr.GetClient(), clientset, mgr.GetConfig(), clusterID, namespaceManager)
	foreignClusterReconciler := &foreignclustercontroller.ForeignClusterReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		ResyncPeriod: opts.ResyncPeriod,

		NetworkingEnabled:     opts.NetworkingEnabled,
		AuthenticationEnabled: opts.AuthenticationEnabled,
		OffloadingEnabled:     opts.OffloadingEnabled,

		APIServerCheckers: foreignclustercontroller.NewAPIServerCheckers(idManager, opts.ForeignClusterPingInterval, opts.ForeignClusterPingTimeout),
	}
	if err = foreignClusterReconciler.SetupWithManager(mgr, opts.ForeignClusterWorkers); err != nil {
		return fmt.Errorf("unable to setup the foreigncluster reconciler: %w", err)
	}

	// Start the manager.
	klog.Info("starting manager as controller manager")
	if err := mgr.Start(cmd.Context()); err != nil {
		return fmt.Errorf("unable to start the manager: %w", err)
	}

	return nil
}

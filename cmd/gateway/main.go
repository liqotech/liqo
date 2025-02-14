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

// Package wireguard contains the logic to configure the Wireguard interface.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/concurrent"
	"github.com/liqotech/liqo/pkg/gateway/connection"
	"github.com/liqotech/liqo/pkg/gateway/connection/conncheck"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/remapping"
	"github.com/liqotech/liqo/pkg/route"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/kernel"
	kernelversion "github.com/liqotech/liqo/pkg/utils/kernel/version"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/resource"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	connoptions       *connection.Options
	scheme            = runtime.NewScheme()
	globalLabels      argsutils.StringMap
	globalAnnotations argsutils.StringMap
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(networkingv1beta1.AddToScheme(scheme))
}

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-gateway",
		RunE: run,
	}

	flagsutils.InitKlogFlags(cmd.Flags())
	restcfg.InitFlags(cmd.Flags())

	gwoptions := gateway.NewOptions()
	connoptions = connection.NewOptions(
		gwoptions,
		conncheck.NewOptions(),
	)

	gateway.InitFlags(cmd.Flags(), connoptions.GwOptions)
	if err := gateway.MarkFlagsRequired(&cmd); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	connection.InitFlags(cmd.Flags(), connoptions)

	// Register the flags for setting global labels and annotations
	cmd.Flags().Var(&globalLabels, "global-labels", "Global labels to be added to all created resources (key=value)")
	cmd.Flags().Var(&globalAnnotations, "global-annotations", "Global annotations to be added to all created resources (key=value)")

	if err := cmd.Execute(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	var err error

	// Check if the minimum kernel version is satisfied.
	if !connoptions.GwOptions.DisableKernelVersionCheck {
		if err := kernelversion.CheckKernelVersion(&connoptions.GwOptions.MinimumKernelVersion); err != nil {
			return fmt.Errorf("kernel version check failed: %w, disable this check with --%s", err, gateway.FlagNameDisableKernelVersionCheck)
		}
	}

	// Enable ip_forwarding.
	if err = kernel.EnableIPForwarding(); err != nil {
		return err
	}

	// Disable rp_filter.
	if err = kernel.DisableRtFilter(); err != nil {
		return err
	}

	// Set controller-runtime logger.
	log.SetLogger(klog.NewKlogr())

	// Initialize global labels from flag
	resource.SetGlobalLabels(globalLabels.StringMap)
	resource.SetGlobalAnnotations(globalAnnotations.StringMap)

	// Get the rest config.
	cfg := config.GetConfigOrDie()

	// Create the client. This client should be used only outside the reconciler.
	// This client does not need a cache.
	cl, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return fmt.Errorf("unable to create client: %w", err)
	}

	// Create the manager.
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: connoptions.GwOptions.MetricsAddress,
		},
		HealthProbeBindAddress: connoptions.GwOptions.ProbeAddr,
		LeaderElection:         connoptions.GwOptions.LeaderElection,
		LeaderElectionID: fmt.Sprintf(
			"%s.%s.%s.connections.liqo.io",
			connoptions.GwOptions.Name, connoptions.GwOptions.Namespace, connoptions.GwOptions.Mode,
		),
		LeaderElectionNamespace:       connoptions.GwOptions.Namespace,
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		LeaseDuration:                 &connoptions.GwOptions.LeaderElectionLeaseDuration,
		RenewDeadline:                 &connoptions.GwOptions.LeaderElectionRenewDeadline,
		RetryPeriod:                   &connoptions.GwOptions.LeaderElectionRetryPeriod,
	})
	if err != nil {
		return fmt.Errorf("unable to create manager: %w", err)
	}

	// Register the healthiness probes.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up healthz probe: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up readyz probe: %w", err)
	}

	if connoptions.EnableConnectionController {
		// Setup the connection controller.
		connr, err := connection.NewConnectionsReconciler(
			cmd.Context(),
			mgr.GetClient(),
			mgr.GetScheme(),
			mgr.GetEventRecorderFor("connection-controller"),
			connoptions,
		)
		if err != nil {
			return fmt.Errorf("unable to create connectioons reconciler: %w", err)
		}

		if err = connr.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("unable to setup connections reconciler: %w", err)
		}
	}

	rcr, err := route.NewRouteConfigurationReconcilerWithoutFinalizer(
		mgr.GetClient(),
		mgr.GetScheme(),
		connoptions.GwOptions.Name,
		mgr.GetEventRecorderFor("routeconfiguration-controller"),
		[]labels.Set{
			gateway.ForgeRouteExternalTargetLabels(connoptions.GwOptions.RemoteClusterID),
			gateway.ForgeRouteInternalTargetLabels(),
			gateway.ForgeRouteInternalTargetLabelsByNode(connoptions.GwOptions.NodeName),
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create routeconfiguration reconciler: %w", err)
	}

	if err := rcr.SetupWithManager(cmd.Context(), mgr); err != nil {
		return fmt.Errorf("unable to setup routeconfiguration reconciler: %w", err)
	}

	// Setup the firewall configuration controller.
	fwcr, err := firewall.NewFirewallConfigurationReconcilerWithoutFinalizer(
		mgr.GetClient(),
		mgr.GetScheme(),
		connoptions.GwOptions.Name,
		mgr.GetEventRecorderFor("firewall-controller"),
		[]labels.Set{
			gateway.ForgeFirewallInternalTargetLabels(),
			remapping.ForgeFirewallTargetLabels(connoptions.GwOptions.RemoteClusterID),
			remapping.ForgeFirewallTargetLabelsIPMappingGw(),
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create firewall configuration reconciler: %w", err)
	}

	if err := fwcr.SetupWithManager(cmd.Context(), mgr, true); err != nil {
		return fmt.Errorf("unable to setup firewall configuration reconciler: %w", err)
	}

	runnable, err := concurrent.NewRunnableGatewayStartup(
		cl,
		connoptions.GwOptions.PodName,
		connoptions.GwOptions.Name,
		connoptions.GwOptions.Namespace,
		connoptions.GwOptions.ConcurrentContainersNames,
	)
	if err != nil {
		return fmt.Errorf("unable to create concurrent runnable: %w", err)
	}

	if err := mgr.Add(runnable); err != nil {
		return fmt.Errorf("unable to add concurrent runnable: %w", err)
	}

	// Start the manager.
	return mgr.Start(cmd.Context())
}

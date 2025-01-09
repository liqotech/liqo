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
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/fabric"
	sourcedetector "github.com/liqotech/liqo/pkg/fabric/source-detector"
	"github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/concurrent"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/remapping"
	"github.com/liqotech/liqo/pkg/route"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	kernelversion "github.com/liqotech/liqo/pkg/utils/kernel/version"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/resource"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	options           = fabric.NewOptions()
	scheme            = runtime.NewScheme()
	globalLabels      argsutils.StringMap
	globalAnnotations argsutils.StringMap
)

func init() {
	utilruntime.Must(networkingv1beta1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
}

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-fabric",
		RunE: run,
	}

	flagsutils.InitKlogFlags(cmd.Flags())
	restcfg.InitFlags(cmd.Flags())
	fabric.InitFlags(cmd.Flags(), options)

	// Register the flags for setting global labels and annotations
	cmd.Flags().Var(&globalLabels, "global-labels", "Global labels to be added to all created resources (key=value)")
	cmd.Flags().Var(&globalAnnotations, "global-annotations", "Global annotations to be added to all created resources (key=value)")

	if err := fabric.MarkFlagsRequired(&cmd); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	var err error

	// Check if the minimum kernel version is satisfied.
	if !options.DisableKernelVersionCheck {
		if err := kernelversion.CheckKernelVersion(&options.MinimumKernelVersion); err != nil {
			return fmt.Errorf("kernel version check failed: %w, disable this check with --%s", err, fabric.FlagNameDisableKernelVersionCheck)
		}
	}

	// Set controller-runtime logger.
	log.SetLogger(klog.NewKlogr())

	// Initialize global labels from flag
	resource.SetGlobalLabels(globalLabels.StringMap)
	resource.SetGlobalAnnotations(globalAnnotations.StringMap)

	// Get the rest config.
	cfg := config.GetConfigOrDie()

	// Create a label selector to filter only the events for gateway pods
	reqGatewayPods, err := labels.NewRequirement(
		gateway.GatewayComponentKey,
		selection.Equals,
		[]string{gateway.GatewayComponentGateway},
	)
	reqActiveGatewayPods, err := labels.NewRequirement(
		concurrent.ActiveGatewayKey,
		selection.Equals,
		[]string{concurrent.ActiveGatewayValue},
	)
	utilruntime.Must(err)

	// Create the manager.
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: options.MetricsAddress,
		},
		HealthProbeBindAddress: options.ProbeAddr,
		LeaderElection:         false,
		NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			opts.ByObject = map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Label: labels.NewSelector().Add(*reqGatewayPods).Add(*reqActiveGatewayPods),
				},
			}
			return cache.New(config, opts)
		},
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

	gwr, err := sourcedetector.NewGatewayReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("gateway-controller"),
		options,
	)
	if err != nil {
		return fmt.Errorf("unable to create gateway reconciler: %w", err)
	}

	if err := gwr.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup gateway reconciler: %w", err)
	}

	// Setup the firewall configuration controller.
	fwcr, err := firewall.NewFirewallConfigurationReconcilerWithFinalizer(
		mgr.GetClient(),
		mgr.GetScheme(),
		options.PodName,
		mgr.GetEventRecorderFor("firewall-controller"),
		[]labels.Set{
			fabric.ForgeFirewallTargetLabels(),
			remapping.ForgeFirewallTargetLabelsIPMappingFabric(),
			fabric.ForgeFirewallTargetLabelsSingleNode(options.NodeName),
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create firewall configuration reconciler: %w", err)
	}

	if err := fwcr.SetupWithManager(cmd.Context(), mgr, options.EnableNftMonitor); err != nil {
		return fmt.Errorf("unable to setup firewall configuration reconciler: %w", err)
	}

	// Setup the route configuration controller.
	rcr, err := route.NewRouteConfigurationReconcilerWithFinalizer(
		mgr.GetClient(),
		mgr.GetScheme(),
		options.PodName,
		mgr.GetEventRecorderFor("route-controller"),
		[]labels.Set{fabric.ForgeRouteTargetLabels()},
	)
	if err != nil {
		return fmt.Errorf("unable to create route configuration reconciler: %w", err)
	}

	if err := rcr.SetupWithManager(cmd.Context(), mgr); err != nil {
		return fmt.Errorf("unable to setup route configuration reconciler: %w", err)
	}

	ifr, err := fabric.NewInternalFabricReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("internalfabric-controller"),
		options,
	)
	if err != nil {
		return fmt.Errorf("unable to create internal fabric reconciler: %w", err)
	}

	if err := ifr.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup internal fabric reconciler: %w", err)
	}

	// Start the manager.
	return mgr.Start(cmd.Context())
}

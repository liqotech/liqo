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

// Package wireguard contains the logic to configure the Wireguard interface.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/connection"
	"github.com/liqotech/liqo/pkg/gateway/connection/conncheck"
	"github.com/liqotech/liqo/pkg/gateway/remapping"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	addToSchemeFunctions = []func(*runtime.Scheme) error{
		networkingv1alpha1.AddToScheme,
	}
	connoptions  *connection.Options
	remapoptions *remapping.Options
)

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-gateway",
		RunE: run,
	}

	legacyflags := flag.NewFlagSet("legacy", flag.ExitOnError)
	restcfg.InitFlags(legacyflags)
	klog.InitFlags(legacyflags)
	flagsutils.FromFlagToPflag(legacyflags, cmd.Flags())

	defInfaName, err := getDefaultInterfaceName()
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	gwoptions := gateway.NewOptions()
	connoptions = connection.NewOptions(
		gwoptions,
		conncheck.NewOptions(),
	)
	remapoptions = remapping.NewOptions(
		gwoptions, defInfaName,
	)

	gateway.InitFlags(cmd.Flags(), connoptions.GwOptions)
	if err := gateway.MarkFlagsRequired(&cmd); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	connection.InitFlags(cmd.Flags(), connoptions)

	if err := cmd.Execute(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	var err error
	ctx := ctrl.SetupSignalHandler()
	scheme := runtime.NewScheme()

	// Adds the APIs to the scheme.
	for _, addToScheme := range addToSchemeFunctions {
		if err = addToScheme(scheme); err != nil {
			return fmt.Errorf("unable to add scheme: %w", err)
		}
	}

	// Set controller-runtime logger.
	log.SetLogger(klog.NewKlogr())

	// Get the rest config.
	cfg := config.GetConfigOrDie()

	// Create the manager.
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				connoptions.GwOptions.Namespace: {},
			},
		},
		Metrics: server.Options{
			BindAddress: "0", // Metrics are exposed by "connection" container.
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

	if connoptions.EnableConnectionController {
		// Setup the connection controller.
		connr, err := connection.NewConnectionsReconciler(
			ctx,
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

	// Setup the firewall configuration controller.
	fwcr, err := firewall.NewFirewallConfigurationReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("firewall-controller"),
		remapping.ForgeFirewallTargetLabels(connoptions.GwOptions.RemoteClusterID),
		false,
	)
	if err != nil {
		return fmt.Errorf("unable to create firewall configuration reconciler: %w", err)
	}

	if err := fwcr.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup firewall configuration reconciler: %w", err)
	}

	// Setup the configuration controller.
	cfgr := remapping.NewRemappingReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("firewall-controller"),
		remapoptions,
	)

	if err := cfgr.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup configuration reconciler: %w", err)
	}

	// Start the manager.
	return mgr.Start(ctx)
}

func getDefaultInterfaceName() (string, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
		Dst: nil,
	}, netlink.RT_FILTER_DST)
	if err != nil {
		return "", err
	}
	if len(routes) == 0 {
		return "", fmt.Errorf("no default route found")
	}
	link, err := netlink.LinkByIndex(routes[0].LinkIndex)
	if err != nil {
		return "", err
	}
	if link == nil {
		return "", fmt.Errorf("no default interface found")
	}
	return link.Attrs().Name, err
}

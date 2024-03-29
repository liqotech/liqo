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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/connection"
	"github.com/liqotech/liqo/pkg/gateway/connection/conncheck"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/remapping"
	"github.com/liqotech/liqo/pkg/route"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/kernel"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	connoptions  *connection.Options
	remapoptions *remapping.Options
	scheme       = runtime.NewScheme()
)

func init() {
	utilruntime.Must(networkingv1alpha1.AddToScheme(scheme))
}

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;delete

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-gateway",
		RunE: run,
	}

	legacyflags := flag.NewFlagSet("legacy", flag.ExitOnError)
	restcfg.InitFlags(legacyflags)
	klog.InitFlags(legacyflags)
	flagsutils.FromFlagToPflag(legacyflags, cmd.Flags())

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

	if err := cmd.Execute(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	var err error

	// Enable ip_forwarding.
	if err = kernel.EnableIPForwarding(); err != nil {
		return err
	}

	// Set controller-runtime logger.
	log.SetLogger(klog.NewKlogr())

	// Get the rest config.
	cfg := config.GetConfigOrDie()

	// Create the manager.
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
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

	if err := rcr.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup routeconfiguration reconciler: %w", err)
	}

	// Setup the firewall configuration controller.
	fwcr, err := firewall.NewFirewallConfigurationReconcilerWithoutFinalizer(
		mgr.GetClient(),
		mgr.GetScheme(),
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

	if err := fwcr.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup firewall configuration reconciler: %w", err)
	}

	// Start the manager.
	return mgr.Start(cmd.Context())
}

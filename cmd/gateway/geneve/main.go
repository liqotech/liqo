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
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/cleanup"
	"github.com/liqotech/liqo/pkg/gateway/concurrent"
	"github.com/liqotech/liqo/pkg/gateway/fabric"
	"github.com/liqotech/liqo/pkg/gateway/fabric/geneve"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	scheme  = runtime.NewScheme()
	options = fabric.NewOptions(gateway.NewOptions())
)

func init() {
	utilruntime.Must(networkingv1beta1.AddToScheme(scheme))
}

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-geneve",
		RunE: run,
	}

	flagsutils.InitKlogFlags(cmd.Flags())
	restcfg.InitFlags(cmd.Flags())
	fabric.InitFlags(cmd.Flags(), options)

	gateway.InitFlags(cmd.Flags(), options.GwOptions)
	if err := gateway.MarkFlagsRequired(&cmd); err != nil {
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

	// Set controller-runtime logger.
	log.SetLogger(klog.NewKlogr())

	// Get the rest config.
	cfg := config.GetConfigOrDie()

	// Create the manager.
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: options.GwOptions.MetricsAddress,
		},
		HealthProbeBindAddress: options.GwOptions.ProbeAddr,
		LeaderElection:         false,
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

	inr, err := geneve.NewInternalNodeReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("internalnode-controller"),
		options,
	)
	if err != nil {
		return fmt.Errorf("unable to create internalnode reconciler: %w", err)
	}

	if err := inr.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup internalnode reconciler: %w", err)
	}

	runnableGuest, err := concurrent.NewRunnableGuest(options.GwOptions.ContainerName)
	if err != nil {
		return fmt.Errorf("unable to create runnable guest: %w", err)
	}
	if err := runnableGuest.Start(cmd.Context()); err != nil {
		return fmt.Errorf("unable to start runnable guest: %w", err)
	}
	defer runnableGuest.Close()

	runnableGeneveCleanup, err := cleanup.NewRunnableGeneveCleanup(mgr.GetClient(), options.GeneveCleanupInterval)
	if err != nil {
		return fmt.Errorf("unable to create runnable geneve cleanup: %w", err)
	}

	if err := mgr.Add(runnableGeneveCleanup); err != nil {
		return fmt.Errorf("unable to add geneve cleanup runnable: %w", err)
	}

	// Start the manager.
	return mgr.Start(cmd.Context())
}

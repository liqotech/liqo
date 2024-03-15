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
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/fabric"
	"github.com/liqotech/liqo/pkg/gateway/fabric/geneve"
	"github.com/liqotech/liqo/pkg/gateway/forge"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	scheme  = runtime.NewScheme()
	options = fabric.NewOptions(gateway.NewOptions())
)

func init() {
	utilruntime.Must(networkingv1alpha1.AddToScheme(scheme))
}

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-geneve",
		RunE: run,
	}

	legacyflags := flag.NewFlagSet("legacy", flag.ExitOnError)
	restcfg.InitFlags(legacyflags)
	klog.InitFlags(legacyflags)
	flagsutils.FromFlagToPflag(legacyflags, cmd.Flags())

	fabric.InitFlags(cmd.Flags(), options)
	if err := fabric.MarkFlagsRequired(&cmd); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

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
		LeaderElection:         options.GwOptions.LeaderElection,
		LeaderElectionID: fmt.Sprintf(
			"%s.%s.%s.genevegateway.liqo.io",
			forge.GatewayResourceName(options.GwOptions.Name), options.GwOptions.Namespace, options.GwOptions.Mode,
		),
		LeaderElectionNamespace:       options.GwOptions.Namespace,
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		LeaseDuration:                 &options.GwOptions.LeaderElectionLeaseDuration,
		RenewDeadline:                 &options.GwOptions.LeaderElectionRenewDeadline,
		RetryPeriod:                   &options.GwOptions.LeaderElectionRetryPeriod,
	})
	if err != nil {
		return fmt.Errorf("unable to create manager: %w", err)
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

	// Start the manager.
	return mgr.Start(cmd.Context())
}

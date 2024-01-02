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
	"net"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel/wireguard"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	addToSchemeFunctions = []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		networkingv1alpha1.AddToScheme,
		ipamv1alpha1.AddToScheme,
	}
	options = wireguard.NewOptions(gateway.NewOptions())
)

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-wireguard",
		RunE: run,
	}

	legacyflags := flag.NewFlagSet("legacy", flag.ExitOnError)
	restcfg.InitFlags(legacyflags)
	klog.InitFlags(legacyflags)
	flagsutils.FromFlagToPflag(legacyflags, cmd.Flags())

	gateway.InitFlags(cmd.Flags(), options.GwOptions)
	wireguard.InitFlags(cmd.Flags(), options)
	if err := wireguard.MarkFlagsRequired(&cmd, options); err != nil {
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

	// Create the client. This client should be used only outside the reconciler.
	// This client don't need a cache.
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
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				options.GwOptions.Namespace: {},
			},
		},
		Metrics: server.Options{
			BindAddress: options.GwOptions.MetricsAddress,
		},
		HealthProbeBindAddress: options.GwOptions.ProbeAddr,
		LeaderElection:         options.GwOptions.LeaderElection,
		LeaderElectionID: fmt.Sprintf(
			"%s.%s.%s.wgtunnel.liqo.io",
			gateway.GenerateResourceName(options.GwOptions.Name), options.GwOptions.Namespace, options.GwOptions.Mode,
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

	// Setup the controller.
	pkr, err := wireguard.NewPublicKeysReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("public-keys-controller"),
		options,
	)
	if err != nil {
		return fmt.Errorf("unable to create public keys reconciler: %w", err)
	}

	dnsChan := make(chan event.GenericEvent)
	if options.GwOptions.Mode == gateway.ModeClient {
		if wireguard.IsDNSRoutineRequired(options) {
			go wireguard.StartDNSRoutine(cmd.Context(), dnsChan, options)
			klog.Infof("Starting DNS routine: resolving the endpoint address every %s", options.DNSCheckInterval.String())
		} else {
			options.EndpointIP = net.ParseIP(options.EndpointAddress)
			klog.Infof("Setting static endpoint IP: %s", options.EndpointIP.String())
		}
	}

	// Setup the controller.
	if err = pkr.SetupWithManager(mgr, dnsChan); err != nil {
		return fmt.Errorf("unable to setup public keys reconciler: %w", err)
	}

	// Ensure presence of Secret with private and public keys.
	if err = wireguard.EnsureKeysSecret(cmd.Context(), cl, options); err != nil {
		return fmt.Errorf("unable to manage wireguard keys secret: %w", err)
	}

	// Create the wg-liqo interface and init the wireguard configuration depending on the mode (client/server).
	if err := wireguard.InitWireguardLink(options); err != nil {
		return fmt.Errorf("unable to init wireguard link: %w", err)
	}

	// Start the manager.
	return mgr.Start(cmd.Context())
}

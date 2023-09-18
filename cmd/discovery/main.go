// Copyright 2019-2023 The Liqo Authors
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

package main

import (
	"flag"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	discovery "github.com/liqotech/liqo/pkg/discoverymanager"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = advtypes.AddToScheme(scheme)
	_ = nettypes.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.Info("Starting")

	clusterFlags := args.NewClusterIdentityFlags(true, nil)
	namespace := flag.String("namespace", "default", "Namespace where your configs are stored.")
	requeueAfter := flag.Duration("requeue-after", 30*time.Second,
		"Period after that the PeeringRequests status is synchronized")

	var mdnsConfig discovery.MDNSConfig
	flag.BoolVar(&mdnsConfig.EnableAdvertisement, "mdns-enable-advertisement", false, "Enable the mDNS advertisement on LANs")
	flag.BoolVar(&mdnsConfig.EnableDiscovery, "mdns-enable-discovery", false, "Enable the mDNS discovery on LANs")
	flag.StringVar(&mdnsConfig.Service, "mdns-service-name", "_liqo_auth._tcp",
		"The name of the service used for mDNS advertisement/discovery on LANs")
	flag.StringVar(&mdnsConfig.Domain, "mdns-domain-name", "local.",
		"The name of the domain used for mDNS advertisement/discovery on LANs")
	flag.DurationVar(&mdnsConfig.TTL, "mdns-ttl", 90*time.Second,
		"The time-to-live before an automatically discovered clusters is deleted if no longer announced")
	flag.DurationVar(&mdnsConfig.ResolveRefreshTime, "mdns-resolve-refresh-time", 10*time.Minute,
		"Period after that mDNS resolve context is refreshed")

	dialTCPTimeout := flag.Duration("dial-tcp-timeout", 500*time.Millisecond,
		"Time to wait for a TCP connection to a remote cluster before to consider it as not reachable")

	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	log.SetLogger(klog.NewKlogr())

	clusterIdentity := clusterFlags.ReadOrDie()

	klog.Info("Namespace: ", *namespace)
	klog.Info("RequeueAfter: ", *requeueAfter)

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider:   mapper.LiqoMapperProvider(scheme),
		Scheme:           scheme,
		LeaderElection:   false,
		LeaderElectionID: "b3156c4e.liqo.io",
	})
	if err != nil {
		klog.Errorf("Unable to create main manager: %w", err)
		os.Exit(1)
	}

	// Create an accessory manager restricted to the given namespace only, to avoid introducing
	// performance overhead and requiring excessively wide permissions when not necessary.
	auxmgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: "0", // Disable the metrics of the auxiliary manager to prevent conflicts.
		},
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				*namespace: {},
			},
		},
	})
	if err != nil {
		klog.Errorf("Unable to create auxiliary (namespaced) manager: %w", err)
		os.Exit(1)
	}

	namespacedClient := client.NewNamespacedClient(auxmgr.GetClient(), *namespace)

	klog.Info("Starting the discovery logic")
	discoveryCtl := discovery.NewDiscoveryCtrl(mgr.GetClient(), namespacedClient, *namespace,
		clusterIdentity, mdnsConfig, *dialTCPTimeout)
	if err := mgr.Add(discoveryCtl); err != nil {
		klog.Errorf("Unable to add the discovery controller to the manager: %w", err)
		os.Exit(1)
	}

	if err := mgr.Add(auxmgr); err != nil {
		klog.Errorf("Unable to add the auxiliary manager to the main one: %w", err)
		os.Exit(1)
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Errorf("Unable to start manager: %w", err)
		os.Exit(1)
	}
}

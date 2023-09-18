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
	"context"
	"flag"
	"os"
	"sync"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	tunneloperator "github.com/liqotech/liqo/internal/liqonet/tunnel-operator"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/conncheck"
	liqonetns "github.com/liqotech/liqo/pkg/liqonet/netns"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/liqonet/utils/links"
	liqonetsignals "github.com/liqotech/liqo/pkg/liqonet/utils/signals"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

type gatewayOperatorFlags struct {
	enableLeaderElection bool
	leaseDuration        time.Duration
	renewDeadline        time.Duration
	retryPeriod          time.Duration
	tunnelMTU            uint
	tunnelListeningPort  uint
	updateStatusInterval time.Duration
	securityMode         *argsutils.StringEnum
}

func addGatewayOperatorFlags(liqonet *gatewayOperatorFlags) {
	liqonet.securityMode = argsutils.NewEnum([]string{string(liqoconst.FullPodToPodSecurityMode),
		string(liqoconst.IntraClusterTrafficSegregationSecurityMode)},
		string(liqoconst.FullPodToPodSecurityMode))
	flag.BoolVar(&liqonet.enableLeaderElection, "gateway.leader-elect", false,
		"leader-elect enables leader election for controller manager.")
	flag.DurationVar(&liqonet.leaseDuration, "gateway.lease-duration", 7*time.Second,
		"lease-duration is the duration that non-leader candidates will wait to force acquire leadership")
	flag.DurationVar(&liqonet.renewDeadline, "gateway.renew-deadline", 5*time.Second,
		"renew-deadline is the duration that the acting control plane will retry refreshing leadership before giving up")
	flag.DurationVar(&liqonet.retryPeriod, "gateway.retry-period", 2*time.Second,
		"retry-period is the duration the LeaderElector clients should wait between tries of actions")
	flag.UintVar(&liqonet.tunnelMTU, "gateway.mtu", liqoconst.DefaultMTU,
		"mtu is the maximum transmission unit for interfaces managed by the gateway operator")
	flag.UintVar(&liqonet.tunnelListeningPort, "gateway.listening-port", liqoconst.GatewayListeningPort,
		"listening-port is the port used by the vpn tunnel")
	flag.DurationVar(&liqonet.updateStatusInterval, "gateway.ping-latency-update-interval", 30*time.Second,
		"ping-latency-update-interval is the interval at which the gateway operator updates the latency value in the status of the tunnel-endpoint")
	flag.UintVar(&conncheck.PingLossThreshold, "gateway.ping-loss-threshold", 5,
		"ping-loss-threshold is the number of lost packets after which the connection check is considered as failed.")
	flag.DurationVar(&conncheck.PingInterval, "gateway.ping-interval", 2*time.Second,
		"ping-interval is the interval between two connection checks")
	flag.Var(liqonet.securityMode, "gateway.security-mode", "security-mode represents different security modes regarding connectivity among clusters")
}

func runGatewayOperator(commonFlags *liqonetCommonFlags, gatewayFlags *gatewayOperatorFlags) {
	ctx, _ := liqonetsignals.NotifyContextPosix(context.Background(), liqonetsignals.ShutdownSignals...)
	wg := sync.WaitGroup{}
	metricsAddr := commonFlags.metricsAddr
	enableLeaderElection := gatewayFlags.enableLeaderElection
	leaseDuration := gatewayFlags.leaseDuration
	renewDeadLine := gatewayFlags.renewDeadline
	retryPeriod := gatewayFlags.retryPeriod
	securityMode := liqoconst.SecurityModeType(gatewayFlags.securityMode.String())

	// If port is not in the correct range, then return an error.
	if gatewayFlags.tunnelListeningPort < liqoconst.UDPMinPort || gatewayFlags.tunnelListeningPort > liqoconst.UDPMaxPort {
		klog.Errorf("port %d should be greater than %d and minor than %d", gatewayFlags.tunnelListeningPort, liqoconst.UDPMinPort, liqoconst.UDPMaxPort)
		os.Exit(1)
	}
	port := gatewayFlags.tunnelListeningPort
	MTU := gatewayFlags.tunnelMTU
	updateStatusInterval := gatewayFlags.updateStatusInterval

	// Get the pod ip and parse to net.IP.
	podIP, err := liqonetutils.GetPodIP()
	if err != nil {
		klog.Errorf("unable to get podIP: %v", err)
		os.Exit(1)
	}
	podNamespace, err := liqonetutils.GetPodNamespace()
	if err != nil {
		klog.Errorf("unable to get pod namespace: %v", err)
		os.Exit(1)
	}

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	main, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              liqoconst.GatewayLeaderElectionID,
		LeaderElectionNamespace:       podNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		LeaseDuration:                 &leaseDuration,
		RenewDeadline:                 &renewDeadLine,
		RetryPeriod:                   &retryPeriod,
		NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			opts.ByObject = map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Field: fields.OneTermEqualSelector("metadata.namespace", podNamespace),
				},
			}
			return cache.New(config, opts)
		},
	})
	if err != nil {
		klog.Errorf("unable to get main manager: %s", err)
		os.Exit(1)
	}

	// Create a label selector to filter only the events for pods managed by a ShadowPod (i.e., remote offloaded pods).
	reqRemoteLiqoPods, err := labels.NewRequirement(liqoconst.ManagedByLabelKey, selection.Equals, []string{liqoconst.ManagedByShadowPodValue})
	utilruntime.Must(err)

	// Create an accessory manager that cache only pods managed by a ShadowPod (i.e., remote offloaded pods).
	// This manager caches only the pods that are offloaded from a remote cluster and are scheduled on this.
	auxmgrOffloadedPods, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics:        server.Options{BindAddress: "0"}, // Disable the metrics of the auxiliary manager to prevent conflicts.
		NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			opts.ByObject = map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Label: labels.NewSelector().Add(*reqRemoteLiqoPods),
				},
			}
			return cache.New(config, opts)
		},
	})
	if err != nil {
		klog.Errorf("Unable to create auxiliary manager: %w", err)
		os.Exit(1)
	}

	if err := main.Add(auxmgrOffloadedPods); err != nil {
		klog.Errorf("Unable to add the auxiliary manager to the main one: %w", err)
		os.Exit(1)
	}

	clientset := kubernetes.NewForConfigOrDie(main.GetConfig())
	eventRecorder := main.GetEventRecorderFor(liqoconst.LiqoGatewayOperatorName + "." + podIP.String())
	// This map is updated by the tunnel operator after a successful tunnel creation
	// and is consumed by the natmapping operator to check whether the tunnel is ready or not.
	var readyClustersMutex sync.Mutex
	readyClusters := make(map[string]struct{})
	// Create new network namespace for the gateway (gatewayNetns).
	// If the namespace already exists it will be deleted and recreated.
	// It is created here because both the tunnel-operator and the natmapping-operator
	// need to know the namespace to configure.
	gatewayNetns, err := liqonetns.CreateNetns(liqoconst.GatewayNetnsName)
	if err != nil {
		klog.Errorf("an error occurred while creating custom network namespace: %s", err.Error())
		os.Exit(1)
	}
	// Get the current network namespace (hostNetns).
	hostNetns, err := ns.GetCurrentNS()
	if err != nil {
		klog.Errorf("an error occurred while getting host network namespace: %s", err.Error())
		os.Exit(1)
	}
	klog.Infof("created custom network namespace {%s}", liqoconst.GatewayNetnsName)

	labelController := tunneloperator.NewLabelerController(podIP.String(), main.GetClient())
	if err = labelController.SetupWithManager(main); err != nil {
		klog.Errorf("unable to setup labeler controller: %s", err)
		os.Exit(1)
	}
	tunnelController, err := tunneloperator.NewTunnelController(ctx, &wg, podIP.String(), podNamespace, eventRecorder,
		clientset, main.GetClient(), &readyClustersMutex, readyClusters, gatewayNetns, hostNetns, int(MTU), int(port), updateStatusInterval, securityMode)
	// If something goes wrong while creating and configuring the tunnel controller
	// then make sure that we remove all the resources created during the create process.
	if err != nil {
		klog.Errorf("an error occurred while creating the tunnel controller: %v", err)
		klog.Info("cleaning up gateway network namespace")
		if err := liqonetns.DeleteNetns(liqoconst.GatewayNetnsName); err != nil {
			klog.Errorf("an error occurred while deleting netns {%s}: %v", liqoconst.GatewayNetnsName, err)
		}
		klog.Info("cleaning up wireguard tunnel interface")
		if err := links.DeleteIFaceByName(liqoconst.DeviceName); err != nil {
			klog.Errorf("an error occurred while deleting iface {%s}: %v", liqoconst.DriverName, err)
		}
		os.Exit(1)
	}
	if err = tunnelController.SetupWithManager(main); err != nil {
		klog.Errorf("unable to setup tunnel controller: %s", err)
		os.Exit(1)
	}
	natMappingController, err := tunneloperator.NewNatMappingController(main.GetClient(), &readyClustersMutex,
		readyClusters, gatewayNetns)
	if err != nil {
		klog.Errorf("an error occurred while creating the natmapping controller: %v", err)
		os.Exit(1)
	}
	if err = natMappingController.SetupWithManager(main); err != nil {
		klog.Errorf("unable to setup natmapping controller: %s", err)
		os.Exit(1)
	}

	if securityMode == liqoconst.IntraClusterTrafficSegregationSecurityMode {
		podsInfo := &sync.Map{}
		endpointslicesInfo := &sync.Map{}
		offloadedPodController, err := tunneloperator.NewOffloadedPodController(auxmgrOffloadedPods.GetClient(), gatewayNetns, podsInfo, endpointslicesInfo)
		if err != nil {
			klog.Errorf("an error occurred while creating the offloaded pod controller: %v", err)
			os.Exit(1)
		}
		if err = offloadedPodController.SetupWithManager(auxmgrOffloadedPods); err != nil {
			klog.Errorf("unable to setup offloaded pod controller: %s", err)
			os.Exit(1)
		}
		reflectedEndpointsliceController, err := tunneloperator.NewReflectedEndpointsliceController(
			main.GetClient(), main.GetScheme(), gatewayNetns, podsInfo, endpointslicesInfo)
		if err != nil {
			klog.Errorf("an error occurred while creating the reflected endpointslice controller: %v", err)
			os.Exit(1)
		}
		if err = reflectedEndpointsliceController.SetupWithManager(main); err != nil {
			klog.Errorf("unable to setup reflected endpointslice controller: %s", err)
			os.Exit(1)
		}
	}

	klog.Info("Starting manager as Tunnel-Operator")
	if err := main.Start(tunnelController.SetupSignalHandlerForTunnelOperator(ctx, &wg)); err != nil {
		klog.Errorf("unable to start tunnel controller: %s", err)
		os.Exit(1)
	}
}

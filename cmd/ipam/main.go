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

// Package main contains the main function for the Liqo controller manager.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	liqoipam "github.com/liqotech/liqo/pkg/ipam"
	"github.com/liqotech/liqo/pkg/leaderelection"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

const leaderElectorName = "liqo-ipam-leader-election"

var (
	scheme  = runtime.NewScheme()
	options = liqoipam.NewOptions()
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(ipamv1alpha1.AddToScheme(scheme))
}

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ipamstorages,verbs=get;list;watch;create;update;patch

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-ipam",
		RunE: run,
	}

	flagsutils.InitKlogFlags(cmd.Flags())
	restcfg.InitFlags(cmd.Flags())

	liqoipam.InitFlags(cmd.Flags(), options)
	if err := liqoipam.MarkFlagsRequired(&cmd, options); err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	var err error

	// The IpamStorage resource will be stored in the same namespace of the IPAM pod.
	podNamespace := os.Getenv("POD_NAMESPACE")

	// Set controller-runtime logger.
	log.SetLogger(klog.NewKlogr())

	// Get the rest config.
	cfg := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	// Get dynamic client.
	dynClient := dynamic.NewForConfigOrDie(cfg)

	// Setup IPAM.
	ipam := liqoipam.NewIPAM()

	startIPAMServer := func() {
		// Initialize and start IPAM server.
		if err = initializeIPAM(ipam, options, dynClient, podNamespace); err != nil {
			klog.Errorf("Failed to initialize IPAM: %s", err)
			os.Exit(1)
		}
	}

	stopIPAMServer := func() {
		ipam.Terminate()
	}

	ctx := ctrl.SetupSignalHandler()

	// If the lease is disabled, start IPAM server without leader election mechanism (i.e., do not support IPAM high-availability).
	if !options.LeaseEnabled {
		startIPAMServer()
		<-ctx.Done()
		stopIPAMServer()
		return nil
	}

	// Else, initialize the leader election mechanism to manage multiple replicas of the IPAM server running in active-passive mode.
	leaderelectionOpts := &leaderelection.Opts{
		PodInfo: leaderelection.PodInfo{
			PodName:        os.Getenv("POD_NAME"),
			Namespace:      podNamespace,
			DeploymentName: ptr.To(os.Getenv("DEPLOYMENT_NAME")),
		},
		LeaderElectorName: leaderElectorName,
		LeaseDuration:     options.LeaseDuration,
		RenewDeadline:     options.LeaseRenewDeadline,
		RetryPeriod:       options.LeaseRetryPeriod,
		InitCallback:      startIPAMServer,
		StopCallback:      stopIPAMServer,
		LabelLeader:       options.LabelLeader,
	}

	localClient := kubernetes.NewForConfigOrDie(cfg)
	eb := record.NewBroadcaster()
	eb.StartRecordingToSink(&corev1clients.EventSinkImpl{Interface: localClient.CoreV1().Events(corev1.NamespaceAll)})

	leaderElector, err := leaderelection.Init(leaderelectionOpts, cfg, eb)
	if err != nil {
		return err
	}

	// Start IPAM using leader election mechanism.
	leaderelection.Run(ctx, leaderElector)

	return nil
}

func initializeIPAM(ipam *liqoipam.IPAM, opts *liqoipam.Options, dynClient dynamic.Interface, namespace string) error {
	if ipam == nil {
		return fmt.Errorf("IPAM pointer is nil. Initialize it before calling this function")
	}

	if err := ipam.Init(liqoipam.Pools, dynClient, namespace); err != nil {
		return err
	}

	// Configure PodCIDR
	if err := ipam.SetPodCIDR(opts.PodCIDR.String()); err != nil {
		return err
	}

	// Configure ServiceCIDR
	if err := ipam.SetServiceCIDR(opts.ServiceCIDR.String()); err != nil {
		return err
	}

	// Configure additional network pools.
	for _, pool := range opts.AdditionalPools.StringList.StringList {
		if err := ipam.AddNetworkPool(pool); err != nil {
			return err
		}
	}

	// Configure reserved subnets.
	if err := ipam.SetReservedSubnets(opts.ReservedPools.StringList.StringList); err != nil {
		return err
	}

	if err := ipam.Serve(consts.IpamPort); err != nil {
		return err
	}

	return nil
}

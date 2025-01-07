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

// Package main contains the main function for the Liqo controller manager.
package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/ipam"
	"github.com/liqotech/liqo/pkg/leaderelection"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

const leaderElectorName = "liqo-ipam-leaderelection"

var (
	scheme  = runtime.NewScheme()
	options ipam.Options
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(ipamv1alpha1.AddToScheme(scheme))
}

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch

func main() {
	var cmd = cobra.Command{
		Use:  "liqo-ipam",
		RunE: run,
	}

	flagsutils.InitKlogFlags(cmd.Flags())
	restcfg.InitFlags(cmd.Flags())

	// Server options.
	cmd.Flags().IntVar(&options.ServerOpts.Port, "port", consts.IpamPort, "The port on which to listen for incoming gRPC requests.")
	cmd.Flags().DurationVar(&options.ServerOpts.SyncInterval, "sync-interval", consts.SyncInterval,
		"The interval at which the IPAM will synchronize the IPAM storage.")
	cmd.Flags().DurationVar(&options.ServerOpts.SyncGracePeriod, "sync-graceperiod", consts.SyncGracePeriod,
		"The grace period the sync routine wait before releasing an ip or a network.")
	cmd.Flags().BoolVar(&options.ServerOpts.GraphvizEnabled, "enable-graphviz", false, "Enable the graphviz output for the IPAM.")
	cmd.Flags().StringSliceVar(&options.ServerOpts.Pools, "pools", consts.PrivateAddressSpace,
		"The pools used by the IPAM to acquire Networks and IPs from. Default: private addesses space.",
	)

	// Leader election flags.
	cmd.Flags().BoolVar(&options.EnableLeaderElection, "leader-election", false, "Enable leader election for IPAM. "+
		"Enabling this will ensure there is only one active IPAM.")
	cmd.Flags().StringVar(&options.LeaderElectionNamespace, "leader-election-namespace", consts.DefaultLiqoNamespace,
		"The namespace in which the leader election lease will be created.")
	cmd.Flags().StringVar(&options.LeaderElectionName, "leader-election-name", leaderElectorName,
		"The name of the leader election lease.")
	cmd.Flags().DurationVar(&options.LeaseDuration, "lease-duration", 15*time.Second,
		"The duration that non-leader candidates will wait to force acquire leadership.")
	cmd.Flags().DurationVar(&options.RenewDeadline, "renew-deadline", 10*time.Second,
		"The duration that the acting IPAM will retry refreshing leadership before giving up.")
	cmd.Flags().DurationVar(&options.RetryPeriod, "retry-period", 5*time.Second,
		"The duration the LeaderElector clients should wait between tries of actions.")
	cmd.Flags().StringVar(&options.PodName, "pod-name", "",
		"The name of the pod running the IPAM service.")
	cmd.Flags().StringVar(&options.DeploymentName, "deployment-name", "", "The name of the deployment running the IPAM service.")

	utilruntime.Must(cmd.MarkFlagRequired("pod-name"))

	if err := cmd.Execute(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Set controller-runtime logger.
	log.SetLogger(klog.NewKlogr())

	// Get the rest config.
	cfg := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	// Get the client.
	cl, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return err
	}

	if options.EnableLeaderElection {
		if leader, err := leaderelection.Blocking(ctx, cfg, record.NewBroadcaster(), &leaderelection.Opts{
			PodInfo: leaderelection.PodInfo{
				PodName:        options.PodName,
				Namespace:      options.LeaderElectionNamespace,
				DeploymentName: &options.DeploymentName,
			},
			Client:            cl,
			LeaderElectorName: options.LeaderElectionName,
			LeaseDuration:     options.LeaseDuration,
			RenewDeadline:     options.RenewDeadline,
			RetryPeriod:       options.RetryPeriod,
			LabelLeader:       true,
		}); err != nil {
			return err
		} else if !leader {
			klog.Error("IPAM is not the leader, exiting")
			os.Exit(1)
		}
	}

	liqoIPAM, err := ipam.New(ctx, cl, &options.ServerOpts)
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", options.ServerOpts.Port))
	if err != nil {
		return err
	}

	server := grpc.NewServer()

	// Register health service
	grpc_health_v1.RegisterHealthServer(server, liqoIPAM.HealthServer)

	// Register IPAM service
	ipam.RegisterIPAMServer(server, liqoIPAM)

	if err := server.Serve(lis); err != nil { // we do not need to close the listener as Serve will close it when returning
		klog.Errorf("failed to serve: %v", err)
		return err
	}

	return nil
}

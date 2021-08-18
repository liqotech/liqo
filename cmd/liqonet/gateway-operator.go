package main

import (
	"flag"
	"os"
	"sync"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	tunneloperator "github.com/liqotech/liqo/internal/liqonet/tunnel-operator"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonetns "github.com/liqotech/liqo/pkg/liqonet/netns"
	tunnelwg "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/mapperUtils"
)

type gatewayOperatorFlags struct {
	enableLeaderElection bool
	leaseDuration        time.Duration
	renewDeadline        time.Duration
	retryPeriod          time.Duration
}

func addGatewayOperatorFlags(liqonet *gatewayOperatorFlags) {
	flag.BoolVar(&liqonet.enableLeaderElection, "gateway.leader-elect", false,
		"leader-elect enables leader election for controller manager.")
	flag.DurationVar(&liqonet.leaseDuration, "gateway.lease-duration", 7*time.Second,
		"lease-duration is the duration that non-leader candidates will wait to force acquire leadership")
	flag.DurationVar(&liqonet.renewDeadline, "gateway.renew-deadline", 5*time.Second,
		"renew-deadline is the duration that the acting control plane will retry refreshing leadership before giving up")
	flag.DurationVar(&liqonet.retryPeriod, "gateway.retry-period", 2*time.Second,
		"retry-period is the duration the LeaderElector clients should wait between tries of actions")
}

func runGatewayOperator(commonFlags *liqonetCommonFlags, gatewayFlags *gatewayOperatorFlags) {
	metricsAddr := commonFlags.metricsAddr
	enableLeaderElection := gatewayFlags.enableLeaderElection
	leaseDuration := gatewayFlags.leaseDuration
	renewDeadLine := gatewayFlags.renewDeadline
	retryPeriod := gatewayFlags.retryPeriod

	// Get the pod ip and parse to net.IP.
	podIP, err := utils.GetPodIP()
	if err != nil {
		klog.Errorf("unable to get podIP: %v", err)
		os.Exit(1)
	}
	podNamespace, err := utils.GetPodNamespace()
	if err != nil {
		klog.Errorf("unable to get pod namespace: %v", err)
		os.Exit(1)
	}
	main, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		MapperProvider:                mapperUtils.LiqoMapperProvider(scheme),
		Scheme:                        scheme,
		MetricsBindAddress:            metricsAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              liqoconst.GatewayLeaderElectionID,
		LeaderElectionNamespace:       podNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		LeaseDuration:                 &leaseDuration,
		RenewDeadline:                 &renewDeadLine,
		RetryPeriod:                   &retryPeriod,
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: tunneloperator.LabelSelector,
		}),
	})
	if err != nil {
		klog.Errorf("unable to get main manager: %s", err)
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
	tunnelController, err := tunneloperator.NewTunnelController(podIP.String(), podNamespace, eventRecorder,
		clientset, main.GetClient(), &readyClustersMutex, readyClusters, gatewayNetns, hostNetns)
	// If something goes wrong while creating and configuring the tunnel controller
	// then make sure that we remove all the resources created during the create process.
	if err != nil {
		klog.Errorf("an error occurred while creating the tunnel controller: %v", err)
		klog.Info("cleaning up gateway network namespace")
		if err := liqonetns.DeleteNetns(liqoconst.GatewayNetnsName); err != nil {
			klog.Errorf("an error occurred while deleting netns {%s}: %v", liqoconst.GatewayNetnsName, err)
		}
		klog.Info("cleaning up wireguard tunnel interface")
		if err := utils.DeleteIFaceByName(tunnelwg.DeviceName); err != nil {
			klog.Errorf("an error occurred while deleting iface {%s}: %v", tunnelwg.DriverName, err)
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

	klog.Info("Starting manager as Tunnel-Operator")
	if err := main.Start(tunnelController.SetupSignalHandlerForTunnelOperator()); err != nil {
		klog.Errorf("unable to start tunnel controller: %s", err)
		os.Exit(1)
	}
}

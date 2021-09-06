package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/liqotech/liqo/internal/liqonet/tunnelEndpointCreator"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/mapperUtils"
	"github.com/liqotech/liqo/pkg/utils/args"
)

type networkManagerFlags struct {
	podCIDR     string
	serviceCIDR string

	additionalPools args.StringList
	reservedPools   args.StringList
}

func addNetworkManagerFlags(managerFlags *networkManagerFlags) {
	flag.StringVar(&managerFlags.podCIDR, "manager.pod-cidr", "", "The subnet used by the cluster for the pods, in CIDR notation")
	flag.StringVar(&managerFlags.serviceCIDR, "manager.service-cidr", "", "The subnet used by the cluster for the pods, in services notation")
	flag.Var(&managerFlags.reservedPools, "manager.reserved-pools",
		"Private CIDRs slices used by the Kubernetes infrastructure, in addition to the pod and service CIDR (e.g., the node subnet).")
	flag.Var(&managerFlags.additionalPools, "manager.additional-pools",
		"Network pools used to map a cluster network into another one in order to prevent conflicts, in addition to standard private CIDRs.")
}

func validateNetworkManagerFlags(managerFlags *networkManagerFlags) error {
	cidrRegex := regexp.MustCompile(`^(\d{1,3}.){3}\d{1,3}(/(\d|[12]\d|3[012]))$`)

	if !cidrRegex.MatchString(managerFlags.podCIDR) {
		return fmt.Errorf("pod CIDR is empty or invalid (%q)", managerFlags.podCIDR)
	}

	if !cidrRegex.MatchString(managerFlags.serviceCIDR) {
		return fmt.Errorf("service CIDR is empty or invalid (%q)", managerFlags.serviceCIDR)
	}

	for _, pool := range managerFlags.reservedPools.StringList {
		if !cidrRegex.MatchString(pool) {
			return fmt.Errorf("reserved pool entry empty or invalid (%q)", pool)
		}
	}

	for _, pool := range managerFlags.additionalPools.StringList {
		if !cidrRegex.MatchString(pool) {
			return fmt.Errorf("additional pool entry empty or invalid (%q)", pool)
		}
	}

	return nil
}

func runEndpointCreatorOperator(commonFlags *liqonetCommonFlags, managerFlags *networkManagerFlags) {
	if err := validateNetworkManagerFlags(managerFlags); err != nil {
		klog.Errorf("Failed to parse flags: %s", err)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		MapperProvider:     mapperUtils.LiqoMapperProvider(scheme),
		Scheme:             scheme,
		MetricsBindAddress: commonFlags.metricsAddr,
	})
	if err != nil {
		klog.Errorf("unable to get manager: %s", err)
		os.Exit(1)
	}
	clientset := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	dynClient := dynamic.NewForConfigOrDie(mgr.GetConfig())

	ipam, err := initializeIPAM(dynClient, managerFlags)
	if err != nil {
		klog.Errorf("Failed to initialize IPAM: %w", err)
		os.Exit(1)
	}

	externalCIDR, err := ipam.GetExternalCIDR(utils.GetMask(managerFlags.podCIDR))
	if err != nil {
		klog.Errorf("Failed to initialize the external CIDR: %w", err)
		os.Exit(1)
	}

	podNamespace, err := utils.GetPodNamespace()
	if err != nil {
		klog.Errorf("unable to get pod namespace: %v", err)
		os.Exit(1)
	}
	r := &tunnelEndpointCreator.TunnelEndpointCreator{
		Client:                     mgr.GetClient(),
		Scheme:                     mgr.GetScheme(),
		ClientSet:                  clientset,
		DynClient:                  dynClient,
		Manager:                    mgr,
		Namespace:                  podNamespace,
		WaitConfig:                 &sync.WaitGroup{},
		Configured:                 make(chan bool, 1),
		ForeignClusterStartWatcher: make(chan bool, 1),
		ForeignClusterStopWatcher:  make(chan struct{}),
		IPManager:                  ipam,
		RetryTimeout:               30 * time.Second,

		PodCIDR:      managerFlags.podCIDR,
		ExternalCIDR: externalCIDR,
	}

	r.WaitConfig.Add(2)

	if err = r.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to create controller controller TunnelEndpointCreator: %s", err)
		os.Exit(1)
	}

	go r.StartForeignClusterWatcher()
	go r.StartServiceWatcher()
	go r.StartSecretWatcher()

	klog.Info("starting manager as tunnelEndpointCreator-operator")
	if err := mgr.Start(r.SetupSignalHandlerForTunEndCreator()); err != nil {
		klog.Errorf("an error occurred while starting manager: %s", err)
		os.Exit(1)
	}
}

func initializeIPAM(client dynamic.Interface, managerFlags *networkManagerFlags) (*liqonetIpam.IPAM, error) {
	ipam := liqonetIpam.NewIPAM()

	if err := ipam.Init(liqonetIpam.Pools, client, liqoconst.NetworkManagerIpamPort); err != nil {
		return nil, err
	}

	if err := ipam.SetPodCIDR(managerFlags.podCIDR); err != nil {
		return nil, err
	}
	if err := ipam.SetServiceCIDR(managerFlags.serviceCIDR); err != nil {
		return nil, err
	}

	for _, pool := range managerFlags.additionalPools.StringList {
		if err := ipam.AddNetworkPool(pool); err != nil {
			return nil, err
		}
	}

	for _, pool := range managerFlags.reservedPools.StringList {
		if err := ipam.AcquireReservedSubnet(pool); err != nil {
			return nil, err
		}
	}

	return ipam, nil
}

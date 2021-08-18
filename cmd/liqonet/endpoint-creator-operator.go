package main

import (
	"os"
	"sync"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterConfig "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/internal/liqonet/tunnelEndpointCreator"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/mapperUtils"
)

func runEndpointCreatorOperator(commonFlags *liqonetCommonFlags) {
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
	ipam := liqonetIpam.NewIPAM()
	err = ipam.Init(liqonetIpam.Pools, dynClient, liqoconst.NetworkManagerIpamPort)
	if err != nil {
		klog.Errorf("cannot init IPAM:%w", err)
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
		ReservedSubnets:            make([]string, 0),
		AdditionalPools:            make([]string, 0),
		Configured:                 make(chan bool, 1),
		ForeignClusterStartWatcher: make(chan bool, 1),
		ForeignClusterStopWatcher:  make(chan struct{}),
		IPManager:                  ipam,
		RetryTimeout:               30 * time.Second,
	}
	r.WaitConfig.Add(3)
	// starting configuration watcher
	config, err := ctrl.GetConfig()
	if err != nil {
		klog.Error(err)
		os.Exit(2)
	}
	r.WatchConfiguration(config, &clusterConfig.GroupVersion)
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

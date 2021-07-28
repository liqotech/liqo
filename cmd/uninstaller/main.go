package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/uninstaller"
	"github.com/liqotech/liqo/pkg/utils"
)

// cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=get;list;watch;
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;patch;update;delete;deletecollection;

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()
	kubeconfigPath, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		kubeconfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	klog.Infof("Loading dynamic client: %s", kubeconfigPath)
	config, err := utils.GetRestConfig(kubeconfigPath)
	if err != nil {
		klog.Errorf("Unable to create client config: %s", err)
		os.Exit(1)
	}

	client := dynamic.NewForConfigOrDie(config)
	klog.Infof("Loaded dynamic client: %s", kubeconfigPath)

	if err = uninstaller.DisableDiscoveryAndPeering(ctx, client); err != nil {
		klog.Errorf("Unable to deactivate discovery mechanism: %s", err)
		os.Exit(1)
	}
	klog.Info("Outgoing Resource sharing has been disabled")

	// Trigger unjoin clusters
	err = uninstaller.UnjoinClusters(ctx, client)
	if err != nil {
		klog.Errorf("Unable to unjoin from peered clusters: %s", err)
		os.Exit(1)
	}
	klog.Info("Foreign Cluster unjoin operation has been correctly performed")

	if err := uninstaller.WaitForResources(client); err != nil {
		klog.Errorf("Unable to wait deletion of objects: %s", err)
		os.Exit(1)
	}

	if err := uninstaller.DeleteAllForeignClusters(ctx, client); err != nil {
		klog.Errorf("Unable to delete foreign clusters: %s", err)
		os.Exit(1)
	}
}

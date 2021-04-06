package main

import (
	"flag"
	"github.com/liqotech/liqo/pkg/uninstaller"
	"github.com/pkg/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"os"
	"path/filepath"
)

func main() {
	var kubeconfigPath string
	var config *rest.Config
	klog.Info("Loading client config")
	flag.StringVar(&kubeconfigPath, "kubeconfigPath", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "For debug purpose, set path to local kubeconfig")
	flag.Parse()

	klog.Infof("Loading dynamic client: %s", kubeconfigPath)
	config, err := userConfig(kubeconfigPath)
	if err != nil {
		klog.Errorf("Unable to create client config: %s", err)
		os.Exit(1)
	}

	client := dynamic.NewForConfigOrDie(config)
	klog.Infof("Loaded dynamic client: %s", kubeconfigPath)

	// Trigger unjoin clusters
	err = uninstaller.UnjoinClusters(client)
	if err != nil {
		klog.Errorf("Unable to unjoin from peered clusters: %s", err)
		os.Exit(1)
	}
	klog.Info("Foreign Cluster unjoin operation has been correctly performed")

	if err = uninstaller.DisableBroadcasting(client); err != nil {
		klog.Errorf("Unable to deactivate outgoing resource sharing: %s", err)
		os.Exit(1)
	}
	klog.Info("Outgoing Resource sharing has been disabled")

	if err := uninstaller.WaitForResources(client); err != nil {
		klog.Errorf("Unable to wait deletion of objects: %s", err)
		os.Exit(1)
	}
}

func userConfig(configPath string) (*rest.Config, error) {
	var config *rest.Config
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, errors.Wrap(err, "error building Client config")
		}
	} else {
		// Set to in-cluster config.
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, "error building in cluster config")
		}
	}
	return config, nil
}

// Copyright 2019-2022 The Liqo Authors
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
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;update
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

	namespace, ok := os.LookupEnv("POD_NAMESPACE")
	if !ok {
		klog.Fatal("The POD_NAMESPACE environment variable is not set")
	}

	klog.Infof("Loading dynamic client: %s", kubeconfigPath)
	config, err := utils.GetRestConfig(kubeconfigPath)
	if err != nil {
		klog.Errorf("Unable to create client config: %s", err)
		os.Exit(1)
	}

	client := dynamic.NewForConfigOrDie(config)
	klog.Infof("Loaded dynamic client: %s", kubeconfigPath)

	// Stop the discovery component, to prevent the subsequent discovery of new clusters.
	// This is currently a workaround mostly to avoid problems in E2E tests.
	if err := uninstaller.ScaleDiscoveryDeployment(ctx, client, namespace); err != nil {
		klog.Warning("Failed to stop the discovery component")
	}

	// Trigger unjoin clusters
	err = uninstaller.UnjoinClusters(ctx, client)
	if err != nil {
		klog.Errorf("Unable to unjoin from peered clusters: %s", err)
		os.Exit(1)
	}
	klog.Info("Foreign Cluster unjoin operation has been correctly performed")

	if err := uninstaller.WaitForResources(client, uninstaller.PhaseUnpeering); err != nil {
		klog.Errorf("Unable to wait deletion of objects: %s", err)
		os.Exit(1)
	}

	if err := uninstaller.DeleteAllForeignClusters(ctx, client); err != nil {
		klog.Errorf("Unable to delete foreign clusters: %s", err)
		os.Exit(1)
	}

	// Wait for the foreign clusters to be effectively deleted, to allow releasing possible finalizers.
	if err := uninstaller.WaitForResources(client, uninstaller.PhaseCleanup); err != nil {
		klog.Errorf("Unable to wait deletion of objects: %s", err)
		os.Exit(1)
	}
}

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

package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/uninstaller"
	"github.com/liqotech/liqo/pkg/utils"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = liqov1beta1.AddToScheme(scheme)
	_ = offloadingv1beta1.AddToScheme(scheme)
	_ = ipamv1alpha1.AddToScheme(scheme)
	_ = networkingv1beta1.AddToScheme(scheme)
	_ = authv1beta1.AddToScheme(scheme)
}

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters,verbs=get;list;watch;patch;update;delete;deletecollection;
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes,verbs=get;list;watch;patch;update;delete;
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;patch;update;delete;
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch;patch;update;delete;
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch;patch;update;delete;
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch;patch;update;delete;
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices,verbs=get;list;watch;patch;update;delete;

func main() {
	log.SetLogger(klog.NewKlogr())

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

	dynClient := dynamic.NewForConfigOrDie(config)
	klog.Infof("Loaded dynamic client: %s", kubeconfigPath)

	// Get controller runtime client
	cl, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		klog.Errorf("unable to create the client: %s", err)
		os.Exit(1)
	}

	// Run pre-uninstall checks
	if err := utils.PreUninstall(ctx, cl); err != nil {
		klog.Errorf("Pre-uninstall checks failed: %v", err)
		os.Exit(1)
	}

	// Annotate the controller-manager deployment to signal the uninstall process.
	if err := uninstaller.AnnotateControllerManagerDeployment(ctx, dynClient, namespace); err != nil {
		klog.Errorf("Unable to annotate the controller-manager: %s", err)
		os.Exit(1)
	}

	if err := uninstaller.DeleteAllForeignClusters(ctx, dynClient); err != nil {
		klog.Errorf("Unable to delete foreign clusters: %s", err)
		os.Exit(1)
	}

	if err := uninstaller.DeleteInternalNodes(ctx, dynClient); err != nil {
		klog.Errorf("Unable to delete internal nodes: %s", err)
		os.Exit(1)
	}

	if err := uninstaller.DeleteIPs(ctx, dynClient); err != nil {
		klog.Errorf("Unable to delete IP addresses: %s", err)
		os.Exit(1)
	}

	if err := uninstaller.DeleteNetworks(ctx, dynClient); err != nil {
		klog.Errorf("Unable to delete Network CIDRs: %s", err)
		os.Exit(1)
	}

	// Wait for resources to be effectively deleted, to allow releasing possible finalizers.
	if err := uninstaller.WaitForResources(dynClient, uninstaller.PhaseCleanup); err != nil {
		klog.Errorf("Unable to wait deletion of objects: %s", err)
		os.Exit(1)
	}
}

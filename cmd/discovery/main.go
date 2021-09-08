// Copyright 2019-2021 The Liqo Authors
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
	"flag"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	foreignclusteroperator "github.com/liqotech/liqo/internal/discovery/foreign-cluster-operator"
	searchdomainoperator "github.com/liqotech/liqo/internal/discovery/search-domain-operator"
	"github.com/liqotech/liqo/pkg/clusterid"
	"github.com/liqotech/liqo/pkg/mapperUtils"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = advtypes.AddToScheme(scheme)
	_ = nettypes.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.Info("Starting")

	var namespace string
	var requeueAfter int64 // seconds
	var kubeconfigPath string
	var resolveContextRefreshTime int // minutes
	var dialTCPTimeout int64          // milliseconds

	flag.StringVar(&namespace, "namespace", "default", "Namespace where your configs are stored.")
	flag.Int64Var(&requeueAfter, "requeueAfter", 30, "Period after that PeeringRequests status is rechecked (seconds)")
	flag.StringVar(&kubeconfigPath,
		"kubeconfigPath", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "For debug purpose, set path to local kubeconfig")
	flag.IntVar(&resolveContextRefreshTime,
		"resolveContextRefreshTime", 10, "Period after that mDNS resolve context is refreshed (minutes)")
	flag.Int64Var(&dialTCPTimeout,
		"dialTcpTimeout", 500, "Time to wait for a TCP connection to a remote cluster before to consider it as not reachable (milliseconds)")

	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	klog.Info("Namespace: ", namespace)
	klog.Info("RequeueAfter: ", requeueAfter)

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("Failed to create a new Kubernetes client: %w", err)
		os.Exit(1)
	}

	localClusterID, err := clusterid.NewClusterIDFromClient(clientset)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
	err = localClusterID.SetupClusterID(namespace)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	discoveryCtl, err := discovery.NewDiscoveryCtrl(namespace, localClusterID, kubeconfigPath,
		resolveContextRefreshTime, time.Duration(dialTCPTimeout)*time.Millisecond)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	discoveryCtl.StartDiscovery()

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider:   mapperUtils.LiqoMapperProvider(scheme),
		Scheme:           scheme,
		LeaderElection:   false,
		LeaderElectionID: "b3156c4e.liqo.io",
	})
	if err != nil {
		klog.Errorf("Unable to create main manager: %w", err)
		os.Exit(1)
	}

	// Create an accessory manager restricted to the given namespace only, to avoid introducing
	// performance overhead and requiring excessively wide permissions when not necessary.
	auxmgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider:     mapperUtils.LiqoMapperProvider(scheme),
		Scheme:             scheme,
		Namespace:          namespace,
		MetricsBindAddress: "0", // Disable the metrics of the auxiliary manager to prevent conflicts.
	})
	if err != nil {
		klog.Errorf("Unable to create auxiliary (namespaced) manager: %w", err)
		os.Exit(1)
	}

	klog.Info("Starting SearchDomain operator")
	searchdomainoperator.StartOperator(mgr, time.Duration(requeueAfter)*time.Second, discoveryCtl)

	klog.Info("Starting ForeignCluster operator")
	namespacedClient := client.NewNamespacedClient(auxmgr.GetClient(), namespace)
	foreignclusteroperator.StartOperator(mgr, namespacedClient, clientset, namespace,
		time.Duration(requeueAfter)*time.Second, discoveryCtl, localClusterID)

	if err := mgr.Add(auxmgr); err != nil {
		klog.Errorf("Unable to add the auxiliary manager to the main one: %w", err)
		os.Exit(1)
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Errorf("Unable to start manager: %w", err)
		os.Exit(1)
	}
}

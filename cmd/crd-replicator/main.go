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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/mapperUtils"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	clusterID := flag.String("cluster-id", "", "The cluster ID of identifying the current cluster")

	restcfg.InitFlags(nil)
	klog.InitFlags(nil)

	flag.Parse()

	cfg := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		MapperProvider: mapperUtils.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Port:           9443,
		LeaderElection: false,
	})
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(-1)
	}
	// Create a clientSet.
	k8sClient := kubernetes.NewForConfigOrDie(cfg)

	namespaceManager := tenantnamespace.NewTenantNamespaceManager(k8sClient)
	dynClient := dynamic.NewForConfigOrDie(cfg)

	d := &crdreplicator.Controller{
		Scheme:              mgr.GetScheme(),
		Client:              mgr.GetClient(),
		ClientSet:           k8sClient,
		ClusterID:           *clusterID,
		RemoteDynClients:    make(map[string]dynamic.Interface),
		LocalDynClient:      dynClient,
		RegisteredResources: crdreplicator.GetResourcesToReplicate(),
		LocalWatchers:       make(map[string]chan struct{}),
		RemoteWatchers:      make(map[string]map[string]chan struct{}),
		NamespaceManager:    namespaceManager,
		IdentityReader: identitymanager.NewCertificateIdentityReader(
			k8sClient, *clusterID, namespaceManager),
		LocalToRemoteNamespaceMapper:     map[string]string{},
		RemoteToLocalNamespaceMapper:     map[string]string{},
		ClusterIDToLocalNamespaceMapper:  map[string]string{},
		ClusterIDToRemoteNamespaceMapper: map[string]string{},
	}
	if err = d.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to setup the crdreplicator-operator")
		os.Exit(1)
	}

	klog.Info("Starting crdreplicator-operator")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

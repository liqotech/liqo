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
	"os"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/args"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	// Manager flags
	metricsAddr := pflag.String("metrics-address", ":8082", "The address the metric endpoint binds to")

	clusterFlags := args.NewClusterIDFlags(true, nil)
	resyncPeriod := pflag.Duration("resync-period", 10*time.Hour, "The resync period for the informers")
	workers := pflag.Uint("workers", 1, "The number of workers managing the reflection of each remote cluster")

	restcfg.InitFlags(nil)
	flagsutils.InitKlogFlags(nil)

	pflag.Parse()

	log.SetLogger(klog.NewKlogr())

	ctx := ctrl.SetupSignalHandler()
	clusterID := clusterFlags.ReadOrDie()

	cfg := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: *metricsAddr,
		},
		LeaderElection: false,
	})
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(-1)
	}
	// Create a clientSet.
	k8sClient := kubernetes.NewForConfigOrDie(cfg)

	namespaceManager := tenantnamespace.NewCachedManager(ctx, k8sClient, mgr.GetScheme())

	dynClient := dynamic.NewForConfigOrDie(cfg)

	reflectionManager := reflection.NewManager(dynClient, clusterID, *workers, *resyncPeriod)
	reflectionManager.Start(ctx, resources.GetResourcesToReplicate())

	d := &crdreplicator.Controller{
		Scheme:    mgr.GetScheme(),
		Client:    mgr.GetClient(),
		ClusterID: clusterID,

		RegisteredResources: resources.GetResourcesToReplicate(),
		ReflectionManager:   reflectionManager,
		Reflectors:          make(map[liqov1beta1.ClusterID]*reflection.Reflector),

		IdentityReader: identitymanager.NewCertificateIdentityReader(ctx,
			mgr.GetClient(), k8sClient, mgr.GetConfig(),
			clusterID, namespaceManager),
	}
	if err = d.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to setup the crdReplicator controller")
		os.Exit(1)
	}

	klog.Info("Starting crdReplicator manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Error(err, "unable to start the crdReplicator manager")
		os.Exit(1)
	}
}

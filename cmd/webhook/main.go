// Copyright 2019-2024 The Liqo Authors
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

// Package main contains the main function for the Liqo controller manager.
package main

import (
	"flag"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	liqov1alpha1 "github.com/liqotech/liqo/apis/core/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
	"github.com/liqotech/liqo/pkg/utils/indexer"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	fwcfgwh "github.com/liqotech/liqo/pkg/webhooks/firewallconfiguration"
	fcwh "github.com/liqotech/liqo/pkg/webhooks/foreigncluster"
	nsoffwh "github.com/liqotech/liqo/pkg/webhooks/namespaceoffloading"
	podwh "github.com/liqotech/liqo/pkg/webhooks/pod"
	resourceslicewh "github.com/liqotech/liqo/pkg/webhooks/resourceslice"
	routecfgwh "github.com/liqotech/liqo/pkg/webhooks/routeconfiguration"
	shadowpodswh "github.com/liqotech/liqo/pkg/webhooks/shadowpod"
	virtualnodewh "github.com/liqotech/liqo/pkg/webhooks/virtualnode"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = liqov1alpha1.AddToScheme(scheme)
	_ = offloadingv1alpha1.AddToScheme(scheme)
	_ = ipamv1alpha1.AddToScheme(scheme)
	_ = networkingv1alpha1.AddToScheme(scheme)
	_ = authv1alpha1.AddToScheme(scheme)
}

func main() {
	// Manager flags
	webhookPort := flag.Uint("webhook-port", 9443, "The port the webhook server binds to")
	metricsAddr := flag.String("metrics-address", ":8080", "The address the metric endpoint binds to")
	probeAddr := flag.String("health-probe-address", ":8081", "The address the health probe endpoint binds to")
	leaderElection := flag.Bool("enable-leader-election", false, "Enable leader election for controller manager")

	// Global parameters
	clusterIDFlags := argsutils.NewClusterIDFlags(true, nil)
	liqoNamespace := flag.String("liqo-namespace", consts.DefaultLiqoNamespace,
		"Name of the namespace where the liqo components are running")
	podcidr := flag.String("podcidr", "", "The CIDR to use for the pod network")
	vkOptsDefaultTemplate := flag.String("vk-options-default-template", "", "Namespaced name of the virtual-kubelet options template")
	enableResourceValidation := flag.Bool("enable-resource-enforcement", false,
		"Enforce offerer-side that offloaded pods do not exceed offered resources (based on container limits)")
	refreshInterval := flag.Duration("resource-validator-refresh-interval",
		5*time.Minute, "The interval at which the resource validator cache is refreshed")
	addVirtualNodeTolerationOnOffloadedPods := flag.Bool("add-virtual-node-toleration-on-offloaded-pods", false,
		"Automatically add the virtual node toleration on offloaded pods")

	liqoerrors.InitFlags(nil)
	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	log.SetLogger(klog.NewKlogr())

	clusterID := clusterIDFlags.ReadOrDie()

	ctx := ctrl.SetupSignalHandler()

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	// Create the main manager.
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		MapperProvider: mapper.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Metrics: server.Options{
			BindAddress: *metricsAddr,
		},
		HealthProbeBindAddress:        *probeAddr,
		LeaderElection:                *leaderElection,
		LeaderElectionID:              "66cf253f.webhook.liqo.io",
		LeaderElectionNamespace:       *liqoNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port: int(*webhookPort),
			},
		},
	})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	// Register the healthiness probes.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Errorf("Unable to set up healthz probe: %v", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Errorf("Unable to set up readyz probe: %v", err)
		os.Exit(1)
	}

	if err := indexer.IndexField(ctx, mgr, &corev1.Pod{}, indexer.FieldNodeNameFromPod, indexer.ExtractNodeName); err != nil {
		klog.Errorf("Unable to setup the indexer for the Pod nodeName field: %v", err)
		os.Exit(1)
	}

	spv := shadowpodswh.NewValidator(mgr.GetClient(), *enableResourceValidation)

	if err := mgr.Add(manager.RunnableFunc(spv.CacheRefresher(*refreshInterval))); err != nil {
		klog.Errorf("Unable to add the resource validator cache refresher to the manager: %v", err)
		os.Exit(1)
	}

	// Options for the virtual kubelet.
	vkOptsDefaultTemplateRef, err := argsutils.GetObjectRefFromNamespacedName(*vkOptsDefaultTemplate)
	if err != nil {
		klog.Errorf("Invalid namespaced name for virtual-kubelet options template %s: %v", *vkOptsDefaultTemplate, err)
		os.Exit(1)
	}

	// Register the webhooks.
	mgr.GetWebhookServer().Register("/mutate/foreign-cluster", fcwh.NewMutator())
	mgr.GetWebhookServer().Register("/validate/shadowpods", &webhook.Admission{Handler: spv})
	mgr.GetWebhookServer().Register("/mutate/shadowpods", shadowpodswh.NewMutator(mgr.GetClient()))
	mgr.GetWebhookServer().Register("/validate/namespace-offloading", nsoffwh.New())
	mgr.GetWebhookServer().Register("/mutate/pod", podwh.New(mgr.GetClient(), *addVirtualNodeTolerationOnOffloadedPods))
	mgr.GetWebhookServer().Register("/mutate/virtualnodes", virtualnodewh.New(
		mgr.GetClient(), clusterID, *podcidr, *liqoNamespace, vkOptsDefaultTemplateRef))
	mgr.GetWebhookServer().Register("/validate/resourceslices", resourceslicewh.NewValidator())
	mgr.GetWebhookServer().Register("/validate/firewallconfigurations", fwcfgwh.NewValidator(mgr.GetClient()))
	mgr.GetWebhookServer().Register("/mutate/firewallconfigurations", fwcfgwh.NewMutator())
	mgr.GetWebhookServer().Register("/validate/routeconfigurations", routecfgwh.NewValidator(mgr.GetClient()))

	// Start the manager.
	klog.Info("starting webhooks manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

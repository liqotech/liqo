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

// Package main contains the main function for the Liqo controller manager.
package main

import (
	"os"
	"time"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/leaderelection"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/indexer"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
	fwcfgwh "github.com/liqotech/liqo/pkg/webhooks/firewallconfiguration"
	fcwh "github.com/liqotech/liqo/pkg/webhooks/foreigncluster"
	nsoffwh "github.com/liqotech/liqo/pkg/webhooks/namespaceoffloading"
	podwh "github.com/liqotech/liqo/pkg/webhooks/pod"
	resourceslicewh "github.com/liqotech/liqo/pkg/webhooks/resourceslice"
	routecfgwh "github.com/liqotech/liqo/pkg/webhooks/routeconfiguration"
	"github.com/liqotech/liqo/pkg/webhooks/secretcontroller"
	shadowpodswh "github.com/liqotech/liqo/pkg/webhooks/shadowpod"
	tenantwh "github.com/liqotech/liqo/pkg/webhooks/tenant"
	virtualnodewh "github.com/liqotech/liqo/pkg/webhooks/virtualnode"
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

func main() {
	// Manager flags
	webhookPort := pflag.Uint("webhook-port", 9443, "The port the webhook server binds to")
	metricsAddr := pflag.String("metrics-address", ":8082", "The address the metric endpoint binds to")
	probeAddr := pflag.String("health-probe-address", ":8081", "The address the health probe endpoint binds to")
	leaderElection := pflag.Bool("enable-leader-election", false, "Enable leader election for the webhook pod")
	secretName := pflag.String("secret-name", "", "The name of the secret containing the webhook certificates")

	// Global parameters
	clusterIDFlags := argsutils.NewClusterIDFlags(true, nil)
	liqoNamespace := pflag.String("liqo-namespace", consts.DefaultLiqoNamespace,
		"Name of the namespace where the liqo components are running")
	podcidr := pflag.String("podcidr", "", "The CIDR to use for the pod network")
	vkOptsDefaultTemplate := pflag.String("vk-options-default-template", "", "Namespaced name of the virtual-kubelet options template")
	enableResourceValidation := pflag.Bool("enable-resource-enforcement", false,
		"Enforce offerer-side that offloaded pods do not exceed offered resources (based on container limits)")
	refreshInterval := pflag.Duration("resource-validator-refresh-interval",
		5*time.Minute, "The interval at which the resource validator cache is refreshed")
	liqoRuntimeClassName := pflag.String("liqo-runtime-class", consts.LiqoRuntimeClassName,
		"Define the Liqo runtime class forcing the pods to be scheduled on virtual nodes")

	flagsutils.InitKlogFlags(pflag.CommandLine)
	restcfg.InitFlags(pflag.CommandLine)

	pflag.Parse()

	log.SetLogger(klog.NewKlogr())

	clusterID := clusterIDFlags.ReadOrDie()

	ctx := ctrl.SetupSignalHandler()

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	// create a client used for configuration
	cl, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	// forge secret for the webhook
	if *secretName != "" {
		var secret corev1.Secret
		if err := cl.Get(ctx, client.ObjectKey{Namespace: *liqoNamespace, Name: *secretName}, &secret); err != nil {
			klog.Error(err)
			os.Exit(1)
		}

		if err := secretcontroller.HandleSecret(ctx, cl, &secret); err != nil {
			klog.Error(err)
			os.Exit(1)
		}

		if err := cl.Update(ctx, &secret); err != nil {
			klog.Error(err)
			os.Exit(1)
		}

		klog.Info("webhook secret correctly enforced")
	}

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
	mgr.GetWebhookServer().Register("/mutate/pod", podwh.New(mgr.GetClient(), *liqoRuntimeClassName))
	mgr.GetWebhookServer().Register("/mutate/virtualnodes", virtualnodewh.New(
		mgr.GetClient(), clusterID, *podcidr, *liqoNamespace, vkOptsDefaultTemplateRef))
	mgr.GetWebhookServer().Register("/validate/resourceslices", resourceslicewh.NewValidator(mgr.GetClient()))
	mgr.GetWebhookServer().Register("/validate/firewallconfigurations", fwcfgwh.NewValidator(mgr.GetClient()))
	mgr.GetWebhookServer().Register("/mutate/firewallconfigurations", fwcfgwh.NewMutator())
	mgr.GetWebhookServer().Register("/validate/routeconfigurations", routecfgwh.NewValidator(mgr.GetClient()))
	mgr.GetWebhookServer().Register("/validate/tenants", tenantwh.NewValidator(mgr.GetClient()))
	mgr.GetWebhookServer().Register("/mutate/tenants", tenantwh.NewMutator(mgr.GetClient()))

	// Register the secret controller
	secretReconciler := secretcontroller.NewSecretReconciler(mgr.GetClient(), mgr.GetScheme(),
		mgr.GetEventRecorderFor("secret-controller"))
	if err := secretReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to set up the secret controller: %v", err)
		os.Exit(1)
	}

	// Configure an indexer for the Tenant resource names
	// Add an index to the cache for a specific resource
	if err := mgr.GetFieldIndexer().IndexField(
		ctx,
		&authv1beta1.Tenant{},
		"metadata.name",
		tenantwh.NameExtractor,
	); err != nil {
		klog.Errorf("Unable to set up Tenant cache indexes: %v", err)
		os.Exit(1)
	}

	if leaderElection != nil && *leaderElection {
		leaderelection.LabelerOnElection(ctx, mgr, &leaderelection.PodInfo{
			PodName:        os.Getenv("POD_NAME"),
			Namespace:      os.Getenv("POD_NAMESPACE"),
			DeploymentName: ptr.To(os.Getenv("DEPLOYMENT_NAME")),
		})
	}

	// Start the manager.
	klog.Info("starting webhooks manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

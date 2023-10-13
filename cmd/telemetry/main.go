// Copyright 2019-2023 The Liqo Authors
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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/telemetry"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/json"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = offloadingv1alpha1.AddToScheme(scheme)
	_ = sharingv1alpha1.AddToScheme(scheme)
	_ = netv1alpha1.AddToScheme(scheme)
	_ = virtualkubeletv1alpha1.AddToScheme(scheme)
}

// cluster-role
// +kubebuilder:rbac:groups=core,resources=configmaps;nodes;pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers,verbs=get;list;watch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;list;watch
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=virtualnodes,verbs=get;list;watch

func main() {
	var clusterLabels argsutils.StringMap

	telemetryEndpoint := flag.String("telemetry-endpoint", "https://api.telemetry.liqo.io/v1", "telemetry endpoint")
	timeout := flag.Duration("timeout", 10*time.Second, "timeout for requests")
	namespace := flag.String("namespace", "liqo", "the namespace where liqo is deployed")
	liqoVersion := flag.String("liqo-version", "", "the liqo version")
	kubernetesVersion := flag.String("kubernetes-version", "", "the kubernetes version")
	securityMode := flag.String("security-mode", "", "the security mode of the cluster")
	dryRun := flag.Bool("dry-run", false, "if true, do not send the telemetry item and print it on stdout")
	flag.Var(&clusterLabels, consts.ClusterLabelsParameter,
		"The set of labels which characterizes the local cluster when exposed remotely as a virtual node")

	flag.Parse()

	log.SetLogger(klog.NewKlogr())

	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())
	restMapper, err := mapper.LiqoMapperProvider(scheme)(config, nil)
	if err != nil {
		klog.Errorf("unable to create mapper: %v", err)
		os.Exit(1)
	}

	cl, err := client.New(config, client.Options{
		Mapper: restMapper,
		Scheme: scheme,
	})
	if err != nil {
		klog.Errorf("failed to create client: %v", err)
		os.Exit(1)
	}

	builder := &telemetry.Builder{
		Client:            cl,
		Namespace:         *namespace,
		LiqoVersion:       *liqoVersion,
		KubernetesVersion: *kubernetesVersion,
		SecurityMode:      *securityMode,
		ClusterLabels:     clusterLabels.StringMap,
	}

	telemetryItem, err := builder.ForgeTelemetryItem(ctx)
	if err != nil {
		klog.Errorf("failed to forge telemetry item: %v", err)
		os.Exit(1)
	}

	if *dryRun {
		klog.Infof("dry-run enabled, telemetry item:")
		fmt.Println(json.Pretty(telemetryItem))
		return
	}
	err = telemetry.Send(ctx, *telemetryEndpoint, telemetryItem, *timeout)
	if err != nil {
		klog.Errorf("failed to send telemetry item: %v", err)
		// do not exit with code != 0, we want not to fail the job on network errors
	}
}

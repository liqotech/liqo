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

// Package main is the entrypoint of the metric-agent.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/remotemetrics"
	clientutils "github.com/liqotech/liqo/pkg/utils/clients"
	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// cluster-role
// +kubebuilder:rbac:groups=core,resources=nodes;pods;namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes/proxy,verbs=get

func main() {
	ctx := context.Background()

	keyPath := pflag.String("key-path", "server.key", "Path to the key file")
	certPath := pflag.String("cert-path", "server.crt", "Path to the certificate file")
	readTimeout := pflag.Duration("read-timeout", 0, "Read timeout")
	writeTimeout := pflag.Duration("write-timeout", 0, "Write timeout")
	port := pflag.Int("port", 8443, "Port to listen on")

	flagsutils.InitKlogFlags(pflag.CommandLine)
	restcfg.InitFlags(pflag.CommandLine)

	pflag.Parse()

	log.SetLogger(klog.NewKlogr())

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())
	kcl := kubernetes.NewForConfigOrDie(config)

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		klog.Errorf("error adding client-go scheme: %s", err)
		os.Exit(1)
	}

	liqoMapper, err := (mapper.LiqoMapperProvider(scheme))(config, nil)
	if err != nil {
		klog.Errorf("mapper: %s", err)
		os.Exit(1)
	}

	podsLabelRequirement, err := labels.NewRequirement(consts.ManagedByLabelKey,
		selection.Equals, []string{consts.ManagedByShadowPodValue})
	if err != nil {
		klog.Errorf("error creating label requirement: %s", err)
		os.Exit(1)
	}

	cacheOptions := &cache.Options{
		Scheme: scheme,
		Mapper: liqoMapper,
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Pod{}: {
				Label: labels.NewSelector().Add(*podsLabelRequirement),
			},
		},
	}
	if err != nil {
		klog.Errorf("error creating pod cache: %s", err)
		os.Exit(1)
	}

	cl, err := clientutils.GetCachedClientWithConfig(ctx, scheme, liqoMapper, config, cacheOptions)
	if err != nil {
		klog.Errorf("error creating client: %s", err)
		os.Exit(1)
	}

	router, err := remotemetrics.GetHTTPHandler(kcl.RESTClient(), cl)
	if err != nil {
		klog.Errorf("error creating http handler: %s", err)
		os.Exit(1)
	}

	server := http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      router,
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
	}

	klog.Infof("starting server on port %d", *port)
	if err := server.ListenAndServeTLS(*certPath, *keyPath); err != nil {
		klog.Errorf("error starting server: %s", err)
		os.Exit(1)
	}
}

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
	"flag"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/remotemetrics"
	cachedclient "github.com/liqotech/liqo/pkg/utils/cachedClient"
	"github.com/liqotech/liqo/pkg/utils/mapper"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// cluster-role
// +kubebuilder:rbac:groups=core,resources=nodes;pods;namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes/proxy,verbs=get

func main() {
	ctx := context.Background()

	keyPath := flag.String("key-path", "server.key", "Path to the key file")
	certPath := flag.String("cert-path", "server.crt", "Path to the certificate file")
	port := flag.Int("port", 8443, "Port to listen on")

	klog.InitFlags(nil)
	restcfg.InitFlags(nil)
	flag.Parse()

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())
	kcl := kubernetes.NewForConfigOrDie(config)

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	liqoMapper, err := (mapper.LiqoMapperProvider(scheme))(config)
	if err != nil {
		klog.Fatalf("mapper: %s", err)
	}

	podsLabelRequirement, err := labels.NewRequirement(consts.ManagedByLabelKey,
		selection.Equals, []string{consts.ManagedByShadowPodValue})
	utilruntime.Must(err)

	clientCache, err := cache.New(config, cache.Options{
		Scheme: scheme,
		Mapper: liqoMapper,
		SelectorsByObject: cache.SelectorsByObject{
			&corev1.Pod{}: {
				Label: labels.NewSelector().Add(*podsLabelRequirement),
			},
		},
	})
	if err != nil {
		klog.Fatalf("error creating cache: %s", err)
	}

	cl, err := cachedclient.GetCachedClientWithConfig(ctx, scheme, config, clientCache)
	if err != nil {
		klog.Fatal(err)
	}

	router, err := remotemetrics.GetHTTPHandler(kcl.RESTClient(), cl)
	if err != nil {
		klog.Fatal(err)
	}

	err = http.ListenAndServeTLS(fmt.Sprintf(":%d", *port), *certPath, *keyPath, router)
	if err != nil {
		klog.Fatal("ListenAndServe: ", err)
	}
}

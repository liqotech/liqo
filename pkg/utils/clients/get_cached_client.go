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

// Package clients contains utility methods to create and manage clients with custom features.
package clients

import (
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetCachedClientWithConfig returns a controller runtime client with the cache initialized only for the resources added to
// the scheme. The necessary rest.Config is passed as third parameter, it must not be nil.
func GetCachedClientWithConfig(ctx context.Context,
	scheme *runtime.Scheme, mapper meta.RESTMapper, conf *rest.Config, cacheOptions *ctrlcache.Options) (client.Client, error) {
	cache, err := ctrlcache.New(conf, *cacheOptions)
	if err != nil {
		return nil, fmt.Errorf("unable to create the pod cache: %w", err)
	}

	go func() {
		klog.Info("starting pod cache")
		if err := cache.Start(ctx); err != nil {
			klog.Errorf("error starting pod cache: %s", err)
			os.Exit(1)
		}
	}()

	klog.Info("waiting for pod cache sync")

	if ok := cache.WaitForCacheSync(ctx); !ok {
		return nil, fmt.Errorf("unable to sync pod cache")
	}

	klog.Info("pod cache synced")

	if conf == nil {
		return nil, fmt.Errorf("the rest.Config parameter is nil")
	}

	cl, err := client.New(conf, client.Options{
		Scheme: scheme,
		Mapper: mapper,
		Cache: &client.CacheOptions{
			Reader: cache,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("unable to create the client: %w", err)
	}

	return cl, nil
}

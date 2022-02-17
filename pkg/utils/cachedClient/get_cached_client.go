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

// Package cachedclient contains utility methods to create a new controller runtime client with cache.
package cachedclient

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/liqotech/liqo/pkg/utils/mapper"
)

// GetCachedClient returns a controller runtime client with the cache initialized only for the resources added to
// the scheme. The necessary rest.Config is generated inside this function.
func GetCachedClient(ctx context.Context, scheme *runtime.Scheme) (client.Client, error) {
	conf := ctrl.GetConfigOrDie()
	if conf == nil {
		err := fmt.Errorf("unable to get the config file")
		klog.Error(err)
		return nil, err
	}

	return GetCachedClientWithConfig(ctx, scheme, conf, nil)
}

// GetCachedClientWithConfig returns a controller runtime client with the cache initialized only for the resources added to
// the scheme. The necessary rest.Config is passed as third parameter, it must not be nil.
func GetCachedClientWithConfig(ctx context.Context,
	scheme *runtime.Scheme, conf *rest.Config, clientCache cache.Cache) (client.Client, error) {
	if conf == nil {
		err := fmt.Errorf("the rest.Config parameter is nil")
		klog.Error(err)
		return nil, err
	}

	liqoMapper, err := (mapper.LiqoMapperProvider(scheme))(conf)
	if err != nil {
		klog.Errorf("mapper: %s", err)
		return nil, err
	}

	if clientCache == nil || reflect.ValueOf(clientCache).IsNil() {
		clientCache, err = cache.New(conf, cache.Options{Scheme: scheme, Mapper: liqoMapper})
		if err != nil {
			klog.Errorf("cache: %s", err)
			return nil, err
		}
	}

	go func() {
		if err = clientCache.Start(ctx); err != nil {
			klog.Errorf("unable to start cache: %s", err)
		}
	}()

	newClient, err := cluster.DefaultNewClient(clientCache, conf, client.Options{Scheme: scheme, Mapper: liqoMapper})
	if err != nil {
		klog.Errorf("unable to create the client: %s", err)
		return nil, err
	}
	return newClient, nil
}

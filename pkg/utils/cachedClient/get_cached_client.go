// Package cachedclient contains utility methods to create a new controller runtime client with cache.
package cachedclient

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/liqotech/liqo/pkg/mapperUtils"
)

// GetCachedClient returns a controller runtime client with the cache initialized only for the resources added to
// the scheme.
func GetCachedClient(ctx context.Context, scheme *runtime.Scheme) (client.Client, error) {
	conf := ctrl.GetConfigOrDie()
	if conf == nil {
		err := fmt.Errorf("unable to get config file for cluster home")
		klog.Error(err)
		return nil, err
	}

	mapper, err := (mapperUtils.LiqoMapperProvider(scheme))(conf)
	if err != nil {
		klog.Errorf("mapper: %s", err)
		return nil, err
	}

	clientCache, err := cache.New(conf, cache.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		klog.Errorf("cache: %s", err)
		return nil, err
	}

	go func() {
		if err = clientCache.Start(ctx); err != nil {
			klog.Errorf("unable to start cache: %s", err)
		}
	}()

	newClient, err := cluster.DefaultNewClient(clientCache, conf, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		klog.Errorf("unable to create the client: %s", err)
		return nil, err
	}
	return newClient, nil
}

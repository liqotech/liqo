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

package wireguard

import (
	"context"
	"errors"
	"net"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// StartDNSRoutine run a routine which periodically resolves the DNS associated to the wireguard client endpoint.
// The DNS is resolved every 5 minutes.
// If the DNS changed a new publickkeys-controller reconcile is triggered through a generic event.
func StartDNSRoutine(ctx context.Context, ch chan event.GenericEvent, opts *Options) {
	// Try to solve the DNS every 5 seconds until the DNS is resolved for 10 minutes.
	// In some cases (like AWS LoadBalancer) the DNS is not immediatlly populated or can contain not working IPs.
	timeout, _ := context.WithTimeoutCause(ctx, time.Minute*10, context.DeadlineExceeded)
	err := wait.PollUntilContextCancel(timeout, time.Second*5, true, forgeResolveCallback(opts, ch))
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		klog.Error(err)
		os.Exit(1)
	}
	err = wait.PollUntilContextCancel(ctx, opts.DNSCheckInterval, true, forgeResolveCallback(opts, ch))
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func forgeResolveCallback(opts *Options, ch chan event.GenericEvent) func(_ context.Context) (done bool, err error) {
	return func(_ context.Context) (done bool, err error) {
		ips, err := net.LookupIP(opts.EndpointAddress)
		if err != nil {
			dnsErr := &net.DNSError{}
			if !errors.As(err, &dnsErr) {
				return false, err
			}
			switch {
			case dnsErr.IsNotFound:
				klog.Warningf("DNS %q not found", opts.EndpointAddress)
				return false, nil
			case dnsErr.IsTimeout:
				klog.Warningf("DNS %q timeout", opts.EndpointAddress)
				return false, nil
			default:
				return false, err
			}
		}

		// Checks if the DNS resolution has changed
		for _, ip := range ips {
			if opts.EndpointIP.Equal(ip) {
				return false, nil
			}
		}

		klog.Infof("DNS %q resolved to %q: updating endpoint", opts.EndpointAddress, ips[0])

		// Copies the new IPs to store for the next check
		opts.EndpointIPMutex.Lock()
		defer opts.EndpointIPMutex.Unlock()
		if len(ips) == 0 {
			return false, nil
		}
		opts.EndpointIP = ips[0]

		// Triggers a new reconcile
		ch <- event.GenericEvent{}

		return false, nil
	}
}

// IsDNSRoutineRequired checks if the client endpoint is a DNS.
// If it is a DNS the DNS routine is required.
func IsDNSRoutineRequired(opts *Options) bool {
	if opts.GwOptions.Mode != gateway.ModeClient {
		return false
	}
	return net.ParseIP(opts.EndpointAddress) == nil
}

// NewDNSSource creates a new Source for the DNS watcher.
func NewDNSSource(src <-chan event.GenericEvent, eh handler.EventHandler) source.Source {
	return source.Channel(src, eh)
}

// NewDNSEventHandler creates a new EventHandler.
func NewDNSEventHandler(cl client.Client, opts *Options) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			labelSet := labels.Set{
				string(consts.RemoteClusterID): opts.GwOptions.RemoteClusterID,
			}
			list, err := getters.ListPublicKeysByLabel(ctx, cl, opts.GwOptions.Namespace, labels.SelectorFromSet(labelSet))
			if err != nil {
				klog.Error(err)
				return nil
			}
			var requests []reconcile.Request
			for i := range list.Items {
				item := &list.Items[i]
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: item.Name, Namespace: item.Namespace}})
			}
			return requests
		})
}

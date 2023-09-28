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

package wireguard

import (
	"context"
	"errors"
	"net"
	"os"

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
	"github.com/liqotech/liqo/pkg/gateway/tunnel/common"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// StartDNSRoutine run a routine which periodically resolves the DNS associated to the wireguard client endpoint.
// The DNS is resolved every 5 minutes.
// If the DNS changed a new publickkeys-controller reconcile is triggered through a generic event.
func StartDNSRoutine(ctx context.Context, ch chan event.GenericEvent, opts *Options) {
	err := wait.PollUntilContextCancel(ctx, opts.DNSCheckInterval, true, func(ctx context.Context) (done bool, err error) {
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
	})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

// IsDNSRoutineRequired checks if the client endpoint is a DNS.
// If it is a DNS the DNS routine is required.
func IsDNSRoutineRequired(opts *Options) bool {
	if opts.Mode != common.ModeClient {
		return false
	}
	return net.ParseIP(opts.EndpointAddress) == nil
}

// NewDNSSource creates a new Source for the DNS watcher.
func NewDNSSource(src <-chan event.GenericEvent) *source.Channel {
	return &source.Channel{
		Source: src,
	}
}

// NewDNSEventHandler creates a new EventHandler.
func NewDNSEventHandler(cl client.Client, opts *Options) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			labelSet := labels.Set{
				string(LabelsMode):             string(opts.Mode),
				string(consts.RemoteClusterID): opts.RemoteClusterID,
			}
			list, err := getters.ListPublicKeysByLabel(ctx, cl, opts.Namespace, labels.SelectorFromSet(labelSet))
			if err != nil {
				klog.Error(err)
			}
			if len(list.Items) == 0 {
				klog.Errorf("There are no public keys with label %s", labelSet)
				return nil
			}
			if len(list.Items) != 1 {
				klog.Errorf("There are %d public keys with label %s", len(list.Items), labelSet)
				return nil
			}
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: list.Items[0].Name, Namespace: list.Items[0].Namespace}},
			}
		})
}

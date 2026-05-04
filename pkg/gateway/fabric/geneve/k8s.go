// Copyright 2019-2026 The Liqo Authors
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

package geneve

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/gateway/connection/conncheck"
	"github.com/liqotech/liqo/pkg/gateway/fabric"
	timeutils "github.com/liqotech/liqo/pkg/utils/time"
)

// ForgeUpdateGeneveTunnelCallback returns a conncheck.UpdateFunc that writes connectivity results to a GeneveTunnel status.
func ForgeUpdateGeneveTunnelCallback(ctx context.Context, cl client.Client,
	opts *fabric.Options, tunnelName, tunnelNamespace string) conncheck.UpdateFunc {
	return func(connected bool, latency time.Duration, timestamp time.Time) error {
		gt := &networkingv1beta1.GeneveTunnel{}
		if err := cl.Get(ctx, types.NamespacedName{Name: tunnelName, Namespace: tunnelNamespace}, gt); err != nil {
			return err
		}
		val := networkingv1beta1.ConnectionError
		if connected {
			val = networkingv1beta1.Connected
		}
		return UpdateGeneveTunnelStatus(ctx, cl, opts, gt, val, latency, timestamp)
	}
}

// UpdateGeneveTunnelStatus updates the status of a GeneveTunnel, throttled by PingUpdateStatusInterval.
func UpdateGeneveTunnelStatus(ctx context.Context, cl client.Client, opts *fabric.Options,
	gt *networkingv1beta1.GeneveTunnel, value networkingv1beta1.ConnectionStatusValue,
	latency time.Duration, timestamp time.Time) error {
	if gt.Status.Value != value ||
		timestamp.Sub(gt.Status.Latency.Timestamp.Time) > opts.PingUpdateStatusInterval {
		if gt.Status.Value != value {
			klog.Infof("changing genevetunnel %q status to %q", client.ObjectKeyFromObject(gt), value)
		}
		gt.Status.Latency = networkingv1beta1.ConnectionLatency{
			Value:     timeutils.FormatLatency(latency),
			Timestamp: metav1.NewTime(timestamp),
		}
		gt.Status.Value = value
		if err := cl.Status().Update(ctx, gt); err != nil {
			return fmt.Errorf("unable to update genevetunnel %q status: %w", client.ObjectKeyFromObject(gt), err)
		}
	}
	return nil
}

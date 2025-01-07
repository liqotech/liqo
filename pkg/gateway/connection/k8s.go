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

package connection

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	timeutils "github.com/liqotech/liqo/pkg/utils/time"
)

// UpdateConnectionStatus updates the status of a connection.
func UpdateConnectionStatus(ctx context.Context, cl client.Client, opts *Options, connection *networkingv1beta1.Connection,
	value networkingv1beta1.ConnectionStatusValue, latency time.Duration, timestamp time.Time) error {
	if connection.Status.Value != value ||
		timestamp.Sub(connection.Status.Latency.Timestamp.Time) > opts.PingUpdateStatusInterval {
		if connection.Status.Value != value {
			klog.Infof("changing connection %q status to %q",
				client.ObjectKeyFromObject(connection).String(), value)
		}
		connection.Status.Latency = networkingv1beta1.ConnectionLatency{
			Value:     timeutils.FormatLatency(latency),
			Timestamp: metav1.NewTime(timestamp),
		}
		connection.Status.Value = value
		if err := cl.Status().Update(ctx, connection); err != nil {
			return fmt.Errorf("unable to update connection %q: %w",
				client.ObjectKeyFromObject(connection).String(), err)
		}
	}
	return nil
}

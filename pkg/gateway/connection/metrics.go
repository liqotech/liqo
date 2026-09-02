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

package connection

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/liqotech/liqo/pkg/conncheck"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
)

const (
	driverLabel = "gateway"
)

// ObserveLatency returns a conncheck.PingObserver that records the round-trip latency
// into the prometheus metrics at the point of measurement (on each PONG).
func ObserveLatency(remoteClusterID string) conncheck.PingObserver {
	return tunnel.ObserveLatencyMetrics(tunnel.MetricsPeerLatency, tunnel.MetricsPeerLatencyHistogram, tunnel.MetricsPeerIsConnected,
		prometheus.Labels{
			tunnel.MetricsLabels[0]: driverLabel,
			tunnel.MetricsLabels[1]: remoteClusterID,
		})
}

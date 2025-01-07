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

package tunnel

import "github.com/prometheus/client_golang/prometheus"

// PrometheusMetrics is a struct that implements the prometheus.Collector interface's Describe method and other utilities.
type PrometheusMetrics struct{}

var (
	// MetricsPeerReceivedBytes is the metric that counts the number of bytes received from a given peer.
	MetricsPeerReceivedBytes *prometheus.Desc
	// MetricsPeerTransmittedBytes is the metric that counts the number of bytes transmitted to a given peer.
	MetricsPeerTransmittedBytes *prometheus.Desc
	// MetricsPeerLatency is the metric that exposes the latency towards a given peer.
	MetricsPeerLatency *prometheus.Desc
	// MetricsPeerIsConnected is the metric that outputs the connection status.
	MetricsPeerIsConnected *prometheus.Desc
	// MetricsLabels is the labels that are used for the metrics.
	MetricsLabels []string
)

// InitDefaultMetrics initializes the default metrics.
func init() {
	MetricsLabels = []string{"driver", "cluster_id"}

	MetricsPeerReceivedBytes = prometheus.NewDesc(
		"liqo_peer_receive_bytes_total",
		"Number of bytes received from a given peer.",
		MetricsLabels,
		nil,
	)

	MetricsPeerTransmittedBytes = prometheus.NewDesc(
		"liqo_peer_transmit_bytes_total",
		"Number of bytes transmitted to a given peer.",
		MetricsLabels,
		nil,
	)

	MetricsPeerLatency = prometheus.NewDesc(
		"liqo_peer_latency_us",
		"Round-trip latency of a given peer in microseconds.",
		MetricsLabels,
		nil,
	)

	MetricsPeerIsConnected = prometheus.NewDesc(
		"liqo_peer_is_connected",
		"Status of the connectivity to a given peer (true = Liqo tunnel is up and gateways are pinging each other).",
		MetricsLabels,
		nil,
	)
}

// Describe implements prometheus.Collector.
func (m *PrometheusMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- MetricsPeerReceivedBytes
	ch <- MetricsPeerTransmittedBytes
	ch <- MetricsPeerLatency
	ch <- MetricsPeerIsConnected
}

// MetricsErrorHandler is a function that handles metrics errors.
func (m *PrometheusMetrics) MetricsErrorHandler(err error, ch chan<- prometheus.Metric) {
	ch <- prometheus.NewInvalidMetric(MetricsPeerReceivedBytes, err)
	ch <- prometheus.NewInvalidMetric(MetricsPeerTransmittedBytes, err)
	ch <- prometheus.NewInvalidMetric(MetricsPeerLatency, err)
	ch <- prometheus.NewInvalidMetric(MetricsPeerIsConnected, err)
}

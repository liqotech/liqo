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

package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics is a struct that implements the prometheus.Collector interface's Describe method and other utilities.
type Metrics struct{}

var (
	// PeerReceivedBytes is the metric that counts the number of bytes received from a given peer.
	PeerReceivedBytes *prometheus.Desc
	// PeerTransmittedBytes is the metric that counts the number of bytes transmitted to a given peer.
	PeerTransmittedBytes *prometheus.Desc
	// PeerLatency is the metric that exposes the latency towards a given peer.
	PeerLatency *prometheus.Desc
	// PeerIsConnected is the metric that outputs the connection status.
	PeerIsConnected *prometheus.Desc
	// MetricsLabels is the labels that are used for the metrics.
	MetricsLabels []string
)

// InitDefaultMetrics initializes the default metrics.
func init() {
	MetricsLabels = []string{"driver", "device", "cluster_id", "cluster_name"}

	PeerReceivedBytes = prometheus.NewDesc(
		"liqo_peer_receive_bytes_total",
		"Number of bytes received from a given peer.",
		MetricsLabels,
		nil,
	)

	PeerTransmittedBytes = prometheus.NewDesc(
		"liqo_peer_transmit_bytes_total",
		"Number of bytes transmitted to a given peer.",
		MetricsLabels,
		nil,
	)

	PeerLatency = prometheus.NewDesc(
		"liqo_peer_latency_us",
		"Latency of a given peer in microseconds.",
		MetricsLabels,
		nil,
	)

	PeerIsConnected = prometheus.NewDesc(
		"liqo_peer_is_connected",
		"Checks if connection is working.",
		MetricsLabels,
		nil,
	)
}

// Describe implements prometheus.Collector.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- PeerReceivedBytes
	ch <- PeerTransmittedBytes
	ch <- PeerLatency
	ch <- PeerIsConnected
}

// MetricsErrorHandler is a function that handles metrics errors.
func (m *Metrics) MetricsErrorHandler(err error, ch chan<- prometheus.Metric) {
	ch <- prometheus.NewInvalidMetric(PeerReceivedBytes, err)
	ch <- prometheus.NewInvalidMetric(PeerTransmittedBytes, err)
	ch <- prometheus.NewInvalidMetric(PeerLatency, err)
	ch <- prometheus.NewInvalidMetric(PeerIsConnected, err)
}

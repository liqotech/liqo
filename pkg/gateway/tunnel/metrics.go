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

package tunnel

import (
	"math"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusMetrics is a struct that implements the prometheus.Collector interface's Describe method and other utilities.
type PrometheusMetrics struct{}

var (
	// MetricsPeerReceivedBytes is the metric that counts the number of bytes received from a given peer.
	MetricsPeerReceivedBytes *prometheus.Desc
	// MetricsPeerTransmittedBytes is the metric that counts the number of bytes transmitted to a given peer.
	MetricsPeerTransmittedBytes *prometheus.Desc
	// MetricsPeerLatency is the metric that exposes the latency towards a given peer.
	MetricsPeerLatency *prometheus.Desc
	// MetricsPeerLatencyHistogram is the metric that exposes the latency distribution towards a given peer.
	MetricsPeerLatencyHistogram *prometheus.HistogramVec
	// MetricsPeerIsConnected is the metric that outputs the connection status.
	MetricsPeerIsConnected *prometheus.Desc
	// MetricsLabels is the labels that are used for the metrics.
	MetricsLabels []string

	// GeneveMetricsLabels are the labels used for geneve tunnel metrics.
	// The labels are: internal_fabric, internal_node, namespace, remote_cluster_id.
	GeneveMetricsLabels []string
	// MetricsGeneveLatency is the metric that exposes the latency of a geneve tunnel.
	MetricsGeneveLatency *prometheus.Desc
	// MetricsGeneveLatencyHistogram is the metric that exposes the latency distribution of a geneve tunnel.
	MetricsGeneveLatencyHistogram *prometheus.HistogramVec
	// MetricsGeneveIsConnected is the metric that outputs the geneve tunnel connection status.
	MetricsGeneveIsConnected *prometheus.Desc
	// MetricsGeneveReceivedBytes is the metric that counts the number of bytes received through a geneve tunnel.
	MetricsGeneveReceivedBytes *prometheus.Desc
	// MetricsGeneveTransmittedBytes is the metric that counts the number of bytes transmitted through a geneve tunnel.
	MetricsGeneveTransmittedBytes *prometheus.Desc
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

	MetricsPeerLatencyHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "liqo_peer_latency_histogram_us",
		Help:    "Round-trip latency distribution of a given peer in microseconds.",
		Buckets: GenerateFocusBuckets(10000, 5000, 3, 16250, 8, 1.1892, 4),
	}, MetricsLabels)

	MetricsPeerIsConnected = prometheus.NewDesc(
		"liqo_peer_is_connected",
		"Status of the connectivity to a given peer (true = Liqo tunnel is up and gateways are pinging each other).",
		MetricsLabels,
		nil,
	)

	GeneveMetricsLabels = []string{"internal_fabric", "internal_node", "namespace", "remote_cluster_id"}

	MetricsGeneveLatency = prometheus.NewDesc(
		"liqo_geneve_latency_us",
		"Round-trip latency of a geneve tunnel in microseconds.",
		GeneveMetricsLabels,
		nil,
	)

	MetricsGeneveLatencyHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "liqo_geneve_latency_histogram_us",
		Help:    "Round-trip latency distribution of a geneve tunnel in microseconds.",
		Buckets: GenerateFocusBuckets(10000, 5000, 3, 16250, 8, 1.1892, 4),
	}, GeneveMetricsLabels)

	MetricsGeneveIsConnected = prometheus.NewDesc(
		"liqo_geneve_is_connected",
		"Status of the connectivity of a geneve tunnel (true = tunnel is up and nodes are pinging each other).",
		GeneveMetricsLabels,
		nil,
	)

	MetricsGeneveReceivedBytes = prometheus.NewDesc(
		"liqo_geneve_receive_bytes_total",
		"Number of bytes received through a geneve tunnel.",
		GeneveMetricsLabels,
		nil,
	)

	MetricsGeneveTransmittedBytes = prometheus.NewDesc(
		"liqo_geneve_transmit_bytes_total",
		"Number of bytes transmitted through a geneve tunnel.",
		GeneveMetricsLabels,
		nil,
	)
}

// Describe implements prometheus.Collector.
func (m *PrometheusMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- MetricsPeerReceivedBytes
	ch <- MetricsPeerTransmittedBytes
	ch <- MetricsPeerLatency
	ch <- MetricsPeerIsConnected
	MetricsPeerLatencyHistogram.Describe(ch)
}

// MetricsErrorHandler is a function that handles metrics errors.
func (m *PrometheusMetrics) MetricsErrorHandler(err error, ch chan<- prometheus.Metric) {
	ch <- prometheus.NewInvalidMetric(MetricsPeerReceivedBytes, err)
	ch <- prometheus.NewInvalidMetric(MetricsPeerTransmittedBytes, err)
	ch <- prometheus.NewInvalidMetric(MetricsPeerLatency, err)
	ch <- prometheus.NewInvalidMetric(MetricsPeerIsConnected, err)
}

// GenerateFocusBuckets builds a Prometheus-style histogram bucket layout that
// concentrates resolution where it matters most: the latency range you actually
// expect to observe. It does so by stitching together three independent
// sequences into a single, monotonically increasing slice of boundaries.
//
// The three phases are:
//
//  1. INITIAL (linear, wide steps) — covers the very low end of the range.
//     Useful when sub-millisecond or fast-path latencies are common and you
//     want a few coarse buckets to distinguish "very fast" from "fast".
//
//  2. CENTRAL (linear, narrow steps) — covers the "hot zone" where most
//     observations are expected to land. This is where you spend the bulk of
//     your bucket budget to maximize precision in the range that drives SLOs.
//
//  3. FINAL (exponential, growing steps) — covers the long tail of anomalous
//     or worst-case latencies. The multiplicative growth keeps the bucket
//     count low while still spanning one or more orders of magnitude.
//
// Parameters:
//
//   - lowStart, lowStep, lowCount: starting value, step size, and number of
//     buckets for the initial phase. The last bucket of this phase is
//     lowStart + (lowCount-1) * lowStep.
//   - midStep, midCount: step size and number of buckets for the central
//     phase. The central phase begins one midStep after the last bucket of
//     the initial phase, so the two phases connect without overlap or gap.
//   - highFactor, highCount: multiplicative factor and number of buckets for
//     the final phase. The final phase begins one highFactor multiple after
//     the last bucket of the central phase.
//
// The total number of buckets returned is lowCount + midCount + highCount.
//
// Example: GenerateFocusBuckets(1000, 9500, 3, 16250, 8, 1.1892, 4) produces
// 15 buckets that start at 1 ms, reach 20 ms after the initial phase, span
// 20–150 ms with 8 dense buckets in the central phase, and grow exponentially
// from 150 ms to ~300 ms in the final phase.
func GenerateFocusBuckets(
	lowStart, lowStep float64, lowCount int,
	midStep float64, midCount int,
	highFactor float64, highCount int,
) []float64 {
	var buckets []float64

	// 1. INITIAL PHASE (Wide - for very low values)
	current := lowStart
	for i := 0; i < lowCount; i++ {
		buckets = append(buckets, math.Floor(current))
		if i < lowCount-1 {
			current += lowStep
		}
	}

	// 2. CENTRAL PHASE (Narrow/Dense - for the "hot" latency zone)
	// Increment before the loop to avoid duplicating the last value of phase 1.
	current += midStep
	for i := 0; i < midCount; i++ {
		buckets = append(buckets, math.Floor(current))
		if i < midCount-1 {
			current += midStep
		}
	}

	// 3. FINAL PHASE (Exponential/Wide - for anomalous peaks)
	// Increment before the loop to avoid duplicating the last value of phase 2.
	current *= highFactor
	for i := 0; i < highCount; i++ {
		buckets = append(buckets, math.Floor(current))
		if i < highCount-1 {
			current *= highFactor
		}
	}

	return buckets
}

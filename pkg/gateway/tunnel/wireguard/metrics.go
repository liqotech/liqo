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
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.zx2c4.com/wireguard/wgctrl"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

const (
	implLabel        = "implementation"
	driverLabelValue = "wireguard"
)

var (
	// MetricsWgUserImpl is the metric that reports if wireguard is running in userspace mode.
	MetricsWgUserImpl = prometheus.NewDesc(
		"liqo_wireguard_implementation",
		"Wireguard used implementation",
		[]string{tunnel.MetricsLabels[0], implLabel},
		nil,
	)
)

var _ prometheus.Collector = &PrometheusCollector{}

// PrometheusCollector is a prometheus.Collector that collects Wireguard metrics.
type PrometheusCollector struct {
	tunnelMetrics tunnel.PrometheusMetrics
	clientwg      *wgctrl.Client
	clientctrl    client.Client

	metricsOptions *MetricsOptions
}

// MetricsOptions contains the options for the PrometheusCollector.
type MetricsOptions struct {
	RemoteClusterID  string
	Namespace        string
	WgImplementation WgImplementation
}

// NewPrometheusCollector creates a new PrometheusCollector.
func NewPrometheusCollector(clctrl client.Client, metricsOpts *MetricsOptions) (*PrometheusCollector, error) {
	clwg, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("cannot create Wireguard client: %w", err)
	}
	return &PrometheusCollector{
		tunnelMetrics:  tunnel.PrometheusMetrics{},
		clientwg:       clwg,
		clientctrl:     clctrl,
		metricsOptions: metricsOpts,
	}, nil
}

// Describe implements prometheus.Collector.
func (pc *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	pc.tunnelMetrics.Describe(ch)
	ch <- MetricsWgUserImpl
}

// Collect implements prometheus.Collector.
func (pc *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	device, err := pc.clientwg.Device(tunnel.TunnelInterfaceName)
	if err != nil {
		pc.tunnelMetrics.MetricsErrorHandler(fmt.Errorf("error collecting wireguard metrics: %w", err), ch)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		MetricsWgUserImpl,
		prometheus.GaugeValue,
		1,
		[]string{driverLabelValue, string(pc.metricsOptions.WgImplementation)}...,
	)

	if len(device.Peers) != 1 {
		pc.tunnelMetrics.MetricsErrorHandler(
			fmt.Errorf("error collecting wireguard metrics: gateway must have exactly 1 peer, it has %d", len(device.Peers)), ch)
		return
	}

	peer := device.Peers[0]

	labels := []string{driverLabelValue, pc.metricsOptions.RemoteClusterID}

	ctx := context.WithoutCancel(context.Background())
	conn, err := getters.GetConnectionByClusterIDInNamespace(ctx, pc.clientctrl,
		pc.metricsOptions.RemoteClusterID, pc.metricsOptions.Namespace)
	if err != nil {
		pc.tunnelMetrics.MetricsErrorHandler(fmt.Errorf("error collecting wireguard metrics: %w", err), ch)
		return
	}

	connected := isConnected(conn)
	var result float64
	if connected {
		result = 1
	}
	ch <- prometheus.MustNewConstMetric(
		tunnel.MetricsPeerIsConnected,
		prometheus.GaugeValue,
		result,
		labels...,
	)

	if connected {
		ch <- prometheus.MustNewConstMetric(
			tunnel.MetricsPeerReceivedBytes,
			prometheus.CounterValue,
			float64(peer.ReceiveBytes),
			labels...,
		)

		ch <- prometheus.MustNewConstMetric(
			tunnel.MetricsPeerTransmittedBytes,
			prometheus.CounterValue,
			float64(peer.TransmitBytes),
			labels...,
		)

		latency, err := getLatency(conn)
		if err != nil {
			ch <- prometheus.NewInvalidMetric(tunnel.MetricsPeerLatency, err)
		}
		ch <- prometheus.MustNewConstMetric(
			tunnel.MetricsPeerLatency,
			prometheus.GaugeValue,
			float64(latency.Microseconds()),
			labels...,
		)
	}
}

func isConnected(conn *networkingv1beta1.Connection) bool {
	return conn.Status.Value == networkingv1beta1.Connected
}

func getLatency(conn *networkingv1beta1.Connection) (time.Duration, error) {
	return time.ParseDuration(conn.Status.Latency.Value)
}

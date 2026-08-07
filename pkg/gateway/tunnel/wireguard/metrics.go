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

package wireguard

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/gateway/tunnel"
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

	// Interface-level statistics (labels: driver, cluster_id).

	// MetricsInterfaceRxPackets is the metric that counts the packets received on the wireguard interface.
	MetricsInterfaceRxPackets = prometheus.NewDesc(
		"liqo_wireguard_interface_rx_packets_total",
		"Total number of packets received on the Wireguard interface.",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsInterfaceTxPackets is the metric that counts the packets transmitted on the wireguard interface.
	MetricsInterfaceTxPackets = prometheus.NewDesc(
		"liqo_wireguard_interface_tx_packets_total",
		"Total number of packets transmitted on the Wireguard interface.",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsInterfaceRxDropped is the metric that counts the packets dropped on ingress on the wireguard interface.
	MetricsInterfaceRxDropped = prometheus.NewDesc(
		"liqo_wireguard_interface_rx_dropped_total",
		"Total number of received packets dropped on the Wireguard interface (e.g. no space in queues).",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsInterfaceTxDropped is the metric that counts the packets dropped on egress on the wireguard interface.
	MetricsInterfaceTxDropped = prometheus.NewDesc(
		"liqo_wireguard_interface_tx_dropped_total",
		"Total number of transmitted packets dropped on the Wireguard interface (e.g. no space in queues).",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsInterfaceRxErrors is the metric that counts the receive errors on the wireguard interface.
	MetricsInterfaceRxErrors = prometheus.NewDesc(
		"liqo_wireguard_interface_rx_errors_total",
		"Total number of receive errors on the Wireguard interface.",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsInterfaceTxErrors is the metric that counts the transmit errors on the wireguard interface.
	MetricsInterfaceTxErrors = prometheus.NewDesc(
		"liqo_wireguard_interface_tx_errors_total",
		"Total number of transmit errors on the Wireguard interface.",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsInterfaceRxFIFODropped is the metric that counts the RX FIFO buffer errors on the wireguard interface.
	MetricsInterfaceRxFIFODropped = prometheus.NewDesc(
		"liqo_wireguard_interface_rx_fifo_errors_total",
		"Total number of RX FIFO buffer errors on the Wireguard interface.",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsInterfaceTxFIFODropped is the metric that counts the TX FIFO buffer errors on the wireguard interface.
	MetricsInterfaceTxFIFODropped = prometheus.NewDesc(
		"liqo_wireguard_interface_tx_fifo_errors_total",
		"Total number of TX FIFO buffer errors on the Wireguard interface.",
		tunnel.MetricsLabels,
		nil,
	)

	// UDP socket statistics from /proc/net/snmp (labels: driver, cluster_id).
	// These counters are scoped to the gateway pod's network namespace, so they
	// describe the UDP stack used by the WireGuard kernel socket.

	// MetricsUDPRcvbufErrors counts datagrams dropped because the UDP socket
	// receive buffer was full (i.e. WireGuard could not drain packets fast enough).
	MetricsUDPRcvbufErrors = prometheus.NewDesc(
		"liqo_wireguard_udp_rcvbuf_errors_total",
		"Total number of UDP datagrams dropped due to a full socket receive buffer (RcvbufErrors).",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsUDPSndbufErrors counts datagrams dropped because the UDP socket send buffer was full.
	MetricsUDPSndbufErrors = prometheus.NewDesc(
		"liqo_wireguard_udp_sndbuf_errors_total",
		"Total number of UDP datagrams dropped due to a full socket send buffer (SndbufErrors).",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsUDPInErrors counts UDP datagrams dropped for other reasons (e.g. memory pressure).
	MetricsUDPInErrors = prometheus.NewDesc(
		"liqo_wireguard_udp_in_errors_total",
		"Total number of UDP datagrams dropped for reasons other than a full buffer (InErrors).",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsUDPNoPorts counts UDP datagrams received for a closed port.
	MetricsUDPNoPorts = prometheus.NewDesc(
		"liqo_wireguard_udp_no_ports_total",
		"Total number of UDP datagrams received for a closed port (NoPorts).",
		tunnel.MetricsLabels,
		nil,
	)

	// UDP socket queue and limit gauges (labels: driver, cluster_id).

	// MetricsUDPRxQueueBytes reports the current size of the WireGuard UDP socket receive queue.
	MetricsUDPRxQueueBytes = prometheus.NewDesc(
		"liqo_wireguard_udp_rx_queue_bytes",
		"Current number of bytes queued in the WireGuard UDP socket receive queue.",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsUDPTxQueueBytes reports the current size of the WireGuard UDP socket send queue.
	MetricsUDPTxQueueBytes = prometheus.NewDesc(
		"liqo_wireguard_udp_tx_queue_bytes",
		"Current number of bytes queued in the WireGuard UDP socket send queue.",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsUDPRmemMaxBytes reports the kernel maximum UDP socket receive buffer size.
	MetricsUDPRmemMaxBytes = prometheus.NewDesc(
		"liqo_wireguard_udp_rmem_max_bytes",
		"Maximum UDP socket receive buffer size configured for the gateway namespace (rmem_max).",
		tunnel.MetricsLabels,
		nil,
	)
	// MetricsUDPWmemMaxBytes reports the kernel maximum UDP socket send buffer size.
	MetricsUDPWmemMaxBytes = prometheus.NewDesc(
		"liqo_wireguard_udp_wmem_max_bytes",
		"Maximum UDP socket send buffer size configured for the gateway namespace (wmem_max).",
		tunnel.MetricsLabels,
		nil,
	)
)

var _ prometheus.Collector = &PrometheusCollector{}

// emitInvalidInterfaceMetrics reports an invalid metric for every interface-level
// descriptor, used when rtnetlink statistics cannot be retrieved.
func emitInvalidInterfaceMetrics(ch chan<- prometheus.Metric, err error) {
	ch <- prometheus.NewInvalidMetric(MetricsInterfaceRxPackets, err)
	ch <- prometheus.NewInvalidMetric(MetricsInterfaceTxPackets, err)
	ch <- prometheus.NewInvalidMetric(MetricsInterfaceRxDropped, err)
	ch <- prometheus.NewInvalidMetric(MetricsInterfaceTxDropped, err)
	ch <- prometheus.NewInvalidMetric(MetricsInterfaceRxErrors, err)
	ch <- prometheus.NewInvalidMetric(MetricsInterfaceTxErrors, err)
	ch <- prometheus.NewInvalidMetric(MetricsInterfaceRxFIFODropped, err)
	ch <- prometheus.NewInvalidMetric(MetricsInterfaceTxFIFODropped, err)
}

// emitInvalidUDPMetrics reports an invalid metric for every UDP /proc/net/snmp
// descriptor, used when the UDP summary cannot be read.
func emitInvalidUDPMetrics(ch chan<- prometheus.Metric, err error) {
	ch <- prometheus.NewInvalidMetric(MetricsUDPRcvbufErrors, err)
	ch <- prometheus.NewInvalidMetric(MetricsUDPSndbufErrors, err)
	ch <- prometheus.NewInvalidMetric(MetricsUDPInErrors, err)
	ch <- prometheus.NewInvalidMetric(MetricsUDPNoPorts, err)
}

// emitInvalidUDPQueueMetrics reports an invalid metric for the UDP socket queue
// gauges, used when INET_DIAG cannot retrieve the queue sizes.
func emitInvalidUDPQueueMetrics(ch chan<- prometheus.Metric, err error) {
	ch <- prometheus.NewInvalidMetric(MetricsUDPRxQueueBytes, err)
	ch <- prometheus.NewInvalidMetric(MetricsUDPTxQueueBytes, err)
}

// emitInvalidSocketBufferLimitMetrics reports an invalid metric for the socket
// buffer limit gauges, used when /proc/sys/net/core values cannot be read.
func emitInvalidSocketBufferLimitMetrics(ch chan<- prometheus.Metric, err error) {
	ch <- prometheus.NewInvalidMetric(MetricsUDPRmemMaxBytes, err)
	ch <- prometheus.NewInvalidMetric(MetricsUDPWmemMaxBytes, err)
}

// PrometheusCollector is a prometheus.Collector that collects Wireguard metrics.
type PrometheusCollector struct {
	clientwg       *wgctrl.Client
	metricsOptions *MetricsOptions
}

// MetricsOptions contains the options for the PrometheusCollector.
type MetricsOptions struct {
	RemoteClusterID  string
	Namespace        string
	WgImplementation WgImplementation
}

// NewPrometheusCollector creates a new PrometheusCollector.
func NewPrometheusCollector(metricsOpts *MetricsOptions) (*PrometheusCollector, error) {
	clwg, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("cannot create Wireguard client: %w", err)
	}
	return &PrometheusCollector{
		clientwg:       clwg,
		metricsOptions: metricsOpts,
	}, nil
}

// Describe implements prometheus.Collector.
func (pc *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- tunnel.MetricsPeerReceivedBytes
	ch <- tunnel.MetricsPeerTransmittedBytes
	ch <- MetricsWgUserImpl

	ch <- MetricsInterfaceRxPackets
	ch <- MetricsInterfaceTxPackets
	ch <- MetricsInterfaceRxDropped
	ch <- MetricsInterfaceTxDropped
	ch <- MetricsInterfaceRxErrors
	ch <- MetricsInterfaceTxErrors
	ch <- MetricsInterfaceRxFIFODropped
	ch <- MetricsInterfaceTxFIFODropped

	ch <- MetricsUDPRcvbufErrors
	ch <- MetricsUDPSndbufErrors
	ch <- MetricsUDPInErrors
	ch <- MetricsUDPNoPorts

	ch <- MetricsUDPRxQueueBytes
	ch <- MetricsUDPTxQueueBytes
	ch <- MetricsUDPRmemMaxBytes
	ch <- MetricsUDPWmemMaxBytes
}

// Collect implements prometheus.Collector.
func (pc *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	device, err := pc.clientwg.Device(tunnel.TunnelInterfaceName)
	if err != nil {
		err = fmt.Errorf("error collecting wireguard metrics: %w", err)
		ch <- prometheus.NewInvalidMetric(MetricsWgUserImpl, err)
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsPeerReceivedBytes, err)
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsPeerTransmittedBytes, err)
		emitInvalidInterfaceMetrics(ch, err)
		emitInvalidUDPMetrics(ch, err)
		emitInvalidUDPQueueMetrics(ch, err)
		emitInvalidSocketBufferLimitMetrics(ch, err)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		MetricsWgUserImpl,
		prometheus.GaugeValue,
		1,
		[]string{driverLabelValue, string(pc.metricsOptions.WgImplementation)}...,
	)

	if len(device.Peers) != 1 {
		err := fmt.Errorf("error collecting wireguard metrics: gateway must have exactly 1 peer, it has %d", len(device.Peers))
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsPeerReceivedBytes, err)
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsPeerTransmittedBytes, err)
		emitInvalidInterfaceMetrics(ch, err)
		emitInvalidUDPMetrics(ch, err)
		emitInvalidUDPQueueMetrics(ch, err)
		emitInvalidSocketBufferLimitMetrics(ch, err)
		return
	}

	peer := device.Peers[0]

	labels := []string{driverLabelValue, pc.metricsOptions.RemoteClusterID}

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

	pc.collectInterfaceStats(ch, labels)
	pc.collectUDPStats(ch, labels)
	pc.collectUDPQueueStats(ch, labels, device.ListenPort)
	pc.collectSocketBufferLimits(ch, labels)
}

// collectUDPStats collects UDP protocol statistics from /proc/net/snmp,
// scoped to the gateway pod's network namespace. RcvbufErrors/SndbufErrors
// growth indicates that the WireGuard UDP socket buffer is saturating.
func (pc *PrometheusCollector) collectUDPStats(ch chan<- prometheus.Metric, labels []string) {
	summary, err := readUDPSummary()
	if err != nil {
		err = fmt.Errorf("error collecting UDP statistics: %w", err)
		emitInvalidUDPMetrics(ch, err)
		return
	}

	ch <- prometheus.MustNewConstMetric(MetricsUDPRcvbufErrors, prometheus.CounterValue, float64(summary.rcvbuf), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsUDPSndbufErrors, prometheus.CounterValue, float64(summary.sndbuf), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsUDPInErrors, prometheus.CounterValue, float64(summary.inErrors), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsUDPNoPorts, prometheus.CounterValue, float64(summary.noPorts), labels...)
}

// collectUDPQueueStats collects the current send/receive queue sizes of the
// WireGuard UDP socket via the kernel's INET_DIAG netlink interface. The
// listen port reported by the wireguard device is used to locate the correct
// socket row. When the lookup fails (e.g. userspace wireguard without a
// kernel-bound socket, missing CAP_NET_ADMIN, or a transient unbound state),
// the metrics are reported as invalid so the scrape failure is visible in
// Prometheus instead of being masked as zero queue bytes.
func (pc *PrometheusCollector) collectUDPQueueStats(ch chan<- prometheus.Metric, labels []string, listenPort int) {
	queues, err := readUDPQueueStats(listenPort)
	if err != nil {
		err = fmt.Errorf("UDP queue statistics unavailable for WireGuard device on port %d: %w", listenPort, err)
		klog.Warningf("%v", err)
		emitInvalidUDPQueueMetrics(ch, err)
		return
	}

	ch <- prometheus.MustNewConstMetric(MetricsUDPRxQueueBytes, prometheus.GaugeValue, float64(queues.rxBytes), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsUDPTxQueueBytes, prometheus.GaugeValue, float64(queues.txBytes), labels...)
}

// collectSocketBufferLimits exposes the kernel-configured maximum UDP socket
// buffer sizes for the gateway namespace.
func (pc *PrometheusCollector) collectSocketBufferLimits(ch chan<- prometheus.Metric, labels []string) {
	limits, err := readSocketBufferLimits()
	if err != nil {
		err = fmt.Errorf("error collecting socket buffer limits: %w", err)
		emitInvalidSocketBufferLimitMetrics(ch, err)
		return
	}

	ch <- prometheus.MustNewConstMetric(MetricsUDPRmemMaxBytes, prometheus.GaugeValue, float64(limits.rmemMax), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsUDPWmemMaxBytes, prometheus.GaugeValue, float64(limits.wmemMax), labels...)
}

// collectInterfaceStats collects packet-level statistics of the wireguard interface
// (rx/tx packets, drops, errors) via rtnetlink, reporting 64-bit counters.
func (pc *PrometheusCollector) collectInterfaceStats(ch chan<- prometheus.Metric, labels []string) {
	link, err := netlink.LinkByName(tunnel.TunnelInterfaceName)
	if err != nil {
		err = fmt.Errorf("error getting wireguard interface %q stats: %w", tunnel.TunnelInterfaceName, err)
		emitInvalidInterfaceMetrics(ch, err)
		return
	}

	stats := link.Attrs().Statistics
	if stats == nil {
		err = fmt.Errorf("no statistics available for wireguard interface %q", tunnel.TunnelInterfaceName)
		emitInvalidInterfaceMetrics(ch, err)
		return
	}

	ch <- prometheus.MustNewConstMetric(MetricsInterfaceRxPackets, prometheus.CounterValue, float64(stats.RxPackets), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsInterfaceTxPackets, prometheus.CounterValue, float64(stats.TxPackets), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsInterfaceRxDropped, prometheus.CounterValue, float64(stats.RxDropped), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsInterfaceTxDropped, prometheus.CounterValue, float64(stats.TxDropped), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsInterfaceRxErrors, prometheus.CounterValue, float64(stats.RxErrors), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsInterfaceTxErrors, prometheus.CounterValue, float64(stats.TxErrors), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsInterfaceRxFIFODropped, prometheus.CounterValue, float64(stats.RxFifoErrors), labels...)
	ch <- prometheus.MustNewConstMetric(MetricsInterfaceTxFIFODropped, prometheus.CounterValue, float64(stats.TxFifoErrors), labels...)
}

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

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
	geneveutils "github.com/liqotech/liqo/pkg/utils/network/geneve"
)

var _ prometheus.Collector = &PrometheusCollector{}

// MetricsOptions contains the options for the PrometheusCollector.
type MetricsOptions struct {
	RemoteClusterID string
	Namespace       string
}

// PrometheusCollector collects geneve tunnel metrics from GeneveTunnel status.
type PrometheusCollector struct {
	client client.Client
	opts   *MetricsOptions
}

// NewPrometheusCollector creates a new geneve PrometheusCollector.
func NewPrometheusCollector(cl client.Client, opts *MetricsOptions) *PrometheusCollector {
	return &PrometheusCollector{client: cl, opts: opts}
}

// Describe implements prometheus.Collector.
func (pc *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- tunnel.MetricsGeneveLatency
	ch <- tunnel.MetricsGeneveIsConnected
	ch <- tunnel.MetricsGeneveReceivedBytes
	ch <- tunnel.MetricsGeneveTransmittedBytes
	tunnel.MetricsGeneveLatencyHistogram.Describe(ch)
}

// Collect implements prometheus.Collector.
func (pc *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	list := &networkingv1beta1.GeneveTunnelList{}
	if err := pc.client.List(ctx, list, client.InNamespace(pc.opts.Namespace)); err != nil {
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsGeneveLatency, fmt.Errorf("listing genevetunnels: %w", err))
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsGeneveIsConnected, fmt.Errorf("listing genevetunnels: %w", err))
		return
	}

	for i := range list.Items {
		gt := &list.Items[i]
		labels := pc.labelsFor(gt)

		connected := gt.Status.Value == networkingv1beta1.Connected
		var connectedValue float64
		if connected {
			connectedValue = 1
		}
		ch <- prometheus.MustNewConstMetric(
			tunnel.MetricsGeneveIsConnected,
			prometheus.GaugeValue,
			connectedValue,
			labels...,
		)

		if connected && gt.Status.Latency.Value != "" {
			latency, err := time.ParseDuration(gt.Status.Latency.Value)
			if err != nil {
				ch <- prometheus.NewInvalidMetric(tunnel.MetricsGeneveLatency, err)
			} else {
				latencyUs := float64(latency.Microseconds())
				ch <- prometheus.MustNewConstMetric(
					tunnel.MetricsGeneveLatency,
					prometheus.GaugeValue,
					latencyUs,
					labels...,
				)
				tunnel.MetricsGeneveLatencyHistogram.With(prometheus.Labels{
					tunnel.GeneveMetricsLabels[0]: labels[0],
					tunnel.GeneveMetricsLabels[1]: labels[1],
					tunnel.GeneveMetricsLabels[2]: labels[2],
					tunnel.GeneveMetricsLabels[3]: labels[3],
				}).Observe(latencyUs)
			}
		}

		pc.collectTraffic(ctx, gt, labels, ch)
	}

	tunnel.MetricsGeneveLatencyHistogram.Collect(ch)
}

// collectTraffic emits the received/transmitted bytes counters for the local geneve interface
// associated to the given GeneveTunnel.
func (pc *PrometheusCollector) collectTraffic(ctx context.Context, gt *networkingv1beta1.GeneveTunnel,
	labels []string, ch chan<- prometheus.Metric) {
	if gt.Spec.InternalNodeRef == nil {
		return
	}

	internalnode := &networkingv1beta1.InternalNode{}
	if err := pc.client.Get(ctx, types.NamespacedName{Name: gt.Spec.InternalNodeRef.Name}, internalnode); err != nil {
		klog.V(4).Infof("unable to get internalnode %q for geneve traffic metrics: %v", gt.Spec.InternalNodeRef.Name, err)
		return
	}

	interfaceName := internalnode.Spec.Interface.Gateway.Name
	stats, err := geneveutils.GetGeneveInterfaceStatistics(interfaceName)
	if err != nil {
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsGeneveReceivedBytes, err)
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsGeneveTransmittedBytes, err)
		return
	}
	if stats == nil {
		// The interface does not exist (yet) on this node.
		return
	}

	ch <- prometheus.MustNewConstMetric(
		tunnel.MetricsGeneveReceivedBytes,
		prometheus.CounterValue,
		float64(stats.RxBytes),
		labels...,
	)
	ch <- prometheus.MustNewConstMetric(
		tunnel.MetricsGeneveTransmittedBytes,
		prometheus.CounterValue,
		float64(stats.TxBytes),
		labels...,
	)
}

func (pc *PrometheusCollector) labelsFor(gt *networkingv1beta1.GeneveTunnel) []string {
	internalFabric := ""
	if gt.Spec.InternalFabricRef != nil {
		internalFabric = gt.Spec.InternalFabricRef.Name
	}
	internalNode := ""
	if gt.Spec.InternalNodeRef != nil {
		internalNode = gt.Spec.InternalNodeRef.Name
	}
	return []string{internalFabric, internalNode, gt.Namespace, pc.opts.RemoteClusterID}
}

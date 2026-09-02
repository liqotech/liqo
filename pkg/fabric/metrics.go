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

package fabric

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/conncheck"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
	geneveutils "github.com/liqotech/liqo/pkg/utils/network/geneve"
)

var _ prometheus.Collector = &GeneveTrafficCollector{}

// GeneveTrafficCollector exports the received/transmitted bytes counters for
// local geneve interfaces. These must be read at scrape time from the netlink
// interface statistics.
type GeneveTrafficCollector struct {
	client   client.Client
	nodeName string
}

// NewGeneveTrafficCollector creates a new GeneveTrafficCollector.
func NewGeneveTrafficCollector(cl client.Client, nodeName string) *GeneveTrafficCollector {
	return &GeneveTrafficCollector{client: cl, nodeName: nodeName}
}

// Describe implements prometheus.Collector.
func (c *GeneveTrafficCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- tunnel.MetricsGeneveReceivedBytes
	ch <- tunnel.MetricsGeneveTransmittedBytes
}

// Collect implements prometheus.Collector.
func (c *GeneveTrafficCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	list := &networkingv1beta1.GeneveTunnelList{}
	if err := c.client.List(ctx, list); err != nil {
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsGeneveReceivedBytes, fmt.Errorf("listing genevetunnels: %w", err))
		ch <- prometheus.NewInvalidMetric(tunnel.MetricsGeneveTransmittedBytes, fmt.Errorf("listing genevetunnels: %w", err))
		return
	}

	for i := range list.Items {
		gt := &list.Items[i]
		c.collectTraffic(ctx, gt, ch)
	}
}

// collectTraffic emits the received/transmitted bytes counters for the local geneve interface
// associated to the given GeneveTunnel.
func (c *GeneveTrafficCollector) collectTraffic(ctx context.Context, gt *networkingv1beta1.GeneveTunnel,
	ch chan<- prometheus.Metric) {
	if gt.Spec.InternalFabricRef == nil {
		return
	}

	internalfabric := &networkingv1beta1.InternalFabric{}
	if err := c.client.Get(ctx, types.NamespacedName{
		Name:      gt.Spec.InternalFabricRef.Name,
		Namespace: gt.Spec.InternalFabricRef.Namespace,
	}, internalfabric); err != nil {
		klog.V(4).Infof("unable to get internalfabric %q for geneve traffic metrics: %v", gt.Spec.InternalFabricRef.Name, err)
		return
	}

	interfaceName := internalfabric.Spec.Interface.Node.Name
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

	labels := []string{
		internalfabric.Name,
		gt.Spec.InternalNodeRef.Name,
		gt.Namespace,
		internalfabric.Labels[consts.RemoteClusterID],
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

// observeGeneveLatency returns a conncheck.PingObserver that records the round-trip latency
// into the geneve metrics at the point of measurement (on each PONG and on disconnect).
func observeGeneveLatency(internalfabric *networkingv1beta1.InternalFabric, gt *networkingv1beta1.GeneveTunnel) conncheck.PingObserver {
	return tunnel.ObserveLatencyMetrics(tunnel.MetricsGeneveLatency, tunnel.MetricsGeneveLatencyHistogram, tunnel.MetricsGeneveIsConnected,
		prometheus.Labels{
			tunnel.GeneveMetricsLabels[0]: internalfabric.Name,
			tunnel.GeneveMetricsLabels[1]: gt.Spec.InternalNodeRef.Name,
			tunnel.GeneveMetricsLabels[2]: gt.Namespace,
			tunnel.GeneveMetricsLabels[3]: internalfabric.Labels[consts.RemoteClusterID],
		})
}

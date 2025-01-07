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

package metrics

import (
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

var (
	// ErrorsCounter is the counter of the errors occurred during the reflection.
	ErrorsCounter *prometheus.CounterVec
	// ItemsCounter is the counter of the reflected resources.
	// A fast increase of this metric can indicate a race condition between local and remote operators.
	ItemsCounter *prometheus.CounterVec
)

// Init initializes the metrics. If no error occurs or no item is processed, the corresponding metric is not exported.
func init() {
	var MetricsLabels = []string{"namespace", "reflector_resource", "cluster_id", "node_name"}

	ErrorsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "liqo_virtual_kubelet_reflection_error_counter",
			Help: "The counter of the transient errors.",
		},
		MetricsLabels,
	)

	ItemsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "liqo_virtual_kubelet_reflection_item_counter",
			Help: "The counter of the reflected resources. A fast increase of this metric can indicate a race condition between local and remote operators.",
		},
		MetricsLabels,
	)
}

// SetupMetricHandler sets up the metric handler.
func SetupMetricHandler(metricsAddress string) {
	// Register the metrics to the prometheus registry.
	prometheus.MustRegister(ErrorsCounter)
	// Register the metrics to the prometheus registry.
	prometheus.MustRegister(ItemsCounter)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		klog.Infof("Starting the virtual kubelet Metric Handler listening on %q", metricsAddress)

		server := &http.Server{
			Addr:              metricsAddress,
			ReadHeaderTimeout: 10 * time.Second,
		}

		// Key and certificate paths are not specified, since already configured as part of the TLSConfig.
		if err := server.ListenAndServe(); err != nil {
			klog.Errorf("Failed to start the Metric Handler: %v", err)
			os.Exit(1)
		}
	}()
}

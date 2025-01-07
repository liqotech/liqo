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

package remotemetrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Context("Aggregator", func() {

	var aggregator Aggregator
	var metrics Metrics
	var aggregatedMetrics Metrics

	BeforeEach(func() {
		aggregator = &nodeMetricAggregator{}
		metrics = []*Metric{
			{
				promType: "# TYPE node_cpu_usage_seconds_total",
				promHelp: "# HELP node_cpu_usage_seconds_total",
				values: []string{
					"node_cpu_usage_seconds_total 1 1000000000",
					"node_cpu_usage_seconds_total 2 2000000000",
				},
			},
			{
				promType: "# TYPE node_memory_working_set_bytes",
				promHelp: "# HELP node_memory_working_set_bytes",
				values: []string{
					"node_memory_working_set_bytes 1 1000000000",
					"node_memory_working_set_bytes 2 2000000000",
				},
			},
			{
				promType: "# TYPE other_metric_1",
				promHelp: "# HELP other_metric_1",
				values: []string{
					"other_metric_1 1 1000000000",
					"other_metric_1 2 2000000000",
				},
			},
		}
	})

	JustBeforeEach(func() {
		aggregatedMetrics = aggregator.Aggregate(metrics)
	})

	It("should aggregate node metrics", func() {
		Expect(aggregatedMetrics[0]).To(PointTo(Equal(Metric{
			promType: "# TYPE node_cpu_usage_seconds_total",
			promHelp: "# HELP node_cpu_usage_seconds_total",
			values: []string{
				"node_cpu_usage_seconds_total 3.000000 2000000000",
			},
		})))

		Expect(aggregatedMetrics[1]).To(PointTo(Equal(Metric{
			promType: "# TYPE node_memory_working_set_bytes",
			promHelp: "# HELP node_memory_working_set_bytes",
			values: []string{
				"node_memory_working_set_bytes 3.000000 2000000000",
			},
		})))
	})

	It("should not aggregate other metrics", func() {
		Expect(aggregatedMetrics[2]).To(PointTo(Equal(Metric{
			promType: "# TYPE other_metric_1",
			promHelp: "# HELP other_metric_1",
			values: []string{
				"other_metric_1 1 1000000000",
				"other_metric_1 2 2000000000",
			},
		})))
	})

})

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
	"fmt"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

type nodeMetricAggregator struct {
}

// Aggregate aggregates the node metrics.
func (a *nodeMetricAggregator) Aggregate(metrics Metrics) Metrics {
	for _, m := range metrics {
		if !isNodeMetric(m) {
			continue
		}

		var name string
		var value float64
		var timestamp int64
		var totValue float64
		var lastTimestamp int64

		for _, v := range m.values {
			name, value, timestamp = parseValue(v)
			if name == "" {
				continue
			}

			if timestamp > lastTimestamp {
				lastTimestamp = timestamp
			}

			totValue += value
		}

		// TODO: should we scale them by the sharing percentage?
		m.values = []string{fmt.Sprintf("%s %f %d", name, totValue, lastTimestamp)}
	}
	return metrics
}

func isNodeMetric(m *Metric) bool {
	for _, name := range nodeMetricsNames {
		if strings.Contains(m.promHelp, name) {
			return true
		}
	}
	return false
}

func parseValue(line string) (name string, value float64, timestamp int64) {
	strs := strings.Split(line, " ")
	if len(strs) != 3 {
		klog.Warning("unexpected line format: ", line)
		return "", 0, 0
	}

	name = strs[0]
	value, err := strconv.ParseFloat(strs[1], 64)
	if err != nil {
		klog.Warning("unexpected value format: ", line)
		return "", 0, 0
	}

	timestamp, err = strconv.ParseInt(strs[2], 10, 64)
	if err != nil {
		klog.Warning("unexpected timestamp format: ", line)
		return "", 0, 0
	}

	return name, value, timestamp
}

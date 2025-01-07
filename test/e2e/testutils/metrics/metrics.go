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
	"fmt"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ParseMetrics parses a Prometheus metrics file and returns a map of metric families.
func ParseMetrics(body string) (map[string]*dto.MetricFamily, error) {
	parser := expfmt.TextParser{}
	return parser.TextToMetricFamilies(strings.NewReader(body))
}

// RetrieveCounter retrieves a metric of type counter from a metric family, given the metric name and a set of key-value selectors.
func RetrieveCounter(metricFamilies map[string]*dto.MetricFamily, metricName string, keyValueSelectors map[string]string) (float64, error) {
	// Find the metric family
	if metricFamilies == nil {
		return 0, fmt.Errorf("metric families not found")
	}

	mf, ok := metricFamilies[metricName]
	if !ok {
		return 0, errors.NewNotFound(schema.GroupResource{}, metricName)
	}

	// Find the specific metric
	for _, metric := range mf.Metric {
		entry := make(map[string]string)
		for _, pair := range metric.Label {
			entry[pair.GetName()] = pair.GetValue()
		}

		match := true
		for key, value := range keyValueSelectors {
			if entry[key] != value {
				match = false
				break
			}
		}

		if match {
			counter := metric.GetCounter()
			if counter == nil {
				return 0, fmt.Errorf("metric %s is not a counter", metricName)
			}
			return counter.GetValue(), nil
		}
	}

	return 0, errors.NewNotFound(schema.GroupResource{}, metricName)
}

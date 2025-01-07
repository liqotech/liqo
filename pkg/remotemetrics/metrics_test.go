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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Context("Metrics", func() {

	var getMetric = func(name string, starting, nFields int) *Metric {
		values := make([]string, nFields)
		for i := 0; i < nFields; i++ {
			v := starting + i + 1
			values[i] = fmt.Sprintf("%v %d %d000000000", name, v, v)
		}

		return &Metric{
			promType: fmt.Sprintf("# TYPE %s", name),
			promHelp: fmt.Sprintf("# HELP %s", name),
			values:   values,
		}
	}

	type mergeMetricsTestcase struct {
		src            Metrics
		dst            Metrics
		expectedOutput types.GomegaMatcher
	}

	DescribeTable("mergeMetrics table", func(c mergeMetricsTestcase) {
		mergeMetrics(&c.dst, &c.src)
		Expect(c.dst).To(c.expectedOutput)
	}, Entry("should merge metrics to an empty value", mergeMetricsTestcase{
		dst: Metrics{},
		src: Metrics{
			getMetric("metric_name_1", 0, 1),
		},
		expectedOutput: Equal(Metrics{
			getMetric("metric_name_1", 0, 1),
		}),
	}), Entry("should merge metrics to a non-empty value", mergeMetricsTestcase{
		dst: Metrics{
			getMetric("metric_name_1", 0, 1),
		},
		src: Metrics{
			getMetric("metric_name_1", 1, 1),
		},
		expectedOutput: Equal(Metrics{
			getMetric("metric_name_1", 0, 2),
		}),
	}), Entry("should merge metrics", mergeMetricsTestcase{
		dst: Metrics{
			getMetric("metric_name_1", 0, 1),
			getMetric("metric_name_2", 0, 1),
		},
		src: Metrics{
			getMetric("metric_name_1", 1, 1),
			getMetric("metric_name_3", 0, 1),
		},
		expectedOutput: Equal(Metrics{
			getMetric("metric_name_1", 0, 2),
			getMetric("metric_name_2", 0, 1),
			getMetric("metric_name_3", 0, 1),
		}),
	}))

})

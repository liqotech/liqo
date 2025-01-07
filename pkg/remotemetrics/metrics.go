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
	"io"
	"sort"

	"k8s.io/klog/v2"
)

// Write writes the metrics to the given writer.
func (m Metrics) Write(w io.Writer) {
	handle := func(_ int, err error) {
		if err != nil {
			klog.Errorf("failed to write metrics: %v", err)
		}
	}

	for _, v := range m {
		if len(v.values) == 0 {
			continue
		}

		handle(w.Write([]byte(v.promHelp)))
		handle(w.Write([]byte("\n")))

		handle(w.Write([]byte(v.promType)))
		handle(w.Write([]byte("\n")))

		sort.Strings(v.values)
		for _, vv := range v.values {
			handle(w.Write([]byte(vv)))
			handle(w.Write([]byte("\n")))
		}
	}
}

func mergeMetrics(dst, src *Metrics) {
	// TODO: they are sorted, we can improve it
	for _, m := range *src {
		found := false
		for _, fm := range *dst {
			if fm.promHelp == m.promHelp {
				fm.values = append(fm.values, m.values...)
				found = true
				break
			}
		}
		if !found {
			*dst = append(*dst, m)
		}
	}
}

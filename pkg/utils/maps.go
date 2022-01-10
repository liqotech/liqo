// Copyright 2019-2022 The Liqo Authors
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

package utils

// MergeMaps merges two maps.
func MergeMaps(m1, m2 map[string]string) map[string]string {
	if m1 == nil {
		return m2
	}
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

// SubMaps removes elements of m2 from m1.
func SubMaps(m1, m2 map[string]string) map[string]string {
	for k := range m2 {
		delete(m1, k)
	}
	return m1
}

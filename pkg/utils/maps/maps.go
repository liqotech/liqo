// Copyright 2019-2023 The Liqo Authors
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

package maps

// Merge merges two maps.
func Merge[K comparable, V any](m1, m2 map[K]V) map[K]V {
	if m1 == nil {
		return m2
	}
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

// Sub removes elements of m2 from m1.
func Sub[K comparable, V any](m1, m2 map[K]V) map[K]V {
	for k := range m2 {
		delete(m1, k)
	}
	return m1
}

// FilterType is a function type used to filter a map.
type FilterType[K comparable] func(key K) bool

// Filter filters a map, returning a duplicate which contains only the elements matching the filter function.
func Filter[K comparable, V any](m map[K]V, filter FilterType[K]) map[K]V {
	filtered := make(map[K]V)

	for k, v := range m {
		if filter(k) {
			filtered[k] = v
		}
	}

	return filtered
}

// FilterWhitelist returns a filter function returning true if the key is in the whitelist.
func FilterWhitelist[K comparable](whitelist ...K) FilterType[K] {
	return func(check K) bool {
		for _, el := range whitelist {
			if el == check {
				return true
			}
		}
		return false
	}
}

// FilterBlacklist returns a filter function returning true if the key is not the blacklist.
func FilterBlacklist[K comparable](blacklist ...K) FilterType[K] {
	return func(check K) bool {
		return !FilterWhitelist(blacklist...)(check)
	}
}

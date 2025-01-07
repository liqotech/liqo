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

package slice

import "slices"

// Remove returns a newly created slice that contains all items from slice that
// are not equal to s.
func Remove[K comparable](slice []K, s K) []K {
	return slices.DeleteFunc(slice, func(t K) bool {
		return t == s
	})
}

// LongestString returns the longest string in a slice of strings.
func LongestString(slice []string) string {
	var longest string
	for _, s := range slice {
		if len(s) > len(longest) {
			longest = s
		}
	}
	return longest
}

// Merge merges two slices removing duplicates.
func Merge[K comparable](s1, s2 []K) []K {
	if s1 == nil {
		return s2
	}
	if s2 == nil {
		return s1
	}
	// merge and remove duplicates
	for _, item := range s2 {
		if !slices.Contains(s1, item) {
			s1 = append(s1, item)
		}
	}
	return s1
}

// Sub removes elements of s2 from s1.
func Sub[K comparable](s1, s2 []K) []K {
	for _, item := range s2 {
		s1 = Remove(s1, item)
	}
	return s1
}

// ToPointerSlice returns a new slice of pointers to the elements of the input slice.
func ToPointerSlice[T any](s []T) []*T {
	p := make([]*T, len(s))
	for i := range s {
		p[i] = &s[i]
	}
	return p
}

// Map applies a function to each element of a slice and returns a new slice with the results.
func Map[K, V any](s []K, f func(K) V) []V {
	mapped := make([]V, len(s))
	for i, item := range s {
		mapped[i] = f(item)
	}
	return mapped
}

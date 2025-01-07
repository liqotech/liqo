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
	"strings"
)

type matchAll struct {
	matchers []Matcher
}

// MatchAll returns a matcher that matches all the given matchers.
func MatchAll() MatcherCollection {
	return &matchAll{}
}

// Add adds the given matcher to the collection.
func (m *matchAll) Add(matcher Matcher) MatcherCollection {
	m.matchers = append(m.matchers, matcher)
	return m
}

// Match returns true if the line matches the matcher.
func (m *matchAll) Match(line string) bool {
	for _, matcher := range m.matchers {
		if !matcher.Match(line) {
			return false
		}
	}
	return true
}

type matchAny struct {
	matchers []Matcher
}

// MatchAny returns a matcher that matches any of the given matchers.
func MatchAny() MatcherCollection {
	return &matchAny{}
}

// Add adds the given matcher to the collection.
func (m *matchAny) Add(matcher Matcher) MatcherCollection {
	m.matchers = append(m.matchers, matcher)
	return m
}

// Match returns true if the line matches the matcher.
func (m *matchAny) Match(line string) bool {
	for _, matcher := range m.matchers {
		if matcher.Match(line) {
			return true
		}
	}
	return false
}

// MatchNamespaces returns a matcher that matches the given namespaces.
func MatchNamespaces(namespaces ...MappedNamespace) Matcher {
	mAny := MatchAny()
	for _, namespace := range namespaces {
		mAny.Add(&matchNamespace{namespace: namespace})
	}
	return mAny
}

// MatchPods returns a matcher that matches the given pods.
func MatchPods(pods ...string) Matcher {
	mAny := MatchAny()
	for _, pod := range pods {
		mAny.Add(&matchPod{pod: pod})
	}
	return mAny
}

type matchNamespace struct {
	namespace MappedNamespace
}

// Match returns true if the line matches the matcher.
func (m *matchNamespace) Match(line string) bool {
	return strings.Contains(line, fmt.Sprintf("namespace=%q", m.namespace.Namespace))
}

type matchPod struct {
	pod string
}

// Match returns true if the line matches the matcher.
func (m *matchPod) Match(line string) bool {
	return strings.Contains(line, fmt.Sprintf("pod=%q", m.pod))
}

// MatchNodeMetrics returns a matcher that matches the node metrics.
func MatchNodeMetrics() MatcherCollection {
	mAny := MatchAny()
	for _, metric := range nodeMetricsNames {
		mAny.Add(&matchName{name: metric})
	}
	return mAny
}

type matchName struct {
	name string
}

// Match returns true if the line matches the matcher.
func (m *matchName) Match(line string) bool {
	return strings.HasPrefix(line, m.name) && (len(line) == len(m.name) || line[len(m.name)] == '{' || line[len(m.name)] == ' ')
}

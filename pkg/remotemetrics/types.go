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

import "context"

// Scraper is the interface for a remote metrics scraper.
type Scraper interface {
	Scrape(ctx context.Context, path, clusterID string) (Metrics, error)
}

// MappedNamespace contains both the original and the mapped namespace names.
type MappedNamespace struct {
	Namespace    string
	OriginalName string
}

// ResourceGetter is the interface for a local resource getter.
type ResourceGetter interface {
	GetNamespaces(ctx context.Context, clusterID string) []MappedNamespace
	GetPodNames(ctx context.Context, clusterID, node string) []string
	GetNodeNames(ctx context.Context) []string
}

// Aggregator is the interface for a metrics aggregator.
type Aggregator interface {
	Aggregate(metric Metrics) Metrics
}

// MatcherCollection is the interface for a matcher collection.
type MatcherCollection interface {
	Matcher
	Add(matcher Matcher) MatcherCollection
}

// Matcher is the interface for a matcher.
type Matcher interface {
	Match(line string) bool
}

// Mapper is the interface for a mapper.
type Mapper interface {
	Map(line string) string
}

// Metrics is the alias for a list of metrics.
type Metrics []*Metric

// Metric contains the scraped prometheus metrics.
type Metric struct {
	promHelp string
	promType string
	values   []string
}

var (
	nodeMetricsNames = []string{
		"node_cpu_usage_seconds_total",
		"node_memory_working_set_bytes",
	}
)

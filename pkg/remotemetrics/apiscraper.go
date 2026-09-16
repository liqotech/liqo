// Copyright 2019-2026 The Liqo Authors
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
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// maxConcurrentScrapes bounds the number of nodes scraped concurrently for a single metrics request.
	// Without a limit, each request issues one API server proxy request per node at the same time, causing
	// burst of requests that exhaust the client-side rate limiter in clusters with many nodes.
	maxConcurrentScrapes = 32
	// nodeScrapeTimeout bounds the time spent scraping a single node. It prevents unreachable kubelets
	// (e.g., which dial times out after ~30s) from stalling the whole metrics request.
	nodeScrapeTimeout = 10 * time.Second
)

type rawGetter interface {
	get(ctx context.Context, nodeName, path string) ([]byte, error)
}

type apiServiceScraper struct {
	resourceManager ResourceGetter
	rawGetter       rawGetter
}

// NewAPIServiceScraper creates a new scraper that scrapes metrics from the API server.
func NewAPIServiceScraper(restClient rest.Interface, cl client.Client) Scraper {
	return &apiServiceScraper{
		resourceManager: NewResourceGetter(cl),
		rawGetter: &rawGetterImpl{
			restClient: restClient,
		},
	}
}

// Scrape scrapes metrics from the API server for the given (relative) path and clusterID.
// When nodeSelector is not nil nor empty, only the matching nodes are scraped, otherwise all of
// them are scraped and the metrics aggregated.
func (s *apiServiceScraper) Scrape(ctx context.Context, path, clusterID string, nodeSelector labels.Selector) (Metrics, error) {
	nodes := s.resourceManager.GetNodes(ctx, nodeSelector)
	if len(nodes) == 0 {
		klog.Warningf("No nodes matching selector %q found while scraping metrics for cluster id %s", nodeSelector, clusterID)
		return Metrics{}, nil
	}

	metricsChan := make(chan Metrics, len(nodes))
	defer close(metricsChan)

	namespaces := s.resourceManager.GetNamespaces(ctx, clusterID)
	// List the pods of the given cluster once, grouped by node, to avoid performing
	// one list operation for each node being scraped.
	podsByNode := s.resourceManager.GetPodsPerNode(ctx, clusterID)

	nodeMetricsMatcher := MatchNodeMetrics()
	metricsMapper := NewNamespaceMapper(namespaces...)

	var failedNodes atomic.Int32

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(maxConcurrentScrapes)
	for i := range nodes {
		node := nodes[i]
		// run each scraper in a separate goroutine
		errGroup.Go(func() error {
			podMetricsMatcher := MatchAll().
				Add(MatchNamespaces(namespaces...)).
				Add(MatchPods(podsByNode[node]...))

			metrics, err := s.getMetrics(ctx, node, path,
				MatchAny().Add(nodeMetricsMatcher).Add(podMetricsMatcher), metricsMapper)
			if err != nil {
				// A single unreachable or broken kubelet must not fail the whole metrics request,
				// discarding the data successfully retrieved from all the other nodes: log the
				// failure and skip the node. The error is propagated only if all nodes failed.
				if ctx.Err() == nil {
					klog.Warningf("Skipping metrics for node %q, which failed to be scraped: %s", node, err)
				}
				failedNodes.Add(1)
				metrics = Metrics{}
			}

			metricsChan <- metrics
			return nil
		})
	}

	mergeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	fullMetrics := Metrics{}
	wg := sync.WaitGroup{}
	wg.Add(1)

	// merge metrics from all nodes or until context is canceled
	go func() {
		defer wg.Done()
		for range nodes {
			select {
			case <-mergeCtx.Done():
				return
			case m := <-metricsChan:
				mergeMetrics(&fullMetrics, &m)
			}
		}
	}()

	// node scrapers never fail individually (failures are logged and the node is skipped),
	// hence the only possible error is a canceled context.
	if err := errGroup.Wait(); err != nil {
		return Metrics{}, err
	}

	aggregator := &nodeMetricAggregator{}

	// wait for the merge to finish
	wg.Wait()

	// Fail the request only if no node provided metrics at all (e.g., API server unreachable,
	// misconfigured RBAC): serving an empty response would silently mask the issue.
	if len(nodes) > 0 && int(failedNodes.Load()) == len(nodes) {
		return Metrics{}, fmt.Errorf("failed to scrape metrics from all %d nodes", len(nodes))
	}

	return aggregator.Aggregate(fullMetrics), nil
}

// getMetrics scrapes metrics from the API server for the given (relative) path and node,
// then filters the lines matcher by the matcher, and maps them (i.e. translates the namespace name with the original one).
func (s *apiServiceScraper) getMetrics(ctx context.Context,
	nodeName, path string, matcher Matcher, mapper Mapper) (Metrics, error) {
	data, err := s.rawGetter.get(ctx, nodeName, path)
	if err != nil {
		return Metrics{}, err
	}

	var lastMetric *Metric
	m := Metrics{}

	r := bufio.NewReader(bytes.NewReader(data))
	for {
		lineBytes, _, err := r.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return Metrics{}, err
		}

		line := string(lineBytes)

		switch {
		case strings.HasPrefix(line, "# HELP"):
			if lastMetric != nil && len(lastMetric.values) > 0 {
				m = append(m, lastMetric)
			}

			lastMetric = &Metric{
				promHelp: line,
			}
		case lastMetric == nil:
		case strings.HasPrefix(line, "# TYPE"):
			lastMetric.promType = line
		case matcher.Match(line):
			klog.V(4).Infof("Matched metric: %s", line)
			lastMetric.values = append(lastMetric.values, mapper.Map(line))
		default:
			klog.V(5).Infof("Ignored metric: %s", line)
		}
	}

	if lastMetric != nil && len(lastMetric.values) > 0 {
		m = append(m, lastMetric)
	}

	return m, nil
}

type rawGetterImpl struct {
	restClient rest.Interface
}

func (rg *rawGetterImpl) get(ctx context.Context, nodeName, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, nodeScrapeTimeout)
	defer cancel()

	res := rg.restClient.Get().RequestURI(fmt.Sprintf("/api/v1/nodes/%s/proxy/%s", nodeName, path)).Do(ctx)
	err := res.Error()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return res.Raw()
}

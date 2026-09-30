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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/sync/singleflight"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	group        = "metrics.liqo.io"
	version      = "v1beta1"
	groupVersion = group + "/" + version
	basePath     = "/apis/" + groupVersion
)

var (
	availablePaths = []string{
		"metrics",
		"metrics/cadvisor",
		"metrics/resource",
		"metrics/probes",
	}
)

const (
	// defaultScrapeCacheTTL is the time-to-live of cached scrape responses. Multiple consumers
	// (virtual kubelets forwarding metrics-server and Prometheus requests) typically request the
	// same (cluster, path) combination at similar times; caching the response for a short period
	// avoids re-scraping all the nodes for each of them, which is the dominant CPU cost.
	defaultScrapeCacheTTL = 15 * time.Second
	// defaultScrapeErrorCacheTTL is the time-to-live of failed scrape responses. Errors are cached
	// briefly (much shorter than successful ones), so that a persistently failing scraping (e.g., all
	// the nodes unreachable) is not retried at each consumer request, triggering a new full sweep
	// every time, while still allowing a prompt recovery once the issue is solved.
	defaultScrapeErrorCacheTTL = 5 * time.Second
	// scrapeDetachTimeout bounds the time allotted to a deduplicated scrape once detached from
	// the caller's context (see metricHTTP).
	scrapeDetachTimeout = 60 * time.Second
)

// cacheEntry contains a cached scrape response and its expiration time: either data (successful
// scrape) or err (failed one) is set.
type cacheEntry struct {
	data      []byte
	err       error
	expiresAt time.Time
}

type metricHandler struct {
	*httprouter.Router
	scraper Scraper

	cacheTTL      time.Duration
	errorCacheTTL time.Duration
	cacheMu       sync.Mutex
	cache         map[string]cacheEntry
	inflight      singleflight.Group
}

// GetHTTPHandler returns a handler for the metrics API.
func GetHTTPHandler(restClient rest.Interface, cl client.Client) (http.Handler, error) {
	router := &metricHandler{
		Router:        httprouter.New(),
		scraper:       NewAPIServiceScraper(restClient, cl),
		cacheTTL:      defaultScrapeCacheTTL,
		errorCacheTTL: defaultScrapeErrorCacheTTL,
		cache:         map[string]cacheEntry{},
	}

	// Return empty api resource list.
	// K8s expects to be able to retrieve a resource list for each aggregated
	// app in order to discover what resources it provides.
	router.GET("/", health)
	// K8s needs the ability to query info about a specific API group
	router.GET("/apis/"+group, apiGroupInfo)
	// K8s needs the ability to query the list of API groups this endpoint supports
	router.GET("/apis", apiGroupList)

	router.GET(basePath, health)
	router.GET(fmt.Sprintf("%s/scrape/:cluster-id/:path", basePath), router.metricHTTP)
	router.GET(fmt.Sprintf("%s/scrape/:cluster-id/:path/:subpath", basePath), router.metricHTTP)

	// kube-apiserver's OpenAPI aggregation controller fetches these endpoints from
	// every registered APIService. metric-agent does not expose CRUD resources, so
	// we serve minimal stub documents to silence the aggregator's retry loop.
	router.GET("/openapi/v2", openAPIDoc(map[string]interface{}{
		"swagger": "2.0",
		"info": map[string]string{
			"title":   "Liqo Metric Agent",
			"version": version,
		},
	}))
	router.GET("/openapi/v3", openAPIDoc(map[string]interface{}{}))

	return router, nil
}

func health(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	list := &metav1.APIResourceList{}

	list.Kind = "APIResourceList"
	list.GroupVersion = groupVersion
	list.APIVersion = "v1"
	list.APIResources = []metav1.APIResource{
		{
			Name:       "scrape/metrics",
			Namespaced: false,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(list); err != nil {
		klog.Errorf("failed to write response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func getAPIGroup() *metav1.APIGroup {
	return &metav1.APIGroup{
		TypeMeta: metav1.TypeMeta{
			Kind: "APIGroup",
		},
		Name: group,
		PreferredVersion: metav1.GroupVersionForDiscovery{
			GroupVersion: groupVersion,
			Version:      version,
		},
		Versions: []metav1.GroupVersionForDiscovery{
			{
				GroupVersion: groupVersion,
				Version:      version,
			},
		},
		ServerAddressByClientCIDRs: []metav1.ServerAddressByClientCIDR{
			{
				ClientCIDR:    "0.0.0.0/0",
				ServerAddress: "",
			},
		},
	}
}

func apiGroupInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(getAPIGroup()); err != nil {
		klog.Errorf("failed to write response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func apiGroupList(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	list := &metav1.APIGroupList{}
	list.Kind = "APIGroupList"
	list.Groups = append(list.Groups, *getAPIGroup())
	if err := json.NewEncoder(w).Encode(list); err != nil {
		klog.Errorf("failed to write response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func openAPIDoc(doc map[string]interface{}) httprouter.Handle {
	doc["paths"] = map[string]interface{}{}
	return func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(doc); err != nil {
			klog.Errorf("failed to write response: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func (handler *metricHandler) metricHTTP(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := ps.ByName("cluster-id")
	path := ps.ByName("path")
	subpath := ps.ByName("subpath")
	if subpath != "" {
		path = path + "/" + subpath
	}

	if !handler.isValidPath(path) {
		klog.Errorf("invalid path: %s", path)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// An optional selector restricts the scraping to the matching nodes (e.g., the ones targeted
	// by the requesting virtual kubelet through its offloading patch). When absent, all nodes
	// are scraped and aggregated.
	var nodeSelector labels.Selector
	selectorStr := ""
	if raw := req.URL.Query().Get("nodeSelector"); raw != "" {
		parsed, err := labels.Parse(raw)
		if err != nil {
			klog.Errorf("invalid node selector %q: %s", raw, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		nodeSelector = parsed
		selectorStr = nodeSelector.String()
	}

	key := clusterID + "/" + path + "?" + selectorStr

	entry, found := handler.cacheGet(key)

	var data []byte
	if found {
		if entry.err != nil {
			writeScrapeError(w, entry.err)
			return
		}
		klog.V(4).Infof("Serving cached metrics for %q", key)
		data = entry.data
	} else {
		// Deduplicate concurrent scrapes of the same (cluster, path): only one of them
		// actually performs the scraping, while the others wait for the shared result.
		// Failures are cached briefly (errorCacheTTL), so that a persistently broken
		// scraping is not retried at the pace of the consumers' requests.
		res, err, _ := handler.inflight.Do(key, func() (interface{}, error) {
			// Detach the scraping from the caller's context: a client disconnecting early
			// (e.g., a short scrape timeout) must not abort the shared scraping the other
			// waiters depend on, nor prevent the result from being cached.
			ctx, cancel := context.WithTimeout(context.Background(), scrapeDetachTimeout)
			defer cancel()

			metrics, err := handler.scraper.Scrape(ctx, path, clusterID, nodeSelector)
			if err != nil {
				handler.cacheSet(key, cacheEntry{err: err, expiresAt: time.Now().Add(handler.errorCacheTTL)})
				return nil, err
			}

			var buf bytes.Buffer
			metrics.Write(&buf)

			handler.cacheSet(key, cacheEntry{data: buf.Bytes(), expiresAt: time.Now().Add(handler.cacheTTL)})

			return buf.Bytes(), nil
		})
		if err != nil {
			writeScrapeError(w, err)
			return
		}
		data = res.([]byte)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	// #nosec G705 -- the endpoint serves text/plain API responses, not HTML; no XSS surface.
	if _, err := w.Write(data); err != nil {
		klog.Errorf("failed to write metrics: %s", err)
	}
}

func (handler *metricHandler) isValidPath(path string) bool {
	for _, p := range availablePaths {
		if path == p {
			return true
		}
	}
	return false
}

// writeScrapeError logs the given scraping error and serves it to the client as an Internal Server Error.
func writeScrapeError(w http.ResponseWriter, err error) {
	klog.Errorf("failed to scrape metrics: %s", err)
	w.WriteHeader(http.StatusInternalServerError)
	// #nosec G705 -- the endpoint serves text/plain API responses, not HTML; no XSS surface.
	if _, werr := w.Write([]byte(err.Error())); werr != nil {
		klog.Errorf("failed to write error: %s", werr)
	}
}

// cacheGet returns the cache entry for the given key, if present and not expired. Expired
// entries are evicted, so that stale responses do not linger in the cache.
func (handler *metricHandler) cacheGet(key string) (cacheEntry, bool) {
	handler.cacheMu.Lock()
	defer handler.cacheMu.Unlock()

	entry, found := handler.cache[key]
	if !found {
		return cacheEntry{}, false
	}
	if !time.Now().Before(entry.expiresAt) {
		delete(handler.cache, key)
		return cacheEntry{}, false
	}
	return entry, true
}

// cacheSet stores the given entry in the cache, also evicting the entries expired in the
// meantime, to bound the memory retained by keys not requested anymore.
func (handler *metricHandler) cacheSet(key string, entry cacheEntry) {
	handler.cacheMu.Lock()
	defer handler.cacheMu.Unlock()

	now := time.Now()
	for k, e := range handler.cache {
		if !now.Before(e.expiresAt) {
			delete(handler.cache, k)
		}
	}
	handler.cache[key] = entry
}

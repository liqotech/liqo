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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"github.com/julienschmidt/httprouter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
)

// countingScraper is a fake Scraper counting the number of times it has been invoked.
type countingScraper struct {
	calls    atomic.Int32
	metrics  Metrics
	err      error
	selector labels.Selector
}

func (c *countingScraper) Scrape(_ context.Context, _, _ string, selector labels.Selector) (Metrics, error) {
	c.calls.Add(1)
	c.selector = selector
	return c.metrics, c.err
}

var _ = Context("HTTP handler", func() {

	var handler *metricHandler
	var scraper *countingScraper

	scrape := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/scrape/cluster1/metrics", basePath), http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	BeforeEach(func() {
		scraper = &countingScraper{
			metrics: Metrics{{
				promHelp: "# HELP metric1",
				promType: "# TYPE metric1",
				values:   []string{"metric1{namespace=\"namespace1\",pod=\"pod1\"} 1 1000000000"},
			}},
		}
		handler = &metricHandler{
			Router:        httprouter.New(),
			scraper:       scraper,
			cacheTTL:      defaultScrapeCacheTTL,
			errorCacheTTL: defaultScrapeErrorCacheTTL,
			cache:         map[string]cacheEntry{},
		}
		handler.GET(fmt.Sprintf("%s/scrape/:cluster-id/:path", basePath), handler.metricHTTP)
	})

	It("should scrape the metrics and cache the response", func() {
		rec := scrape()
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Body.String()).To(ContainSubstring("metric1{namespace=\"namespace1\",pod=\"pod1\"} 1 1000000000"))
		Expect(scraper.calls.Load()).To(Equal(int32(1)))

		By("serving the same response from the cache, without scraping again")
		rec = scrape()
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Body.String()).To(ContainSubstring("metric1{namespace=\"namespace1\",pod=\"pod1\"} 1 1000000000"))
		Expect(scraper.calls.Load()).To(Equal(int32(1)))

		By("scraping again once the cached response is expired")
		handler.cacheMu.Lock()
		for key, entry := range handler.cache {
			handler.cache[key] = cacheEntry{data: entry.data, expiresAt: time.Now().Add(-time.Hour)}
		}
		handler.cacheMu.Unlock()

		rec = scrape()
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(scraper.calls.Load()).To(Equal(int32(2)))
	})

	It("should briefly cache failed scrapes", func() {
		scraper.err = fmt.Errorf("boom")

		rec := scrape()
		Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		Expect(scraper.calls.Load()).To(Equal(int32(1)))

		By("serving the cached error, without scraping again")
		rec = scrape()
		Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		Expect(rec.Body.String()).To(ContainSubstring("boom"))
		Expect(scraper.calls.Load()).To(Equal(int32(1)))

		By("retrying the scrape once the cached error is expired")
		handler.cacheMu.Lock()
		for key, entry := range handler.cache {
			handler.cache[key] = cacheEntry{data: entry.data, err: entry.err, expiresAt: time.Now().Add(-time.Hour)}
		}
		handler.cacheMu.Unlock()

		scraper.err = nil
		rec = scrape()
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(scraper.calls.Load()).To(Equal(int32(2)))
	})

})

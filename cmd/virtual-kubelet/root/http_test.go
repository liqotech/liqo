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

package root

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
)

var _ = Describe("Metrics proxy routes", func() {
	const clusterID = "local-cluster-id"

	var (
		ctx        context.Context
		mux        *http.ServeMux
		calls      atomic.Int64
		failScrape atomic.Bool
	)

	BeforeEach(func() {
		ctx = context.Background()
		calls.Store(0)
		failScrape.Store(false)

		cl := &fake.RESTClient{
			NegotiatedSerializer: clientscheme.Codecs.WithoutConversion(),
			GroupVersion:         schema.GroupVersion{Group: "metrics.liqo.io", Version: "v1beta1"},
			Client: fake.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				if failScrape.Load() {
					return nil, errors.New("scrape failure")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
					Body:       io.NopCloser(strings.NewReader("metrics-payload")),
				}, nil
			}),
		}

		mux = http.NewServeMux()
		attachMetricsRoutes(ctx, mux, cl, clusterID, "")
	})

	doRequest := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		return rec
	}

	It("should proxy the remote metrics", func() {
		rec := doRequest("/metrics/resource")
		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Body.String()).To(Equal("metrics-payload"))
	})

	It("should serve subsequent requests for the same path from the cache", func() {
		doRequest("/metrics/resource")
		doRequest("/metrics/resource")
		Expect(calls.Load()).To(BeNumerically("==", 1))
	})

	It("should not alias requests differing in path", func() {
		doRequest("/metrics/resource")
		doRequest("/metrics/cadvisor")
		Expect(calls.Load()).To(BeNumerically("==", 2))
	})

	It("should cache failures briefly", func() {
		failScrape.Store(true)

		Expect(doRequest("/metrics/resource").Code).To(Equal(http.StatusInternalServerError))
		Expect(doRequest("/metrics/resource").Code).To(Equal(http.StatusInternalServerError))
		Expect(calls.Load()).To(BeNumerically("==", 1))
	})
})

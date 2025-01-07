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
	"bytes"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Context("ApiScraper", func() {

	var scraper Scraper
	var metrics Metrics
	var err error
	var node1Metrics, node2Metrics Metrics

	BeforeEach(func() {
		node1Metrics = Metrics{
			{
				promType: "# TYPE metric1",
				promHelp: "# HELP metric1",
				values: []string{
					"metric1{namespace=\"namespace1\",pod=\"pod1\"} 1 1000000000",
					"metric1{namespace=\"namespace1\",pod=\"pod2\"} 2 2000000000",
					"metric1{namespace=\"namespace2\",pod=\"pod3\"} 3 3000000000",
				},
			},
			{
				promType: "# TYPE metric2",
				promHelp: "# HELP metric2",
				values: []string{
					"metric2{namespace=\"namespace1\",pod=\"pod1\"} 1 1000000000",
					"metric2{namespace=\"namespace2\",pod=\"pod3\"} 2 2000000000",
				},
			},
		}
		node1Data := new(bytes.Buffer)
		node1Metrics.Write(node1Data)

		node2Metrics = Metrics{
			{
				promType: "# TYPE metric1",
				promHelp: "# HELP metric1",
				values: []string{
					"metric1{namespace=\"namespace1\",pod=\"pod5\"} 4 1000000000",
				},
			},
		}
		node2Data := new(bytes.Buffer)
		node2Metrics.Write(node2Data)

		scraper = &apiServiceScraper{
			resourceManager: &fakeResourceGetter{
				nodes: []string{"node1", "node2", "node3"},
				namespaces: map[string][]MappedNamespace{
					"cluster1": {
						{
							Namespace:    "namespace1",
							OriginalName: "original_namespace1",
						},
					},
					"cluster2": {
						{
							Namespace:    "namespace2",
							OriginalName: "original_namespace2",
						},
					},
				},
				pods: map[string]map[string][]string{
					"node1": {
						"cluster1": {"pod1", "pod2"},
						"cluster2": {"pod3", "pod4"},
					},
					"node2": {
						"cluster1": {"pod5", "pod6"},
						"cluster2": {"pod7", "pod8"},
					},
					"node3": {},
				},
			},
			rawGetter: &fakeRawGetter{
				data: map[string][]byte{
					"node1": node1Data.Bytes(),
					"node2": node2Data.Bytes(),
					"node3": []byte(""),
				},
			},
		}
	})

	JustBeforeEach(func() {
		ctx := context.Background()
		metrics, err = scraper.Scrape(ctx, "metrics", "cluster1")
	})

	It("should scrape metrics", func() {
		Expect(err).ToNot(HaveOccurred())
		Expect(len(metrics)).To(Equal(2))

		Expect(metrics[0].promType).To(Equal("# TYPE metric1"))
		Expect(metrics[0].promHelp).To(Equal("# HELP metric1"))
		Expect(metrics[0].values).To(ConsistOf(
			"metric1{namespace=\"original_namespace1\",pod=\"pod1\"} 1 1000000000",
			"metric1{namespace=\"original_namespace1\",pod=\"pod2\"} 2 2000000000",
			"metric1{namespace=\"original_namespace1\",pod=\"pod5\"} 4 1000000000",
		))

		Expect(metrics[1].promType).To(Equal("# TYPE metric2"))
		Expect(metrics[1].promHelp).To(Equal("# HELP metric2"))
		Expect(metrics[1].values).To(ConsistOf(
			"metric2{namespace=\"original_namespace1\",pod=\"pod1\"} 1 1000000000",
		))
	})

})

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

package restcfg_test

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"

	flagsutils "github.com/liqotech/liqo/pkg/utils/flags"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var _ = Describe("The rate limiting utility functions", func() {

	var (
		cfg    rest.Config
		output *rest.Config
	)

	const (
		timeout = 10
		qps     = 67
		burst   = 89
	)

	Describe("the SetRateLimiter function", func() {
		Context("configuring the rate limiting parameters", func() {
			var fs pflag.FlagSet

			BeforeEach(func() {
				fs = *pflag.NewFlagSet("test-flags", pflag.PanicOnError)
				restcfg.InitFlags(&fs)
				flagsutils.InitKlogFlags(&fs)
			})
			JustBeforeEach(func() { output = restcfg.SetRateLimiter(&cfg) })

			When("using the default configuration", func() {
				It("should return a pointer to the original object", func() { Expect(output).To(BeIdenticalTo(&cfg)) })
				It("should set the default QPS value", func() { Expect(cfg.QPS).To(BeNumerically("==", restcfg.DefaultQPS)) })
				It("should set the default burst value", func() { Expect(cfg.Burst).To(BeNumerically("==", restcfg.DefaultBurst)) })
			})

			When("specifying a custom configuration", func() {
				BeforeEach(func() {
					utilruntime.Must(fs.Set("client-qps", strconv.FormatInt(qps, 10)))
					utilruntime.Must(fs.Set("client-max-burst", strconv.FormatInt(burst, 10)))
				})

				It("should return a pointer to the original object", func() { Expect(output).To(BeIdenticalTo(&cfg)) })
				It("should set the desired QPS value", func() { Expect(cfg.QPS).To(BeNumerically("==", qps)) })
				It("should set the desired burst value", func() { Expect(cfg.Burst).To(BeNumerically("==", burst)) })
			})
		})
	})

	Describe("the SetRateLimiterWithCustomParameters function", func() {
		Context("configuring the rate limiting parameters", func() {
			JustBeforeEach(func() { output = restcfg.SetRateLimiterWithCustomParameters(&cfg, timeout, qps, burst) })

			It("should return a pointer to the original object", func() { Expect(output).To(BeIdenticalTo(&cfg)) })
			It("should set the desired timeout value", func() { Expect(cfg.Timeout).To(BeNumerically("==", timeout)) })
			It("should set the desired QPS value", func() { Expect(cfg.QPS).To(BeNumerically("==", qps)) })
			It("should set the desired burst value", func() { Expect(cfg.Burst).To(BeNumerically("==", burst)) })
		})
	})
})

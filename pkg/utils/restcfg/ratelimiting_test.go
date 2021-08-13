package restcfg_test

import (
	"flag"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var _ = Describe("The rate limiting utility functions", func() {

	var (
		cfg    rest.Config
		output *rest.Config
	)

	const (
		qps   = 67
		burst = 89
	)

	Describe("the SetRateLimiter function", func() {
		Context("configuring the rate limiting parameters", func() {
			var fs flag.FlagSet

			BeforeEach(func() {
				fs = *flag.NewFlagSet("test-flags", flag.PanicOnError)
				restcfg.InitFlags(&fs)
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

	Describe("the SetRateLimiterWithCustomParamenters function", func() {
		Context("configuring the rate limiting parameters", func() {
			JustBeforeEach(func() { output = restcfg.SetRateLimiterWithCustomParamenters(&cfg, qps, burst) })

			It("should return a pointer to the original object", func() { Expect(output).To(BeIdenticalTo(&cfg)) })
			It("should set the desired QPS value", func() { Expect(cfg.QPS).To(BeNumerically("==", qps)) })
			It("should set the desired burst value", func() { Expect(cfg.Burst).To(BeNumerically("==", burst)) })
		})
	})
})

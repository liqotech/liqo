package trace_test

import (
	"flag"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils/trace"
)

var _ = Describe("Trace utilities", func() {

	var fs flag.FlagSet

	BeforeEach(func() {
		fs = *flag.NewFlagSet("test-flags", flag.PanicOnError)
		klog.InitFlags(&fs)
	})

	DescribeTable("The LongThreshold function",
		func(level int, expected time.Duration) {
			utilruntime.Must(fs.Set("v", strconv.FormatInt(int64(level), 10)))
			Expect(trace.LongThreshold()).To(Equal(expected))
		},
		Entry("with log level 0", 0, time.Second),
		Entry("with log level 1", 1, time.Second),
		Entry("with log level 2", 2, 500*time.Millisecond),
		Entry("with log level 3", 3, 500*time.Millisecond),
		Entry("with log level 4", 4, 250*time.Millisecond),
		Entry("with log level 5", 5, 100*time.Millisecond),
		Entry("with log level 6", 6, 100*time.Millisecond),
		Entry("with log level 7", 7, 100*time.Millisecond),
		Entry("with log level 8", 8, 100*time.Millisecond),
		Entry("with log level 9", 9, 100*time.Millisecond),
	)
})

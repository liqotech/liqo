// Copyright 2019-2021 The Liqo Authors
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

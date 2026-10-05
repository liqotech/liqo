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

package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/utils/cache"
)

// countingFunc returns a computation function returning the given value and error, recording its
// invocations in calls.
func countingFunc(calls *atomic.Int64, value string, err error) func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		calls.Add(1)
		return value, err
	}
}

// countingSleepFunc behaves like countingFunc, additionally sleeping for the given delay before
// returning, to keep concurrent invocations in flight.
func countingSleepFunc(calls *atomic.Int64, value string, err error, delay time.Duration) func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		calls.Add(1)
		time.Sleep(delay)
		return value, err
	}
}

var _ = Describe("Cache", func() {
	var ctx context.Context

	BeforeEach(func() { ctx = context.Background() })

	It("should invoke the function once for calls within the TTL", func() {
		var calls atomic.Int64
		c := cache.New[string](time.Minute, 0)
		fn := countingFunc(&calls, "value", nil)

		first, err := c.Do(ctx, "key", fn)
		Expect(err).ToNot(HaveOccurred())
		second, err := c.Do(ctx, "key", fn)
		Expect(err).ToNot(HaveOccurred())

		Expect(first).To(Equal("value"))
		Expect(second).To(Equal("value"))
		Expect(calls.Load()).To(BeNumerically("==", 1))
	})

	It("should recompute the value once the TTL expires", func() {
		var calls atomic.Int64
		c := cache.New[string](10*time.Millisecond, 0)
		fn := countingFunc(&calls, "value", nil)

		_, err := c.Do(ctx, "key", fn)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() int64 {
			_, err := c.Do(ctx, "key", fn)
			Expect(err).ToNot(HaveOccurred())
			return calls.Load()
		}, time.Second).Should(BeNumerically(">", 1))
	})

	It("should deduplicate concurrent computations of the same key", func() {
		const goroutines = 20

		var calls atomic.Int64
		c := cache.New[string](time.Minute, 0)
		fn := countingSleepFunc(&calls, "value", nil, 20*time.Millisecond)

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				<-start
				value, err := c.Do(ctx, "key", fn)
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal("value"))
			}()
		}
		close(start)
		wg.Wait()

		Expect(calls.Load()).To(BeNumerically("==", 1))
	})

	It("should cache and re-serve errors within the error TTL", func() {
		var calls atomic.Int64
		expected := errors.New("failure")
		c := cache.New[string](time.Minute, time.Minute)
		fn := func(context.Context) (string, error) { calls.Add(1); return "", expected }

		_, err := c.Do(ctx, "key", fn)
		Expect(err).To(MatchError(expected))
		_, err = c.Do(ctx, "key", fn)
		Expect(err).To(MatchError(expected))

		Expect(calls.Load()).To(BeNumerically("==", 1))
	})

	It("should not cache errors when the error TTL is not positive", func() {
		var calls atomic.Int64
		expected := errors.New("failure")
		c := cache.New[string](time.Minute, 0)
		fn := func(context.Context) (string, error) { calls.Add(1); return "", expected }

		_, err := c.Do(ctx, "key", fn)
		Expect(err).To(MatchError(expected))
		_, err = c.Do(ctx, "key", fn)
		Expect(err).To(MatchError(expected))

		Expect(calls.Load()).To(BeNumerically("==", 2))
	})

	It("should not cache context cancellation errors", func() {
		var calls atomic.Int64
		c := cache.New[string](time.Minute, time.Minute)
		fn := func(context.Context) (string, error) { calls.Add(1); return "", context.Canceled }

		_, err := c.Do(ctx, "key", fn)
		Expect(err).To(MatchError(context.Canceled))
		_, err = c.Do(ctx, "key", fn)
		Expect(err).To(MatchError(context.Canceled))

		Expect(calls.Load()).To(BeNumerically("==", 2))
	})
})

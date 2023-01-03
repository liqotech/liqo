// Copyright 2019-2023 The Liqo Authors
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

package resolver

import (
	"context"
	"net"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolver", func() {

	var (
		newSyncMap = func(m map[string]*net.IPAddr, sm *sync.Map) {
			sm.Range(func(key, value interface{}) bool {
				sm.Delete(key)
				return true
			})
			for k, v := range m {
				sm.Store(k, v)
			}
		}

		newMap = func(sm *sync.Map) map[string]*net.IPAddr {
			res := map[string]*net.IPAddr{}
			sm.Range(func(k, v interface{}) bool {
				res[k.(string)] = v.(*net.IPAddr)
				return true
			})
			return res
		}
	)

	var (
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	type resolveTestcase struct {
		name          string
		resolver      func(ctx context.Context, host string) ([]net.IPAddr, error)
		cache         map[string]*net.IPAddr
		expectedIP    string
		expectedCache OmegaMatcher
	}

	DescribeTable("Resolve",

		func(c *resolveTestcase) {
			newSyncMap(c.cache, &resolveCache)
			resolverFunc = c.resolver
			ip, err := Resolve(ctx, c.name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ip.String()).To(Equal(c.expectedIP))
			Expect(newMap(&resolveCache)).To(c.expectedCache)
		},

		Entry("should handle an IP name", &resolveTestcase{
			name:          "1.2.3.4",
			resolver:      nil,
			cache:         map[string]*net.IPAddr{},
			expectedIP:    "1.2.3.4",
			expectedCache: BeEmpty(),
		}),

		Entry("should resolve IPv4 addresses", &resolveTestcase{
			name: "example.com",
			resolver: func(ctx context.Context, host string) ([]net.IPAddr, error) {
				return []net.IPAddr{
					{IP: net.IPv4(1, 2, 3, 4)},
					{IP: net.IPv4(1, 2, 3, 5)},
				}, nil
			},
			cache:         map[string]*net.IPAddr{},
			expectedIP:    "1.2.3.4",
			expectedCache: HaveKeyWithValue("example.com", &net.IPAddr{IP: net.IPv4(1, 2, 3, 4)}),
		}),

		Entry("should resolve IPv4 address with cache", &resolveTestcase{
			name: "example.com",
			resolver: func(ctx context.Context, host string) ([]net.IPAddr, error) {
				return []net.IPAddr{
					{IP: net.IPv4(1, 2, 3, 4)},
					{IP: net.IPv4(1, 2, 3, 5)},
				}, nil
			},
			cache: map[string]*net.IPAddr{
				"example.com": {IP: net.IPv4(1, 2, 3, 5)},
			},
			expectedIP:    "1.2.3.5",
			expectedCache: HaveKeyWithValue("example.com", &net.IPAddr{IP: net.IPv4(1, 2, 3, 5)}),
		}),

		Entry("should resolve IPv6 addresses", &resolveTestcase{
			name: "example.com",
			resolver: func(ctx context.Context, host string) ([]net.IPAddr, error) {
				return []net.IPAddr{
					{IP: net.ParseIP("2001:db8:a0b:12f0::1")},
					{IP: net.ParseIP("2001:db8:a0b:12f0::2")},
				}, nil
			},
			cache:         map[string]*net.IPAddr{},
			expectedIP:    "2001:db8:a0b:12f0::1",
			expectedCache: HaveKeyWithValue("example.com", &net.IPAddr{IP: net.ParseIP("2001:db8:a0b:12f0::1")}),
		}),

		Entry("should resolve IPv6 address with cache", &resolveTestcase{
			name: "example.com",
			resolver: func(ctx context.Context, host string) ([]net.IPAddr, error) {
				return []net.IPAddr{
					{IP: net.ParseIP("2001:db8:a0b:12f0::1")},
					{IP: net.ParseIP("2001:db8:a0b:12f0::2")},
				}, nil
			},
			cache: map[string]*net.IPAddr{
				"example.com": {IP: net.ParseIP("2001:db8:a0b:12f0::2")},
			},
			expectedIP:    "2001:db8:a0b:12f0::2",
			expectedCache: HaveKeyWithValue("example.com", &net.IPAddr{IP: net.ParseIP("2001:db8:a0b:12f0::2")}),
		}),

		Entry("should prefer IPv4 address", &resolveTestcase{
			name: "example.com",
			resolver: func(ctx context.Context, host string) ([]net.IPAddr, error) {
				return []net.IPAddr{
					{IP: net.ParseIP("2001:db8:a0b:12f0::1")},
					{IP: net.IPv4(1, 2, 3, 4)},
				}, nil
			},
			cache:         map[string]*net.IPAddr{},
			expectedIP:    "1.2.3.4",
			expectedCache: HaveKeyWithValue("example.com", &net.IPAddr{IP: net.IPv4(1, 2, 3, 4)}),
		}),

		Entry("should prefer IPv4 address with cache", &resolveTestcase{
			name: "example.com",
			resolver: func(ctx context.Context, host string) ([]net.IPAddr, error) {
				return []net.IPAddr{
					{IP: net.ParseIP("2001:db8:a0b:12f0::1")},
					{IP: net.IPv4(1, 2, 3, 4)},
				}, nil
			},
			cache: map[string]*net.IPAddr{
				"example.com": {IP: net.ParseIP("2001:db8:a0b:12f0::1")},
			},
			expectedIP:    "1.2.3.4",
			expectedCache: HaveKeyWithValue("example.com", &net.IPAddr{IP: net.IPv4(1, 2, 3, 4)}),
		}),

		Entry("should update cache if entry is no more valid", &resolveTestcase{
			name: "example.com",
			resolver: func(ctx context.Context, host string) ([]net.IPAddr, error) {
				return []net.IPAddr{
					{IP: net.IPv4(1, 2, 3, 5)},
				}, nil
			},
			cache: map[string]*net.IPAddr{
				"example.com": {IP: net.IPv4(1, 2, 3, 4)},
			},
			expectedIP:    "1.2.3.5",
			expectedCache: HaveKeyWithValue("example.com", &net.IPAddr{IP: net.IPv4(1, 2, 3, 5)}),
		}),
	)

})

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

package cache

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// entry is a cached computation outcome: either a value (successful computation) or an
// error (failed one) is set, together with its expiration time.
type entry[V any] struct {
	value     V
	err       error
	expiresAt time.Time
}

// Cache is a read-through cache keyed by string. Successful computations are cached for ttl,
// failed ones for errorTTL (when positive). Concurrent computations of the same key are
// deduplicated through singleflight, so that only one of them is actually performed.
//
// The values are stored and returned as-is: callers must treat the cached value as read-only.
type Cache[V any] struct {
	ttl      time.Duration
	errorTTL time.Duration

	mu      sync.Mutex
	entries map[string]entry[V]

	group singleflight.Group
}

// New returns a new Cache, caching successful results for ttl and failed ones for errorTTL.
// A non-positive errorTTL disables error caching.
func New[V any](ttl, errorTTL time.Duration) *Cache[V] {
	return &Cache[V]{
		ttl:      ttl,
		errorTTL: errorTTL,
		entries:  map[string]entry[V]{},
	}
}

// Do returns the cached value for key, if present and not expired. Otherwise, it computes it
// through fn and caches the outcome. Concurrent calls for the same key are collapsed into a
// single fn invocation, which receives the context of the caller that first triggered it.
//
// Errors are cached for errorTTL, except context cancellation and deadline-exceeded ones,
// which are never cached (they are not a property of the computation itself).
func (c *Cache[V]) Do(ctx context.Context, key string, fn func(context.Context) (V, error)) (V, error) {
	if cached, found := c.get(key); found {
		return cached.value, cached.err
	}

	// Only one concurrent caller for the same key actually runs fn; the others wait for the
	// shared result.
	res, err, _ := c.group.Do(key, func() (interface{}, error) {
		value, err := fn(ctx)
		if err != nil {
			c.setError(key, err)
			return nil, err
		}
		c.setValue(key, value)
		return value, nil
	})
	if err != nil {
		var zero V
		return zero, err
	}
	return res.(V), nil
}

// get returns the cache entry for the given key, if present and not expired. Expired entries
// are evicted, so that stale results do not linger in the cache.
func (c *Cache[V]) get(key string) (entry[V], bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cached, found := c.entries[key]
	if !found {
		return entry[V]{}, false
	}
	if !time.Now().Before(cached.expiresAt) {
		delete(c.entries, key)
		return entry[V]{}, false
	}
	return cached, true
}

// setValue stores a successful computation outcome for the given key.
func (c *Cache[V]) setValue(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.evictExpired()
	c.entries[key] = entry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
}

// setError stores a failed computation outcome for the given key, unless error caching is
// disabled or the error is a context cancellation/deadline one.
func (c *Cache[V]) setError(key string, err error) {
	if c.errorTTL <= 0 || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.evictExpired()
	c.entries[key] = entry[V]{err: err, expiresAt: time.Now().Add(c.errorTTL)}
}

// evictExpired removes the expired entries, to bound the memory retained by keys not
// requested anymore. It must be called while holding the mutex.
func (c *Cache[V]) evictExpired() {
	now := time.Now()
	for key, cached := range c.entries {
		if !now.Before(cached.expiresAt) {
			delete(c.entries, key)
		}
	}
}

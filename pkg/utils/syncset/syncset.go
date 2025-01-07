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

package syncset

import "sync"

// SyncSet contains a set of elements and provides utility methods safe for concurrent access.
type SyncSet struct {
	mu  sync.RWMutex
	set map[string]struct{}
}

// New returns a nre SyncSet instance.
func New() *SyncSet {
	return &SyncSet{
		set: make(map[string]struct{}),
	}
}

// Add adds the given element to the set (nop is already present).
func (sc *SyncSet) Add(fc string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.set[fc] = struct{}{}
}

// Remove removes the given element to the set (nop is already absent).
func (sc *SyncSet) Remove(fc string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	delete(sc.set, fc)
}

// ForEach executes the given function for all elements in the set.
func (sc *SyncSet) ForEach(fn func(string)) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	for key := range sc.set {
		fn(key)
	}
}

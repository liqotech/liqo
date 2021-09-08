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

package namespacesmapping

import "sync"

type namespaceReadyMapCache struct {
	sync.RWMutex
	mappings map[string]string
}

func (n *namespaceReadyMapCache) write(local, remote string) {
	n.Lock()
	defer n.Unlock()
	n.mappings[local] = remote
}

func (n *namespaceReadyMapCache) read(local string) string {
	n.RLock()
	defer n.RUnlock()
	return n.mappings[local]
}

func (n *namespaceReadyMapCache) inverseRead(remote string) string {
	n.RLock()
	defer n.RUnlock()
	for k, v := range n.mappings {
		if v == remote {
			return k
		}
	}
	return namespaceMapEntryNotAvailable
}

func (n *namespaceReadyMapCache) readAll() map[string]string {
	n.RLock()
	defer n.RUnlock()
	return n.mappings
}

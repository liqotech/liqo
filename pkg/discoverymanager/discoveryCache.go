// Copyright 2019-2022 The Liqo Authors
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

package discovery

import (
	"errors"
	"sync"

	"github.com/jinzhu/copier"

	"github.com/liqotech/liqo/pkg/auth"
)

type discoveryData struct {
	AuthData    *AuthData
	ClusterInfo *auth.ClusterInfo
}

// cache used to match different services coming for the same Liqo instance.
type discoveryCache struct {
	discoveredServices map[string]discoveryData
	lock               sync.RWMutex
}

var resolvedData = discoveryCache{
	discoveredServices: map[string]discoveryData{},
}

func (discoveryCache *discoveryCache) add(key string, data discoverableData) {
	discoveryCache.lock.Lock()
	defer discoveryCache.lock.Unlock()
	if _, ok := discoveryCache.discoveredServices[key]; !ok {
		if authData, ok := data.(*AuthData); ok {
			discoveryCache.discoveredServices[key] = discoveryData{
				AuthData: authData,
			}
		}
	} else {
		if authData, ok := data.(*AuthData); ok {
			oldData := discoveryCache.discoveredServices[key]
			oldData.AuthData = authData
			discoveryCache.discoveredServices[key] = oldData
		}
	}
}

// after that the ForeignCluster is create we can delete the entry in the cache,
// the cache is clean again for the next discovery (and TTL update).
func (discoveryCache *discoveryCache) delete(key string) {
	discoveryCache.lock.Lock()
	defer discoveryCache.lock.Unlock()
	delete(discoveryCache.discoveredServices, key)
}

func (discoveryCache *discoveryCache) get(key string) (*discoveryData, error) {
	discoveryCache.lock.RLock()
	defer discoveryCache.lock.RUnlock()
	v, ok := discoveryCache.discoveredServices[key]
	if !ok {
		return nil, errors.New("key not found")
	}

	res := &discoveryData{}
	if err := copier.Copy(res, v); err != nil {
		return nil, err
	}
	return res, nil
}

func (discoveryCache *discoveryCache) isComplete(key string) bool {
	discoveryCache.lock.RLock()
	defer discoveryCache.lock.RUnlock()
	v, ok := discoveryCache.discoveredServices[key]
	if !ok {
		return false
	}

	return v.isComplete()
}

func (discoveryData *discoveryData) isComplete() bool {
	return discoveryData.AuthData != nil
}

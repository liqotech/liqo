package discovery

import (
	"errors"
	"github.com/jinzhu/copier"
	"sync"
)

type discoveryData struct {
	TxtData  *TxtData
	AuthData *AuthData
}

// cache used to match different services coming for the same Liqo instance
type discoveryCache struct {
	discoveredServices map[string]discoveryData
	lock               sync.RWMutex
}

var resolvedData = discoveryCache{
	discoveredServices: map[string]discoveryData{},
}

func NewDiscoveryData(txtData *TxtData, authData *AuthData) *discoveryData {
	return &discoveryData{
		TxtData:  txtData,
		AuthData: authData,
	}
}

func (discoveryCache *discoveryCache) add(key string, data DiscoverableData) {
	discoveryCache.lock.Lock()
	defer discoveryCache.lock.Unlock()
	if _, ok := discoveryCache.discoveredServices[key]; !ok {
		switch data := data.(type) {
		case *TxtData:
			discoveryCache.discoveredServices[key] = discoveryData{
				TxtData: data,
			}
		case *AuthData:
			discoveryCache.discoveredServices[key] = discoveryData{
				AuthData: data,
			}
		}
	} else {
		oldData := discoveryCache.discoveredServices[key]
		switch data := data.(type) {
		case *TxtData:
			oldData.TxtData = data
		case *AuthData:
			oldData.AuthData = data
		}
		discoveryCache.discoveredServices[key] = oldData
	}
}

// after that the ForeignCluster is create we can delete the entry in the cache,
// the cache is clean again for the next discovery (and TTL update)
func (discoveryCache *discoveryCache) delete(key string) {
	discoveryCache.lock.Lock()
	defer discoveryCache.lock.Unlock()
	delete(discoveryCache.discoveredServices, key)
}

func (discoveryCache *discoveryCache) get(key string) (*discoveryData, error) {
	discoveryCache.lock.RLock()
	defer discoveryCache.lock.RUnlock()
	if v, ok := discoveryCache.discoveredServices[key]; !ok {
		return nil, errors.New("key not found")
	} else {
		res := &discoveryData{}
		if err := copier.Copy(res, v); err != nil {
			return nil, err
		}
		return res, nil
	}
}

func (discoveryCache *discoveryCache) isComplete(key string) bool {
	discoveryCache.lock.RLock()
	defer discoveryCache.lock.RUnlock()
	if v, ok := discoveryCache.discoveredServices[key]; !ok {
		return false
	} else {
		return v.isComplete()
	}
}

func (discoveryData *discoveryData) isComplete() bool {
	return discoveryData.TxtData != nil && discoveryData.AuthData != nil
}

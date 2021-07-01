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

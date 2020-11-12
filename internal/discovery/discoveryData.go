package discovery

import "github.com/grandcat/zeroconf"

type DiscoverableData interface {
	Get(discovery *DiscoveryCtrl, entry *zeroconf.ServiceEntry) error
}

package discovery

import "github.com/grandcat/zeroconf"

type DiscoverableData interface {
	Get(discovery *DiscoveryCtrl, entry *zeroconf.ServiceEntry) error // populate the struct from a DNS entry
	IsComplete() bool                                                 // the struct has all the required fields
}

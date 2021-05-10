package discovery

import "github.com/grandcat/zeroconf"

type discoverableData interface {
	Get(discovery *Controller, entry *zeroconf.ServiceEntry) error // populate the struct from a DNS entry
	IsComplete() bool                                              // the struct has all the required fields
}

package types

import (
	"sync"

	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
)

type NetworkingKey string
type NetworkingValue string

const (
	// VirtualNodeName is the key for the option containing the name to assign to the virtual node.
	VirtualNodeName = "virtualNodeName"
	// RemoteClusterID is the key for the option containing the remote clusterID.
	RemoteClusterID = "remoteClusterID"
	// LiqoIpamServer is the key for the option containing server serving the ipam service.
	LiqoIpamServer = "liqoIpamServer"
)

func NewNetworkingOption(key NetworkingKey, value NetworkingValue) *NetworkingOption {
	return &NetworkingOption{
		key:   key,
		value: value,
		lock:  sync.RWMutex{},
	}
}

type NetworkingOption struct {
	key   NetworkingKey
	value NetworkingValue

	isSet bool
	lock  sync.RWMutex
}

func (o *NetworkingOption) Key() options.OptionKey {
	return options.OptionKey(o.key)
}

func (o *NetworkingOption) Value() options.OptionValue {
	o.lock.RLock()
	defer o.lock.RUnlock()

	return options.OptionValue(o.value)
}

func (o *NetworkingOption) SetValue(v options.OptionValue) {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.value = NetworkingValue(v)
	o.isSet = true
}

func (o *NetworkingOption) IsSet() bool {
	o.lock.RLock()
	defer o.lock.RUnlock()

	return o.isSet
}

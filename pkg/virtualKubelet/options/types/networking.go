package types

import (
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"sync"
)

type NetworkingKey string
type NetworkingValue string

const (
	LocalRemappedPodCIDR  = "localRemappedPodCIDR"
	RemoteRemappedPodCIDR = "remoteRemappedPodCIDR"
	VirtualNodeName       = "virtualNodeName"
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

	lock sync.RWMutex
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
}

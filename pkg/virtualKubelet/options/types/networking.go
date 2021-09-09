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

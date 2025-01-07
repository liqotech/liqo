// Copyright 2019-2025 The Liqo Authors
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

package wireguard

import (
	"fmt"
	"net"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/liqotech/liqo/pkg/gateway"
)

// WgImplementation represents the implementation of the wireguard interface.
type WgImplementation string

const (
	// WgImplementationKernel represents the kernel implementation of the wireguard interface.
	WgImplementationKernel WgImplementation = "kernel"
	// WgImplementationUserspace represents the userspace implementation of the wireguard interface.
	WgImplementationUserspace WgImplementation = "userspace"
)

// String returns the string representation of the wireguard implementation.
func (wgi WgImplementation) String() string {
	return string(wgi)
}

// Set parses the provided string into the wireguard implementation.
func (wgi *WgImplementation) Set(s string) error {
	if s == "" {
		s = WgImplementationKernel.String()
	}
	if s != WgImplementationKernel.String() && s != WgImplementationUserspace.String() {
		return fmt.Errorf("invalid wireguard implementation: %s (allowed values are: %s,%s)", s, WgImplementationKernel, WgImplementationUserspace)
	}
	*wgi = WgImplementation(s)
	return nil
}

// Type returns the type of the wireguard implementation.
func (wgi WgImplementation) Type() string {
	return "string"
}

// Options contains the options for the wireguard interface.
type Options struct {
	GwOptions *gateway.Options

	MTU             int
	PrivateKey      wgtypes.Key
	InterfaceIP     string
	ListenPort      int
	EndpointAddress string
	EndpointPort    int
	KeysDir         string

	EndpointIP      net.IP
	EndpointIPMutex *sync.Mutex

	DNSCheckInterval time.Duration

	Implementation WgImplementation
}

// NewOptions returns a new Options struct.
func NewOptions(options *gateway.Options) *Options {
	return &Options{
		GwOptions:       options,
		EndpointIPMutex: &sync.Mutex{},
	}
}

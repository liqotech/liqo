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

package tunnel

import (
	"github.com/vishvananda/netlink"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
)

// DriverCreateFunc function prototype to create a new driver.
type DriverCreateFunc func(k8sClientset k8s.Interface, namespace string, config Config) (Driver, error)

// Drivers static map of supported drivers.
var Drivers = map[string]DriverCreateFunc{}

// AddDriver adds a supported driver to the drivers map, prints a fatal error in the case of double registration.
func AddDriver(name string, driverCreate DriverCreateFunc) {
	if Drivers[name] != nil {
		klog.Fatalf("Multiple tunnel drivers attempting to register with name %q", name)
	}
	klog.V(5).Infof("driver for %s VPN successfully registered", name)
	Drivers[name] = driverCreate
}

// Config configuration for tunnel drivers passed during the creation.
type Config struct {
	MTU           int
	ListeningPort int
}

// Driver the interface needed to be implemented by new vpn drivers.
type Driver interface {
	Init() error

	ConnectToEndpoint(tep *netv1alpha1.TunnelEndpoint) (*netv1alpha1.Connection, error)

	DisconnectFromEndpoint(tep *netv1alpha1.TunnelEndpoint) error

	GetLink() netlink.Link

	Close() error
}

package tunnel

import (
	"github.com/vishvananda/netlink"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
)

// DriverCreateFunc function prototype to create a new driver.
type DriverCreateFunc func(k8sClientset k8s.Interface, namespace string) (Driver, error)

// Drivers static map of supported drivers.
var Drivers = map[string]DriverCreateFunc{}

// AddDriver adds a supported driver to the drivers map, prints a fatal error in the case of double registration.
func AddDriver(name string, driverCreate DriverCreateFunc) {
	if Drivers[name] != nil {
		klog.Fatalf("Multiple tunnel drivers attempting to register with name %q", name)
	}
	klog.Infof("driver for %s VPN successfully registered", name)
	Drivers[name] = driverCreate
}

// Driver the interface needed to be implemented by new vpn drivers.
type Driver interface {
	Init() error

	ConnectToEndpoint(tep *netv1alpha1.TunnelEndpoint) (*netv1alpha1.Connection, error)

	DisconnectFromEndpoint(tep *netv1alpha1.TunnelEndpoint) error

	GetLink() netlink.Link

	Close() error
}

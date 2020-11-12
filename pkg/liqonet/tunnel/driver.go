package tunnel

import (
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// Function prototype to create a new driver
type DriverCreateFunc func(k8sClientset *k8s.Clientset, namespace string) (Driver, error)

// Static map of supported drivers
var Drivers = map[string]DriverCreateFunc{}

// Adds a supported driver, prints a fatal error in the case of double registration
func AddDriver(name string, driverCreate DriverCreateFunc) {
	if Drivers[name] != nil {
		klog.Fatalf("Multiple tunnel drivers attempting to register with name %q", name)
	}
	klog.Infof("driver for %s VPN successfully registered", name)
	Drivers[name] = driverCreate
}

type Driver interface {
	Init() error

	ConnectToEndpoint(tep *netv1alpha1.TunnelEndpoint) (*netv1alpha1.Connection, error)

	DisconnectFromEndpoint(tep *netv1alpha1.TunnelEndpoint) error

	Close() error
}

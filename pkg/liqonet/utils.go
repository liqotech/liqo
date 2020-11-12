package liqonet

import (
	"bytes"
	"context"
	"fmt"
	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/liqotech/liqo/internal/utils/errdefs"
	"github.com/vishvananda/netlink"
	"golang.org/x/tools/go/ssa/interp/testdata/src/errors"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"net"
	"os"
	"syscall"
)

var (
	ShutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGKILL}
)

func GetPodIP() (net.IP, error) {
	ipAddress, isSet := os.LookupEnv("POD_IP")
	if !isSet {
		return nil, errdefs.NotFound("the pod IP is not set")
	}
	if ipAddress == "" {
		return nil, errors.New("pod IP is not yet set")
	}
	return net.ParseIP(ipAddress), nil
}

func GetPodNamespace() (string, error) {
	namespace, isSet := os.LookupEnv("POD_NAMESPACE")
	if !isSet {
		return "", errdefs.NotFound("the POD_NAMESPACE environment variable is not set as an environment variable")
	}
	return namespace, nil
}

func GetNodeName() (string, error) {
	nodeName, isSet := os.LookupEnv("NODE_NAME")
	if !isSet {
		return nodeName, errdefs.NotFound("NODE_NAME environment variable has not been set. check you manifest file")
	}
	return nodeName, nil
}

func GetInternalIPOfNode(node *corev1.Node) (string, error) {
	var internalIp string
	for _, address := range node.Status.Addresses {
		if address.Type == "InternalIP" {
			internalIp = address.Address
			break
		}
	}
	if internalIp == "" {
		klog.V(4).Infof("internalIP of the node not found, probably is not set")
		return internalIp, errdefs.NotFound("internalIP of the node is not set")
	}
	return internalIp, nil
}

// Helper functions to check if a string is contained in a slice of strings.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// Helper functions to check and remove string from a slice of strings.
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func VerifyNoOverlap(subnets map[string]*net.IPNet, newNet *net.IPNet) bool {
	firstLastIP := make([][]net.IP, 1)

	for _, value := range subnets {
		if bytes.Compare(value.Mask, newNet.Mask) <= 0 {
			first, last := cidr.AddressRange(newNet)
			firstLastIP[0] = []net.IP{first, last}
			if value.Contains(firstLastIP[0][0]) || value.Contains(firstLastIP[0][1]) {
				return true
			}
		} else {
			first, last := cidr.AddressRange(value)
			firstLastIP[0] = []net.IP{first, last}
			if newNet.Contains(firstLastIP[0][0]) || newNet.Contains(firstLastIP[0][1]) {
				return true
			}
		}
	}
	return false
}

func GetClusterID(client *kubernetes.Clientset, cmName, namespace string) (string, error) {
	cmClient := client.CoreV1().ConfigMaps(namespace)
	cm, err := cmClient.Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	clusterID := cm.Data[cmName]
	return clusterID, nil
}

func EnableIPForwarding() error {
	err := ioutil.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0600)
	if err != nil {
		return fmt.Errorf("unable to enable ip forwaring in the gateway pod: %v", err)
	}
	return nil
}

func GetDefaultIfaceName() (string, error) {
	//search for the default route and return the link associated to the route
	//we consider only the ipv4 routes
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", err
	}
	var route netlink.Route
	for _, route = range routes {
		if route.Dst == nil {
			break
		}
	}
	//get default link
	defualtIface, err := netlink.LinkByIndex(route.LinkIndex)
	if err != nil {
		return "", err
	}
	return defualtIface.Attrs().Name, nil
}

func DeleteIFaceByIndex(ifaceIndex int) error {
	existingIface, err := netlink.LinkByIndex(ifaceIndex)
	if err != nil {
		klog.Errorf("unable to retrieve tunnel interface: %v", err)
		return err
	}
	//Remove the existing gre interface
	if err = netlink.LinkDel(existingIface); err != nil {
		klog.Errorf("unable to delete the tunnel after the tunnelEndpoint CR has been removed: %v", err)
		return err
	}
	return err
}
